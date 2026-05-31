package app

import (
	"fmt"
	"log/slog"
)

// Profile selects which user-facing surfaces the runtime starts.
type Profile string

const (
	// ProfileServe starts the Slack bot runtime.
	ProfileServe Profile = "serve"
	// ProfileChat starts the local interactive chat runtime.
	ProfileChat Profile = "chat"
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

// WithProfile sets the runtime profile.
func WithProfile(profile Profile) Option {
	return func(r *Runtime) error {
		switch profile {
		case ProfileServe, ProfileChat:
			r.profile = profile
			return nil
		default:
			return fmt.Errorf("app: unsupported profile %q", profile)
		}
	}
}
