package waffle

import (
	"context"
	"errors"
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

	return bus, nil
}

// Record appends an event and queues matching handlers.
func (b *EventBus) Record(ctx context.Context, event AnyEvent) error {
	b.stateMu.RLock()
	defer b.stateMu.RUnlock()

	if b.closed {
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
			return ctx.Err()
		}
	}

	return nil
}

// Drain waits until all queued and running handlers finish.
func (b *EventBus) Drain(ctx context.Context) error {
	if err := waitContext(ctx, b.pending.Wait); err != nil {
		return err
	}

	return b.takeErrors()
}

// Shutdown stops accepting events and waits for workers to exit.
func (b *EventBus) Shutdown(ctx context.Context) error {
	b.stateMu.Lock()
	if !b.closed {
		b.closed = true
		close(b.jobs)
	}
	b.stateMu.Unlock()

	if err := waitContext(ctx, b.workerWG.Wait); err != nil {
		return err
	}

	return b.takeErrors()
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
				}

				b.pending.Done()
			}
		})
	}
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
