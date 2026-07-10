# Integration

A capability unit connecting Hector to an external service, implemented as a
facet-based `Integration` and supervised via the generic `Host` adapter.

## Principles
- Every integration implements `Integration` (at minimum `Name()`).
- Optional behaviour is expressed as facets: `ToolProvider`, `EventSource`,
  `Surface`, `Initializer`, `io.Closer`. Not tiers — checkboxes.
- `Enabled bool` is the single explicit gate — never infer enablement from
  non-zero fields. Required fields use `validate:"required_if=Enabled true"`.
- Config is validated by the constructor; secrets appear as references, not
  inline values.
- Credential verification is cheap and happens via the `Initializer` facet.
- Integrations import only `pkg/...` (and stdlib / vendor SDKs) — never
  `internal/`, never `modules/`, never each other.
- Dependencies (bus, config) arrive by constructor injection.
- Integration config lives under `integrations.<name>` in app config; the
  sub-block names `tools`, `events`, and `auth` are reserved for future shared
  conventions — vendor-specific config must not use those names.

## Facets

| Facet | Semantics |
|---|---|
| `Integration` | Base contract: `Name() string`. Required. |
| `ToolProvider` | Exposes agent tools via `Tools() []tools.Tool`. |
| `EventSource` | Inbound event loop via `Run(ctx) error`. Must block until ctx is done or a fatal error. |
| `Surface` | Receives replies via `ReplyHandler() comms.ReplyHandler`. |
| `Initializer` | One-time setup (auth verify, client build) via `Init(ctx) error`. |
| `io.Closer` | Teardown of long-lived connections via `Close() error`. |

## Outline

```go
package integrations

type Integration interface{ Name() string }

type ToolProvider interface{ Tools() []tools.Tool }
type EventSource  interface{ Run(ctx context.Context) error }
type Surface      interface{ ReplyHandler() comms.ReplyHandler }
type Initializer  interface{ Init(ctx context.Context) error }

// host.go — turns any Integration into a supervisor.Module:
func NewHost(i Integration) (*Host, error)
func (h *Host) Name() string                    // "integration." + i.Name()
func (h *Host) Init(ctx context.Context) error  // delegates to Initializer, else nil
func (h *Host) Start(ctx context.Context) error // EventSource: Run; else block on ctx.Done()
func (h *Host) Stop(ctx context.Context) error  // io.Closer: Close; else nil
```

### Runtime wiring sketch

```go
// For each enabled integration in config:
tp, _ := i.(integrations.ToolProvider)
if tp != nil {
    for _, t := range tp.Tools() { registry.Register(t) }
}

sf, _ := i.(integrations.Surface)
if sf != nil {
    router.AddHandler(sf.ReplyHandler())
}

h, _ := integrations.NewHost(i)
supervisor.Register(h)
```

## Example

_Example integrations: `integrations/github`, `integrations/slack`_
