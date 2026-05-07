package waffle

// Option configures an EventBus.
type Option func(*EventBus)

// WithWorkers sets the number of handler workers.
func WithWorkers(n int) Option {
	return func(bus *EventBus) {
		if n < 1 {
			panic("waffle: workers must be positive")
		}

		bus.workers = n
	}
}
