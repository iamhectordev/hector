package waffle

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"github.com/sourcegraph/conc"
)

// ErrClosed is returned when recording to a shut down bus.
var ErrClosed = errors.New("waffle: event bus is closed")

// ErrNilOption is returned when NewEventBus receives a nil option.
var ErrNilOption = errors.New("waffle: option cannot be nil")

// EventBus records events and dispatches matching handlers asynchronously.
type EventBus struct {
	mu       sync.Mutex
	stateMu  sync.RWMutex
	pending  sync.WaitGroup
	workers  int
	workerWG conc.WaitGroup
	logger   *slog.Logger

	closed   bool
	jobs     chan job
	events   []AnyEvent
	handlers map[string][]registeredHandler
	errs     []error
}

// NewEventBus creates an in-memory event bus.
func NewEventBus(options ...Option) (*EventBus, error) {
	bus := &EventBus{
		workers:  1,
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

	bus.jobs = make(chan job, bus.workers*64)
	bus.start()
	if log := bus.log(context.Background()); log != nil {
		log.Info("event bus started", "workers", bus.workers)
	}

	return bus, nil
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

	b.mu.Lock()
	b.events = append(b.events, event)
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

// Drain waits until all queued and running handlers finish.
func (b *EventBus) Drain(ctx context.Context) error {
	if err := waitContext(ctx, b.pending.Wait); err != nil {
		if log := b.log(ctx); log != nil {
			log.ErrorContext(ctx, "drain canceled", "err", err)
		}
		return err
	}

	err := b.takeErrors()
	if err != nil {
		if log := b.log(ctx); log != nil {
			log.ErrorContext(ctx, "drain completed with handler errors", "err", err)
		}
	}
	return err
}

// Shutdown stops accepting events and waits for workers to exit.
func (b *EventBus) Shutdown(ctx context.Context) error {
	b.stateMu.Lock()
	if !b.closed {
		b.closed = true
		close(b.jobs)
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

	err := b.takeErrors()
	if err != nil {
		if log := b.log(ctx); log != nil {
			log.ErrorContext(ctx, "shutdown completed with handler errors", "err", err)
		}
	}
	return err
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
					b.addError(err)
					if log := b.log(job.ctx); log != nil {
						log.ErrorContext(job.ctx, "handler failed",
							"handler", job.handler.name,
							"event_type", job.event.Type(),
							"event_id", job.event.ID(),
							"err", err,
						)
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

func (b *EventBus) addError(err error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.errs = append(b.errs, err)
}

func (b *EventBus) takeErrors() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	err := errors.Join(b.errs...)
	b.errs = nil
	return err
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
