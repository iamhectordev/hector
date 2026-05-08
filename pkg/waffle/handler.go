package waffle

import (
	"context"
	"fmt"
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

	b.bus.register(b.def.Type(), registeredHandler{
		name: name,
		handle: func(ctx context.Context, raw any) error {
			event, ok := raw.(Event[T])
			if !ok {
				return fmt.Errorf("waffle: handler %q received event with unexpected payload type", name)
			}

			return handler(ctx, event)
		},
	})

	return nil
}

type registeredHandler struct {
	name   string
	handle func(context.Context, any) error
}
