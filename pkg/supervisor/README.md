# Supervisor

Supervisor runs long-lived modules and coordinates graceful shutdown.

A module implements:

- `Name() string`
- `Init(ctx context.Context) error`
- `Start(ctx context.Context) error`
- `Stop(ctx context.Context) error`

Supervisor calls `Init` on all modules in registration order before starting any of them. If any `Init` fails, `Run` returns immediately with `ReasonInitError` and no modules are started.

After all modules are initialised, Supervisor starts them concurrently, waits for the first stop condition, and then shuts everything down in a controlled order.

Stop conditions include:

- init failed (`Init` returned an error)
- module returned an error
- module panicked
- module stopped unexpectedly (`Start` returned `nil`)
- signal shutdown
- context cancellation

```go
ctx, stopSignals := supervisor.NotifyContext(context.Background())
defer stopSignals()

sv, err := supervisor.New(
	[]supervisor.Module{
		agentModule,
		tuiModule,
	},
	supervisor.WithStopTimeout(5*time.Second),
	supervisor.WithPreStopHook("bus.drain", bus.Drain),
	supervisor.WithPostStopHook("bus.shutdown", bus.Shutdown),
)
if err != nil {
	return err
}

report := sv.Run(ctx)
if err := report.Err(); err != nil {
	return err
}
```

## Signal handling

There are two supported patterns:

1. Main owns signals  
   Use `supervisor.NotifyContext(...)` in main and pass that context to `Run`.

2. Supervisor owns signals  
   Use `supervisor.WithSignalHandling(...)` and call `Run` with a regular context.

Both work. Pick one pattern per execution path.

## Shutdown hooks

Hooks let you run non-module shutdown steps around module stop:

- pre-stop hooks run before module `Stop`
- post-stop hooks run after module `Stop`

Use them for things like draining queues, closing clients, and final cleanup.

## Report

`Run` returns a `Report` with:

- stop reason (`signal`, `module_error`, `module_panic`, etc.)
- trigger module (when relevant)
- signal/cause (when available)
- pre-stop, stop, and post-stop errors
- total shutdown duration

`Report.Err()` joins relevant shutdown errors into a single error value.

