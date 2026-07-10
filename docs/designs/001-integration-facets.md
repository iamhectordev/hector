# 001 — Integration facets

Status: accepted · 2026-07-10

## Problem

Integrations (Slack, GitHub, and everything coming: Jira, Confluence, …) are wired
ad hoc. Slack is a hand-written `supervisor.Module`; GitHub is a `Register` function
whose `io.Closer` lives as a loose field on `Runtime`. Each new integration invents
its own wiring, and there is no shared shape for "connects Hector to an external
service".

## Design

Two layers, mapped by a generic adapter:

- **Module** (existing, unchanged) — the runtime/deployment unit supervised by
  `pkg/supervisor`. Core modules stay hand-written: agent, tools executor, memory,
  surfaces like TUI.
- **Integration** (new) — a capability unit: one package per vendor, contributing
  some subset of orthogonal *facets*. Not tiers — checkboxes.

### Contracts (`integrations/integration.go`)

```go
type Integration interface{ Name() string }

// Optional facets, discovered by type assertion:
type ToolProvider interface{ Tools() []tools.Tool }            // outbound calls
type EventSource  interface{ Run(ctx context.Context) error }  // inbound: blocks; socket loop, poller — its business
type Surface      interface{ ReplyHandler() comms.ReplyHandler }
type Initializer  interface{ Init(ctx context.Context) error } // auth verify, client build
// io.Closer — long-lived connections, closed at Stop
```

Whether events arrive by webhook, socket, or polling is the integration's private
concern; downstream of the bus event nothing can tell the difference.

### Host adapter (`integrations/host.go`)

Written once; turns any integration into a `supervisor.Module`:

```go
func NewHost(i Integration) (*Host, error)
// Name  → "integration." + i.Name()
// Init  → i.Init if Initializer
// Start → i.Run if EventSource, else block on ctx.Done()
// Stop  → i.Close if io.Closer
```

### Wiring (runtime)

The runtime builds `[]Integration` from config (each gated by explicit
`Enabled`), then for each:

- `ToolProvider` → `registry.Register(tp.Tools()...)` at wiring time (registration
  stays wiring-time per `flow/paths/tool.md`; failures surface at startup).
- `Surface` → reply handler into `comms.NewReplyRouter`.
- `NewHost(i)` → supervisor.

Integrations receive dependencies (bus, config) by constructor injection, same as
everything else. No host-context struct until the HTTP gateway / route registrar
exists.

### Layout and import discipline

- Contracts and implementations live under root `integrations/`, one package per
  vendor: `integrations/github`, `integrations/slack`.
- Integrations import only `pkg/...` (and stdlib/vendor SDKs) — never root
  `internal/`, never `modules/`, never each other. To make that possible the tool
  contract (`Tool`, `Definition`, envelope, `SchemaFor`, typed helper, `Registry`)
  moves from `modules/tools` to `pkg/tools` first. `modules/tools` keeps the
  module-shaped remainder: the bus-consumer `Module`, its events, the MCP adapter,
  `mem_recall`.
- Vendor-specific code in root `internal/` (`internal/slack`, `internal/github`)
  folds into its integration package.

### Profiles

`chat` = TUI, `serve` = daemon; never mixed. `serve` stops hard-requiring Slack:
integrations activate purely on `Enabled` config. `serve` with zero enabled
integrations is a startup error ("serve requires at least one enabled
integration") — fail fast; relax later if headless triggers become real.

## Known deferred issues

- **Agent↔Slack coupling**: the agent module listens to Slack-shaped events and
  builds its message parts from them. The package move relocates this leak
  (agent imports `integrations/slack`), it does not fix it. A normalized inbound
  message contract is a separate design.
- **HTTP gateway** (webhook ingress, route registrar facet): deferred until the
  first webhook-mode integration needs it. Pull/stream sources need nothing but
  the bus.
- **Adaptation layer** (tool filtering, pinned args, per-integration guidance for
  the agent; shared with MCP servers): separate design, applies at the registry
  as a decorator, source-agnostic.
- Email and TUI stay core modules for now.

## Migration plan (specs in docs/specs/int1/)

1. `01-tools-to-pkg.md` — move the tool contract to `pkg/tools`.
2. `02-integration-contracts.md` — contracts + host + rewrite of
   `flow/paths/integration.md`.
3. `03-github-integration.md` — GitHub becomes `integrations/github`.
4. `04-slack-integration.md` — Slack becomes `integrations/slack`; Enabled gate;
   serve zero-integration error.
5. `05-architecture-docs.md` — `docs/architecture.md` matches reality.

Slices 3 and 4 are independent after 2. Every slice gates on
`mise run test && mise run lint` with behavior preserved (except the explicit
profile change in slice 4).
