package waffle

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/sourcegraph/conc"
)

const (
	defaultReactionPollInterval = time.Second
	defaultReactionPollLimit    = 100
)

type handlerLookup func(eventType, name string) (registeredHandler, bool)

type reactionDispatcher struct {
	workers       int
	logger        *slog.Logger
	errorHook     ErrorHook
	store         ReactionStore
	handlerLookup handlerLookup
	pollInterval  time.Duration
	pollLimit     int

	mu       sync.Mutex
	started  bool
	jobs     chan reactionJob
	wake     chan struct{}
	cancel   context.CancelFunc
	inFlight map[string]struct{}
	pending  sync.WaitGroup
	workerWG conc.WaitGroup
	pollerWG sync.WaitGroup
}

func newReactionDispatcher(
	workers int,
	logger *slog.Logger,
	errorHook ErrorHook,
	store ReactionStore,
	lookup handlerLookup,
) *reactionDispatcher {
	return &reactionDispatcher{
		workers:       workers,
		logger:        logger,
		errorHook:     errorHook,
		store:         store,
		handlerLookup: lookup,
		pollInterval:  defaultReactionPollInterval,
		pollLimit:     defaultReactionPollLimit,
		inFlight:      make(map[string]struct{}),
	}
}

func (d *reactionDispatcher) Start(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.started {
		return nil
	}
	if err := d.store.ResetRunningReactions(ctx); err != nil {
		return err
	}

	runCtx, cancel := context.WithCancel(ctx)
	d.cancel = cancel
	d.jobs = make(chan reactionJob, d.workers*64)
	d.wake = make(chan struct{}, 1)
	for range d.workers {
		d.workerWG.Go(d.work)
	}
	d.pollerWG.Add(1)
	go d.poll(runCtx)
	d.started = true

	if log := d.log(ctx); log != nil {
		log.InfoContext(ctx, "persistent reactions started", "workers", d.workers)
	}
	return nil
}

func (d *reactionDispatcher) Dispatch(ctx context.Context, event AnyEvent, record EventRecord, handlers []registeredHandler) error {
	now := time.Now().UTC()
	reactions := make([]ReactionRecord, 0, len(handlers))
	for _, handler := range handlers {
		reactions = append(reactions, ReactionRecord{
			ID:          "rxn_" + ulid.Make().String(),
			EventID:     event.ID(),
			HandlerName: handler.name,
			Status:      ReactionPending,
			CreatedAt:   now,
			UpdatedAt:   now,
		})
	}

	if err := d.store.RecordEventReactions(ctx, record, reactions); err != nil {
		return err
	}
	d.notify()
	return nil
}

func (d *reactionDispatcher) Drain(ctx context.Context) error {
	d.drainPending(ctx)
	if err := waitContext(ctx, d.pending.Wait); err != nil {
		if log := d.log(ctx); log != nil {
			log.ErrorContext(ctx, "drain canceled", "err", err)
		}
		return err
	}
	return nil
}

func (d *reactionDispatcher) Shutdown(ctx context.Context) error {
	d.mu.Lock()
	if d.started {
		d.cancel()
		d.started = false
	}
	d.mu.Unlock()

	if err := waitContext(ctx, d.pollerWG.Wait); err != nil {
		if log := d.log(ctx); log != nil {
			log.ErrorContext(ctx, "reaction poller shutdown canceled", "err", err)
		}
		return err
	}

	d.mu.Lock()
	if d.jobs != nil {
		close(d.jobs)
		d.jobs = nil
	}
	d.mu.Unlock()

	if err := waitContext(ctx, d.workerWG.Wait); err != nil {
		if log := d.log(ctx); log != nil {
			log.ErrorContext(ctx, "reaction worker shutdown canceled", "err", err)
		}
		return err
	}
	return nil
}

func (d *reactionDispatcher) notify() {
	d.mu.Lock()
	wake := d.wake
	d.mu.Unlock()
	if wake == nil {
		return
	}

	select {
	case wake <- struct{}{}:
	default:
	}
}

func (d *reactionDispatcher) poll(ctx context.Context) {
	defer d.pollerWG.Done()

	ticker := time.NewTicker(d.pollInterval)
	defer ticker.Stop()

	for {
		d.drainPending(ctx)

		select {
		case <-ctx.Done():
			return
		case <-d.wake:
		case <-ticker.C:
		}
	}
}

func (d *reactionDispatcher) drainPending(ctx context.Context) {
	for {
		reactions, err := d.store.ListPendingReactions(ctx, d.pollLimit)
		if err != nil {
			if log := d.log(ctx); log != nil && !errors.Is(ctx.Err(), context.Canceled) {
				log.ErrorContext(ctx, "list pending reactions failed", "err", err)
			}
			return
		}
		if len(reactions) == 0 {
			return
		}

		queued := false
		for _, reaction := range reactions {
			if ctx.Err() != nil {
				return
			}
			if d.enqueue(ctx, reaction) {
				queued = true
			}
		}
		if !queued || len(reactions) < d.pollLimit {
			return
		}
	}
}

func (d *reactionDispatcher) enqueue(ctx context.Context, reaction ReactionRecord) bool {
	if !d.markInFlight(reaction.ID) {
		return false
	}
	d.pending.Add(1)

	eventRecord, err := d.store.Get(ctx, reaction.EventID)
	if err != nil {
		d.pending.Done()
		d.clearInFlight(reaction.ID)
		if errors.Is(err, ErrEventNotFound) {
			_ = d.store.MarkReactionFailed(ctx, reaction.ID)
		}
		if log := d.log(ctx); log != nil {
			log.ErrorContext(ctx, "load reaction event failed", "reaction_id", reaction.ID, "event_id", reaction.EventID, "err", err)
		}
		return false
	}

	handler, ok := d.handlerLookup(eventRecord.Type, reaction.HandlerName)
	if !ok {
		d.pending.Done()
		d.clearInFlight(reaction.ID)
		return false
	}
	claimed, err := d.store.ClaimReaction(ctx, reaction.ID)
	if err != nil {
		d.pending.Done()
		d.clearInFlight(reaction.ID)
		if log := d.log(ctx); log != nil {
			log.ErrorContext(ctx, "claim reaction failed", "reaction_id", reaction.ID, "err", err)
		}
		return false
	}
	if !claimed {
		d.pending.Done()
		d.clearInFlight(reaction.ID)
		return false
	}

	event, err := handler.decode(eventRecord)
	if err != nil {
		d.pending.Done()
		d.clearInFlight(reaction.ID)
		_ = d.store.MarkReactionFailed(ctx, reaction.ID)
		if log := d.log(ctx); log != nil {
			log.ErrorContext(ctx, "decode reaction event failed", "reaction_id", reaction.ID, "event_id", reaction.EventID, "err", err)
		}
		return false
	}

	select {
	case d.jobs <- reactionJob{ctx: ctx, reaction: reaction, event: event, handler: handler}:
		return true
	case <-ctx.Done():
		d.pending.Done()
		d.clearInFlight(reaction.ID)
		return false
	}
}

func (d *reactionDispatcher) markInFlight(id string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	if _, ok := d.inFlight[id]; ok {
		return false
	}
	d.inFlight[id] = struct{}{}
	return true
}

func (d *reactionDispatcher) clearInFlight(id string) {
	d.mu.Lock()
	delete(d.inFlight, id)
	d.mu.Unlock()
}

func (d *reactionDispatcher) work() {
	for job := range d.jobs {
		d.run(job)
	}
}

func (d *reactionDispatcher) run(job reactionJob) {
	defer d.pending.Done()
	defer d.clearInFlight(job.reaction.ID)

	if err := d.callHandler(job); err != nil {
		if markErr := d.store.MarkReactionFailed(job.ctx, job.reaction.ID); markErr != nil {
			if log := d.log(job.ctx); log != nil {
				log.ErrorContext(job.ctx, "mark reaction failed failed", "reaction_id", job.reaction.ID, "err", markErr)
			}
		}
		if log := d.log(job.ctx); log != nil {
			log.ErrorContext(job.ctx, "handler failed",
				"handler", job.handler.name,
				"event_type", job.event.Type(),
				"event_id", job.event.ID(),
				"reaction_id", job.reaction.ID,
				"err", err,
			)
		}
		if d.errorHook != nil {
			d.errorHook(job.ctx, job.event, job.handler.name, err)
		}
		return
	}

	if err := d.store.MarkReactionSucceeded(job.ctx, job.reaction.ID); err != nil {
		if log := d.log(job.ctx); log != nil {
			log.ErrorContext(job.ctx, "mark reaction succeeded failed", "reaction_id", job.reaction.ID, "err", err)
		}
	}
}

func (d *reactionDispatcher) callHandler(job reactionJob) (err error) {
	defer func() {
		if v := recover(); v != nil {
			err = fmt.Errorf("handler panicked: %v", v)
		}
	}()
	return job.handler.handle(job.ctx, job.event)
}

func (d *reactionDispatcher) log(context.Context) *slog.Logger {
	return d.logger
}

type reactionJob struct {
	ctx      context.Context
	reaction ReactionRecord
	event    AnyEvent
	handler  registeredHandler
}
