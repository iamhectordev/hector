# int1-02 — Integration contracts and host adapter

Design: `docs/designs/001-integration-facets.md`
Golden paths: `flow/paths/module.md`, `flow/paths/integration.md` (being replaced
by this slice), `flow/paths/logging.md`
Depends: int1-01

## Goal

Create the `integrations` package: the facet contracts and the generic host
adapter that turns any integration into a `supervisor.Module`. No integration is
migrated in this slice.

## Changes

### `integrations/integration.go`

```go
package integrations

// Integration is a capability unit connecting Hector to an external service.
type Integration interface{ Name() string }

type ToolProvider interface{ Tools() []tools.Tool }           // pkg/tools
type EventSource  interface{ Run(ctx context.Context) error } // must block until ctx is done or a fatal error
type Surface      interface{ ReplyHandler() comms.ReplyHandler }
type Initializer  interface{ Init(ctx context.Context) error }
```

`io.Closer` is used as-is for the teardown facet (no local alias needed unless
lint requires documentation).

Imports allowed in `integrations/...`: stdlib, `pkg/...`. Nothing under
`internal/` or `modules/`.

### `integrations/host.go`

```go
// NewHost wraps an integration as a supervisor.Module.
func NewHost(i Integration) (*Host, error) // error on nil integration or empty Name()

func (h *Host) Name() string                    // "integration." + i.Name()
func (h *Host) Init(ctx context.Context) error  // delegates to Initializer facet, else nil
func (h *Host) Start(ctx context.Context) error // EventSource: return i.Run(ctx); else block on <-ctx.Done(), return nil
func (h *Host) Stop(ctx context.Context) error  // io.Closer facet: i.Close(); else nil
```

Follow `flow/paths/module.md` semantics exactly: `Start` blocks; a `Run` that
returns a non-context error propagates (the supervisor decides what to do);
`ctx.Done()`-caused termination returns nil. Log via `telem.Logger(ctx)` with
`component=module, module=integration.<name>` (see existing modules for the
idiom).

### Tests — `integrations/host_test.go` (package `integrations_test`)

Blackbox, using a fake integration configurable to implement any facet subset:

1. Tools-only integration: `NewHost` succeeds; `Init`/`Stop` are no-ops; `Start`
   blocks until ctx cancel, then returns nil.
2. `Initializer` facet: `Init` is delegated; an `Init` error propagates.
3. `EventSource`: `Start` runs `Run`; cancelling ctx unblocks and returns nil
   when `Run` returns `ctx.Err()`; a non-context error from `Run` is returned.
4. `io.Closer`: `Stop` calls `Close` exactly once; a `Close` error propagates.
5. `NewHost(nil)` and empty `Name()` return errors.

Use `testify/require` and `t.Context()`.

### `flow/paths/integration.md` — rewrite

Replace the `Register(ctx, cfg, registry) (io.Closer, error)` pattern with the
facet model, following `flow/paths/TEMPLATE.md`. Keep the surviving principles:
explicit `Enabled bool` gate (never inferred from non-zero fields;
`validate:"required_if=Enabled true"` on required fields), config validated by
the constructor, secrets as references, cheap credential verification (now: the
`Initializer` facet), small public client API. Add: the facet list with one-line
semantics each, constructor injection of deps (bus, config), the import
discipline (only `pkg/...`), and a wiring sketch showing the runtime loop
(ToolProvider → registry at wiring time, Surface → reply router, NewHost →
supervisor).

## Non-goals

- No runtime wiring changes (`internal/app` untouched) — the wiring loop lands
  with the first migrated integration (int1-03).
- No gateway/route registrar, no adaptation layer.

## Gate

`mise run test && mise run lint`.
