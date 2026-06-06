package waffle

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"reflect"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"

	"github.com/iamhectordev/hector/pkg/telem"
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
	mu         sync.Mutex
	stateMu    sync.RWMutex
	workers    int
	logger     *slog.Logger
	store      Store
	errorHook  ErrorHook
	dispatcher dispatcher
	persistent bool

	started      bool
	closed       bool
	handlers     map[string][]registeredHandler
	payloadTypes map[string]reflect.Type
}

// NewEventBus creates an in-memory event bus.
func NewEventBus(options ...Option) (*EventBus, error) {
	bus := &EventBus{
		workers:      1,
		store:        NewMemoryStore(),
		handlers:     make(map[string][]registeredHandler),
		payloadTypes: make(map[string]reflect.Type),
	}

	for _, option := range options {
		if option == nil {
			return nil, ErrNilOption
		}
		if err := option(bus); err != nil {
			return nil, err
		}
	}

	if bus.persistent {
		reactionStore, ok := bus.store.(ReactionStore)
		if !ok {
			return nil, errors.New("waffle: persistent reactions require a reaction store")
		}
		bus.dispatcher = newReactionDispatcher(bus.workers, bus.logger, bus.errorHook, reactionStore, bus.handler)
	} else {
		bus.dispatcher = newMemoryDispatcher(bus.workers, bus.logger, bus.errorHook, bus.store)
	}
	return bus, nil
}

// Reader returns a reader backed by this bus's store.
func (b *EventBus) Reader() *Reader {
	return NewReader(b.store)
}

// Record appends an event and queues matching handlers.
func (b *EventBus) Record(ctx context.Context, event AnyEvent) (err error) {
	ctx, span := telem.Trace(ctx, spanEventRecord, eventFields(event)...)
	defer span.End(&err)

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

	record, err := eventRecord(ctx, event)
	if err != nil {
		if log := b.log(ctx); log != nil {
			log.ErrorContext(ctx, "record encoding failed", "event_type", event.Type(), "err", err)
		}
		return err
	}

	b.mu.Lock()
	handlers := append([]registeredHandler(nil), b.handlers[event.Type()]...)
	b.mu.Unlock()
	telem.Event(ctx, "waffle.handlers.selected", telem.Int("waffle.handler.count", len(handlers)))

	if err = b.dispatcher.Dispatch(ctx, event, record, handlers); err != nil {
		if log := b.log(ctx); log != nil {
			log.ErrorContext(ctx, "record dispatch failed", "event_type", event.Type(), "err", err)
		}
		return err
	}
	return nil
}

func eventRecord(ctx context.Context, event AnyEvent) (EventRecord, error) {
	payload, err := eventPayload(event)
	if err != nil {
		return EventRecord{}, err
	}

	headers := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, headers)

	return EventRecord{
		ID:            event.ID(),
		Type:          event.Type(),
		SchemaVersion: event.SchemaVersion(),
		OccurredAt:    event.OccurredAt(),
		Payload:       payload,
		Headers:       map[string]string(headers),
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

	if err := b.dispatcher.Start(ctx); err != nil {
		return err
	}
	b.started = true
	return nil
}

// Drain waits until all queued and running handlers finish.
func (b *EventBus) Drain(ctx context.Context) error {
	return b.dispatcher.Drain(ctx)
}

// Shutdown stops accepting events and waits for workers to exit.
func (b *EventBus) Shutdown(ctx context.Context) error {
	b.stateMu.Lock()
	wasStarted := b.started
	if !b.closed {
		b.closed = true
		if log := b.log(ctx); log != nil {
			log.InfoContext(ctx, "event bus shutting down")
		}
	}
	b.stateMu.Unlock()

	if wasStarted {
		return b.dispatcher.Shutdown(ctx)
	}
	return nil
}

func (b *EventBus) register(eventType string, payloadType reflect.Type, handler registeredHandler) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if existing, ok := b.payloadTypes[eventType]; ok && existing != payloadType {
		return errors.New("waffle: event type registered with different payload type")
	}
	b.payloadTypes[eventType] = payloadType
	b.handlers[eventType] = append(b.handlers[eventType], handler)
	return nil
}

func (b *EventBus) handler(eventType, name string) (registeredHandler, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, handler := range b.handlers[eventType] {
		if handler.name == name {
			return handler, true
		}
	}
	return registeredHandler{}, false
}

func (b *EventBus) log(context.Context) *slog.Logger {
	return b.logger
}
