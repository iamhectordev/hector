package waffle

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/sourcegraph/conc"
)

type dispatcher interface {
	Start(context.Context) error
	Dispatch(context.Context, AnyEvent, EventRecord, []registeredHandler) error
	Drain(context.Context) error
	Shutdown(context.Context) error
}

type memoryDispatcher struct {
	workers   int
	logger    *slog.Logger
	errorHook ErrorHook
	store     EventWriter

	mu       sync.Mutex
	started  bool
	jobs     chan job
	pending  sync.WaitGroup
	workerWG conc.WaitGroup
}

func newMemoryDispatcher(workers int, logger *slog.Logger, errorHook ErrorHook, store EventWriter) *memoryDispatcher {
	return &memoryDispatcher{
		workers:   workers,
		logger:    logger,
		errorHook: errorHook,
		store:     store,
	}
}

func (d *memoryDispatcher) Start(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.started {
		return nil
	}

	d.jobs = make(chan job, d.workers*64)
	for range d.workers {
		d.workerWG.Go(d.work)
	}
	d.started = true

	if log := d.log(ctx); log != nil {
		log.InfoContext(ctx, "event bus started", "workers", d.workers)
	}
	return nil
}

func (d *memoryDispatcher) Dispatch(ctx context.Context, event AnyEvent, record EventRecord, handlers []registeredHandler) error {
	if err := d.store.Append(ctx, record); err != nil {
		return err
	}

	for _, handler := range handlers {
		d.pending.Add(1)

		select {
		case d.jobs <- job{ctx: ctx, event: event, handler: handler}:
		case <-ctx.Done():
			d.pending.Done()
			if log := d.log(ctx); log != nil {
				log.ErrorContext(ctx, "record canceled while queueing handler", "event_type", event.Type(), "err", ctx.Err())
			}
			return ctx.Err()
		}
	}

	return nil
}

func (d *memoryDispatcher) Drain(ctx context.Context) error {
	if err := waitContext(ctx, d.pending.Wait); err != nil {
		if log := d.log(ctx); log != nil {
			log.ErrorContext(ctx, "drain canceled", "err", err)
		}
		return err
	}
	return nil
}

func (d *memoryDispatcher) Shutdown(ctx context.Context) error {
	d.mu.Lock()
	if d.started {
		close(d.jobs)
		d.started = false
	}
	d.mu.Unlock()

	if err := waitContext(ctx, d.workerWG.Wait); err != nil {
		if log := d.log(ctx); log != nil {
			log.ErrorContext(ctx, "shutdown canceled", "err", err)
		}
		return err
	}
	return nil
}

func (d *memoryDispatcher) work() {
	for job := range d.jobs {
		d.run(job)
	}
}

func (d *memoryDispatcher) run(job job) {
	defer d.pending.Done()

	if err := d.callHandler(job); err != nil {
		if log := d.log(job.ctx); log != nil {
			log.ErrorContext(job.ctx, "handler failed",
				"handler", job.handler.name,
				"event_type", job.event.Type(),
				"event_id", job.event.ID(),
				"err", err,
			)
		}
		if d.errorHook != nil {
			d.errorHook(job.ctx, job.event, job.handler.name, err)
		}
	}
}

func (d *memoryDispatcher) callHandler(job job) (err error) {
	defer func() {
		if v := recover(); v != nil {
			err = fmt.Errorf("handler panicked: %v", v)
		}
	}()
	return job.handler.handle(job.ctx, job.event)
}

func (d *memoryDispatcher) log(context.Context) *slog.Logger {
	return d.logger
}

type job struct {
	ctx     context.Context
	event   AnyEvent
	handler registeredHandler
}

func waitContext(ctx context.Context, wait func()) error {
	done := make(chan struct{})
	go func() {
		defer close(done)
		wait()
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
