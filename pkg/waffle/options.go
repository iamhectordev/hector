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
