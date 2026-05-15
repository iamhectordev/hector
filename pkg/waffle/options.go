package waffle

import (
	"fmt"
	"log/slog"
)

// Option configures an EventBus.
type Option func(*EventBus) error

// WithWorkers sets the number of handler workers.
func WithWorkers(n int) Option {
	return func(bus *EventBus) error {
		if n < 1 {
			return fmt.Errorf("waffle: workers must be positive")
		}

		bus.workers = n
		return nil
	}
}

// WithLogger enables runtime logging for the event bus.
func WithLogger(l *slog.Logger) Option {
	return func(bus *EventBus) error {
		if l == nil {
			bus.logger = nil
			return nil
		}
		bus.logger = l.With("component", "waffle")
		return nil
	}
}

// WithErrorHook sets a function called whenever a handler returns an error.
// The hook is called in addition to the built-in log line.
func WithErrorHook(hook ErrorHook) Option {
	return func(bus *EventBus) error {
		if hook == nil {
			return fmt.Errorf("waffle: error hook cannot be nil")
		}
		bus.errorHook = hook
		return nil
	}
}

// WithStore sets the event store used by Record.
func WithStore(store Store) Option {
	return func(bus *EventBus) error {
		if store == nil {
			return fmt.Errorf("waffle: store cannot be nil")
		}

		bus.store = store
		return nil
	}
}

// WithPersistentReactions makes handler reactions durable.
func WithPersistentReactions() Option {
	return func(bus *EventBus) error {
		bus.persistent = true
		return nil
	}
}
