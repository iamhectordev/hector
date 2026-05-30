package app

import (
	"fmt"
	"log/slog"
)

// Option configures a Runtime.
type Option func(*Runtime) error

// WithLogger sets the logger used by the runtime.
func WithLogger(logger *slog.Logger) Option {
	return func(r *Runtime) error {
		if logger == nil {
			return fmt.Errorf("app: logger is required")
		}
		r.logger = logger
		return nil
	}
}
