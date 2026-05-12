package waffle

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"

	"github.com/sourcegraph/conc"
)

// ErrClosed is returned when recording to a shut down bus.
var ErrClosed = errors.New("waffle: event bus is closed")

// ErrNotStarted is returned when recording before the event bus is started.
var ErrNotStarted = errors.New("waffle: event bus is not started")

// ErrNilOption is returned when NewEventBus receives a nil option.
var ErrNilOption = errors.New("waffle: option cannot be nil")

// ErrorHook is called when a handler returns an error.
type ErrorHook func(ctx context.Context, event AnyEvent, handlerName string, err error)

// EventBus records events and dispatches matching handlers asynchronously.
type EventBus struct {
	mu        sync.Mutex
	stateMu   sync.RWMutex
	pending   sync.WaitGroup
	workers   int
	workerWG  conc.WaitGroup
	logger    *slog.Logger
	store     Store
	errorHook ErrorHook

	started  bool
	closed   bool
	jobs     chan job
	handlers map[string][]registeredHandler
}

// NewEventBus creates an in-memory event bus.
func NewEventBus(options ...Option) (*EventBus, error) {
	bus := &EventBus{
		workers:  1,
		store:    NewMemoryStore(),
		handlers: make(map[string][]registeredHandler),
	}

	for _, option := range options {
		if option == nil {
			return nil, ErrNilOption
		}
		if err := option(bus); err != nil {
			return nil, err
		}
	}

	return bus, nil
}

// Reader returns a reader backed by this bus's store.
func (b *EventBus) Reader() *Reader {
	return NewReader(b.store)
}

// Record appends an event and queues matching handlers.
func (b *EventBus) Record(ctx context.Context, event AnyEvent) error {
	b.stateMu.RLock()
	defer b.stateMu.RUnlock()

	if b.closed {
		if log := b.log(ctx); log != nil {
			log.ErrorContext(ctx, "record rejected on closed bus", "event_type", event.Type(), "err", ErrClosed)
		}
		return ErrClosed
	}
	if !b.started {
		if log := b.log(ctx); log != nil {
			log.ErrorContext(ctx, "record rejected on stopped bus", "event_type", event.Type(), "err", ErrNotStarted)
		}
		return ErrNotStarted
	}

	record, err := eventRecord(event)
	if err != nil {
		if log := b.log(ctx); log != nil {
			log.ErrorContext(ctx, "record encoding failed", "event_type", event.Type(), "err", err)
		}
		return err
	}

	if err := b.store.Append(ctx, record); err != nil {
		if log := b.log(ctx); log != nil {
			log.ErrorContext(ctx, "record append failed", "event_type", event.Type(), "err", err)
		}
		return err
	}

	b.mu.Lock()
	handlers := append([]registeredHandler(nil), b.handlers[event.Type()]...)
	b.mu.Unlock()

	for _, handler := range handlers {
		b.pending.Add(1)

		select {
		case b.jobs <- job{ctx: ctx, event: event, handler: handler}:
		case <-ctx.Done():
			b.pending.Done()
			if log := b.log(ctx); log != nil {
				log.ErrorContext(ctx, "record canceled while queueing handler", "event_type", event.Type(), "err", ctx.Err())
			}
			return ctx.Err()
		}
	}

	return nil
}

func eventRecord(event AnyEvent) (EventRecord, error) {
	payload, err := eventPayload(event)
	if err != nil {
		return EventRecord{}, err
	}

	return EventRecord{
		ID:            event.ID(),
		Type:          event.Type(),
		SchemaVersion: event.SchemaVersion(),
		OccurredAt:    event.OccurredAt(),
		Payload:       payload,
	}, nil
}

func eventPayload(event AnyEvent) ([]byte, error) {
	payload, err := json.Marshal(event.Payload())
	if err != nil {
		return nil, err
	}

	return payload, nil
}

// Start begins dispatching recorded events to handlers.
func (b *EventBus) Start(ctx context.Context) error {
	b.stateMu.Lock()
	defer b.stateMu.Unlock()

	if b.closed {
		return ErrClosed
	}
	if b.started {
		return nil
	}

	b.jobs = make(chan job, b.workers*64)
	b.start()
	b.started = true
	if log := b.log(ctx); log != nil {
		log.InfoContext(ctx, "event bus started", "workers", b.workers)
	}
	return nil
}

// Drain waits until all queued and running handlers finish.
func (b *EventBus) Drain(ctx context.Context) error {
	if err := waitContext(ctx, b.pending.Wait); err != nil {
		if log := b.log(ctx); log != nil {
			log.ErrorContext(ctx, "drain canceled", "err", err)
		}
		return err
	}
	return nil
}

// Shutdown stops accepting events and waits for workers to exit.
func (b *EventBus) Shutdown(ctx context.Context) error {
	b.stateMu.Lock()
	if !b.closed {
		b.closed = true
		if b.started {
			close(b.jobs)
		}
		if log := b.log(ctx); log != nil {
			log.InfoContext(ctx, "event bus shutting down")
		}
	}
	b.stateMu.Unlock()

	if err := waitContext(ctx, b.workerWG.Wait); err != nil {
		if log := b.log(ctx); log != nil {
			log.ErrorContext(ctx, "shutdown canceled", "err", err)
		}
		return err
	}
	return nil
}

func (b *EventBus) register(eventType string, handler registeredHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.handlers[eventType] = append(b.handlers[eventType], handler)
}

func (b *EventBus) start() {
	for range b.workers {
		b.workerWG.Go(func() {
			for job := range b.jobs {
				if err := job.handler.handle(job.ctx, job.event); err != nil {
					if log := b.log(job.ctx); log != nil {
						log.ErrorContext(job.ctx, "handler failed",
							"handler", job.handler.name,
							"event_type", job.event.Type(),
							"event_id", job.event.ID(),
							"err", err,
						)
					}
					if b.errorHook != nil {
						b.errorHook(job.ctx, job.event, job.handler.name, err)
					}
				}

				b.pending.Done()
			}
		})
	}
}

func (b *EventBus) log(context.Context) *slog.Logger {
	return b.logger
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
