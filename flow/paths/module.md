# Module

A long-running unit supervised by `pkg/supervisor`.

## Principles
- `Init(ctx)` is called for all modules before any `Start` — use it for one-time setup (auth, listener registration, resource allocation)
- `Start(ctx)` must block — returning signals the module has stopped
- `Stop(ctx)` runs after ctx is cancelled, in reverse registration order
- Modules are wired in the CLI action, not in main

## Outline
```go
func (m *Module) Name() string { return "my-module" }

func (m *Module) Init(ctx context.Context) error {
    // one-time setup: validate credentials, register handlers, etc.
    return nil
}

func (m *Module) Start(ctx context.Context) error {
    // start goroutines, then block
    <-ctx.Done()
    return nil
}

func (m *Module) Stop(ctx context.Context) error { ... }
```
