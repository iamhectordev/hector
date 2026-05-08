package supervisor

import (
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

	// StopErrors maps module name to stop failure (timeout or returned error).
	StopErrors map[string]error
}
