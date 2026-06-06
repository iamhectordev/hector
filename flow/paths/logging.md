# Logging

Structured `slog` logs with tags only.

- Runtime/app code stores the base logger in context with `telem.WithLogger`.
- Get a correlated logger from context with `telem.Logger(ctx)`.
- Always log errors as `("err", err)`
- Use snake_case keys
- Keep messages short
- Do not log secrets or raw user content
- Logs should include trace/span IDs and baggage automatically when the context carries them.

Boundary:
- App/runtime code can use app logging helpers (for example context logger helpers)
- Contained libraries/packages must not depend on app-specific logging packages
- Contained packages use `log/slog` only and accept logger injection via options

Levels:
- `debug` diagnostics
- `info` normal lifecycle
- `warn` degraded/recovered behavior
- `error` failed operation

Pattern:
- Add a private `log(ctx)` method on important types/components
- `log(ctx)` adds type-specific tags via `.With(...)`
- Do not store logger fields on modules unless a package is still on the old pattern.
- Do not rewrite logger into context on normal call paths, except at entry points that install the base logger.

Example (module):
```go
func (m *Module) log(ctx context.Context) *slog.Logger {
	return telem.Logger(ctx).With("component", "module", "module", m.Name())
}
```

Example (supervisor):
```go
func (s *Supervisor) log(ctx context.Context) *slog.Logger {
	return log.FromCtx(ctx).With("component", "supervisor")
}
```

Usage:
```go
m.log(ctx).InfoContext(ctx, "starting")
s.log(ctx).ErrorContext(ctx, "stop failed", "module", name, "err", err)
```
