package waffle

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
)

// Handler processes a typed event.
type Handler[T any] func(context.Context, Event[T]) error

// Binding connects an event definition to one or more handlers.
type Binding[T any] struct {
	bus *EventBus
	def Definition[T]
}

// On starts handler registration for an event definition.
func On[T any](bus *EventBus, def Definition[T]) Binding[T] {
	return Binding[T]{
		bus: bus,
		def: def,
	}
}

// Handle registers a named handler for the binding's event definition.
func (b Binding[T]) Handle(name string, handler Handler[T]) error {
	if name == "" {
		return fmt.Errorf("waffle: handler name cannot be empty")
	}
	if handler == nil {
		return fmt.Errorf("waffle: handler cannot be nil")
	}

	payloadType := reflect.TypeFor[T]()
	return b.bus.register(b.def.Type(), payloadType, registeredHandler{
		name:      name,
		eventType: b.def.Type(),
		decode: func(record EventRecord) (AnyEvent, error) {
			var data T
			if err := json.Unmarshal(record.Payload, &data); err != nil {
				return nil, fmt.Errorf("waffle: decode event %q for handler %q: %w", record.ID, name, err)
			}

			return event[T]{
				id:            record.ID,
				eventType:     record.Type,
				schemaVersion: record.SchemaVersion,
				occurredAt:    record.OccurredAt,
				data:          data,
			}, nil
		},
		handle: func(ctx context.Context, raw AnyEvent) error {
			event, ok := raw.(Event[T])
			if !ok {
				return fmt.Errorf("waffle: handler %q received event with unexpected payload type", name)
			}

			return handler(ctx, event)
		},
	})
}

type registeredHandler struct {
	name      string
	eventType string
	decode    func(EventRecord) (AnyEvent, error)
	handle    func(context.Context, AnyEvent) error
}
