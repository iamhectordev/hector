// Package supervisor runs named modules with coordinated startup and graceful shutdown.
//
// A module is any long-running component that implements:
//   - Name() string
//   - Start(ctx) error
//   - Stop(ctx) error
//
// The supervisor starts all modules, then waits for the first terminal condition:
// module panic, module error, module stopped, signal-caused cancel, or parent cancel.
// After that it cancels the shared run context and calls Stop on all modules in reverse
// order with a per-module timeout.
//
// # Signal ownership
//
// Pick one owner per process path:
//   - Main-owned: use NotifyContext in main and pass the returned context into Run.
//   - Supervisor-owned: set WithSignalHandling and let Run install signal listeners.
//
// Use WithSignalChan only in tests when you need deterministic signal triggering.
package supervisor
