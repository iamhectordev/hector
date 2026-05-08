package waffle

import "fmt"

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
