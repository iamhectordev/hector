package supervisor

import (
	"errors"
	"fmt"
	"os"
	"time"
)

// StopReason explains why [Supervisor.Run] initiated shutdown.
type StopReason string

const (
	ReasonSignal          StopReason = "signal"
	ReasonModulePanic     StopReason = "module_panic"
	ReasonModuleError     StopReason = "module_error"
	ReasonModuleStopped   StopReason = "module_stopped"
	ReasonContextCanceled StopReason = "context_canceled"
	ReasonInitError       StopReason = "init_error"
)

// Report is the outcome of [Supervisor.Run].
type Report struct {
	Reason StopReason

	// TriggerModule is set when Reason is module-related.
	TriggerModule string

	Signal os.Signal
	Cause  error

	PanicValue any

	ShutdownDuration time.Duration

	// PreStopErrors maps pre-stop hook name to failure.
	PreStopErrors map[string]error

	// StopErrors maps module name to stop failure (timeout or returned error).
	StopErrors map[string]error

	// PostStopErrors maps post-stop hook name to failure.
	PostStopErrors map[string]error
}

// Err returns a non-nil error when the run failed or any shutdown step failed.
// Clean exits ([ReasonSignal] without hook/module stop errors) return nil.
func (r Report) Err() error {
	var errs []error
	switch r.Reason {
	case ReasonModuleError:
		if r.Cause != nil {
			errs = append(errs, r.Cause)
		}
	case ReasonContextCanceled:
		if r.Cause != nil {
			errs = append(errs, r.Cause)
		}
	case ReasonModulePanic:
		errs = append(errs, fmt.Errorf("module %s panicked: %v", r.TriggerModule, r.PanicValue))
	case ReasonInitError:
		if r.Cause != nil {
			errs = append(errs, r.Cause)
		}
	}
	for name, err := range r.PreStopErrors {
		errs = append(errs, fmt.Errorf("pre-stop hook %s: %w", name, err))
	}
	for name, err := range r.StopErrors {
		errs = append(errs, fmt.Errorf("module %s: stop: %w", name, err))
	}
	for name, err := range r.PostStopErrors {
		errs = append(errs, fmt.Errorf("post-stop hook %s: %w", name, err))
	}
	return errors.Join(errs...)
}
