# Logging

Structured `slog` logs with tags only.

- Get logger from context: `log.FromCtx(ctx)`
- Always log errors as `("err", err)`
- Use snake_case keys
- Keep messages short
- Do not log secrets or raw user content

Levels:
- `debug` diagnostics
- `info` normal lifecycle
- `warn` degraded/recovered behavior
- `error` failed operation

Pattern:
- Add a private `log(ctx)` method on important types/components
- `log(ctx)` adds type-specific tags via `.With(...)`
- Do not rewrite logger into context on normal call paths

Example (module):
```go
func (m *Module) log(ctx context.Context) *slog.Logger {
	return log.FromCtx(ctx).With("component", "module", "module", m.Name())
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
