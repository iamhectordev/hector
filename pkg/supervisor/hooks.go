package supervisor

import (
	"context"
	"fmt"
)

// ShutdownHook runs during supervisor shutdown.
type ShutdownHook interface {
	Name() string
	Run(context.Context) error
}

type hookFunc struct {
	name string
	fn   func(context.Context) error
}

func (h hookFunc) Name() string {
	return h.name
}

func (h hookFunc) Run(ctx context.Context) error {
	return h.fn(ctx)
}

// WithPostInitHook registers a single hook that runs after all module Init calls
// and before any module Start call.
func WithPostInitHook(name string, fn func(context.Context) error) Option {
	return WithPostInitHooks(hookFunc{name: name, fn: fn})
}

// WithPreStopHook registers a single hook that runs before module Stop calls.
func WithPreStopHook(name string, fn func(context.Context) error) Option {
	return WithPreStopHooks(hookFunc{name: name, fn: fn})
}

// WithPostStopHook registers a single hook that runs after module Stop calls.
func WithPostStopHook(name string, fn func(context.Context) error) Option {
	return WithPostStopHooks(hookFunc{name: name, fn: fn})
}

// WithPreStopHooks registers hooks that run before module Stop calls.
func WithPreStopHooks(hooks ...ShutdownHook) Option {
	return func(c *config) error {
		validated, err := validateHooks("pre-stop", c.preStopHooks, hooks)
		if err != nil {
			return err
		}
		c.preStopHooks = validated
		return nil
	}
}

// WithPostInitHooks registers hooks that run after all module Init calls and
// before any module Start call.
func WithPostInitHooks(hooks ...ShutdownHook) Option {
	return func(c *config) error {
		validated, err := validateHooks("post-init", c.postInitHooks, hooks)
		if err != nil {
			return err
		}
		c.postInitHooks = validated
		return nil
	}
}

// WithPostStopHooks registers hooks that run after module Stop calls.
func WithPostStopHooks(hooks ...ShutdownHook) Option {
	return func(c *config) error {
		validated, err := validateHooks("post-stop", c.postStopHooks, hooks)
		if err != nil {
			return err
		}
		c.postStopHooks = validated
		return nil
	}
}

func validateHooks(phase string, existing []ShutdownHook, newHooks []ShutdownHook) ([]ShutdownHook, error) {
	seen := make(map[string]struct{}, len(existing)+len(newHooks))
	for _, hook := range existing {
		seen[hook.Name()] = struct{}{}
	}

	out := append([]ShutdownHook(nil), existing...)
	for _, hook := range newHooks {
		if hook == nil {
			return nil, fmt.Errorf("supervisor: %s hook cannot be nil", phase)
		}
		name := hook.Name()
		if name == "" {
			return nil, fmt.Errorf("supervisor: %s hook name cannot be empty", phase)
		}
		if _, ok := seen[name]; ok {
			return nil, fmt.Errorf("supervisor: duplicate %s hook name %q", phase, name)
		}
		if hf, ok := hook.(hookFunc); ok && hf.fn == nil {
			return nil, fmt.Errorf("supervisor: %s hook %q function cannot be nil", phase, name)
		}
		seen[name] = struct{}{}
		out = append(out, hook)
	}
	return out, nil
}
