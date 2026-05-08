package supervisor

import (
	"fmt"
	"log/slog"
	"os"
	"time"
)

const defaultStopTimeout = 30 * time.Second

type config struct {
	stopTimeout    time.Duration
	signalHandling bool
	signals        []os.Signal
	signalChan     <-chan os.Signal
	logger         *slog.Logger
}

// Option configures [Supervisor].
type Option func(*config) error

// WithStopTimeout sets the per-module deadline for [Module.Stop].
// Values <= 0 are rejected by [New].
func WithStopTimeout(d time.Duration) Option {
	return func(c *config) error {
		if d <= 0 {
			return fmt.Errorf("supervisor: stop timeout must be positive, got %v", d)
		}
		c.stopTimeout = d
		return nil
	}
}

// WithSignalHandling enables OS signal handling inside [Supervisor.Run].
// If signals are not provided, default platform signals are used.
func WithSignalHandling(signals ...os.Signal) Option {
	return func(c *config) error {
		c.signalHandling = true
		c.signals = append([]os.Signal(nil), signals...)
		return nil
	}
}

// WithSignalChan supplies a signal source instead of [os/signal.Notify].
// In tests, prefer this (for example a buffered channel) so [Supervisor.Run] does not
// call [os/signal.Notify], which can deadlock [testing/synctest] test bubbles.
func WithSignalChan(ch <-chan os.Signal) Option {
	return func(c *config) error {
		c.signalChan = ch
		return nil
	}
}

// WithLogger enables lifecycle logging via [slog].
func WithLogger(l *slog.Logger) Option {
	return func(c *config) error {
		c.logger = l
		return nil
	}
}

func applyDefaults(c *config) {
	if c.stopTimeout == 0 {
		c.stopTimeout = defaultStopTimeout
	}
}
