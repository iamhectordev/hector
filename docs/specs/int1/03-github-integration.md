# int1-03 — GitHub as an integration

Design: `docs/designs/001-integration-facets.md`
Golden paths: `flow/paths/integration.md` (new version), `flow/paths/tool.md`,
`flow/paths/config.md`
Depends: int1-02

## Goal

Fold `internal/github` + `modules/tools/github` into `integrations/github`
implementing the facet contracts, and introduce the generic integration wiring
loop in the runtime. Behavior preserved: same tools registered under the same
names, same MCP client lifecycle.

## Changes

### `integrations/github` (package `github`)

Move the contents of `internal/github` (config, token provider, client) and
`modules/tools/github` (tools, MCP wiring) into one flat package. Then:

```go
// Integration implements integrations.Integration, integrations.ToolProvider, io.Closer.
func New(ctx context.Context, cfg Config) (*Integration, error)
```

- `New` does what `Register` + client construction do today (validate config,
  token provider, REST client, build tools, MCP client) — but performs **no
  registry writes**. It returns the integration holding the built tool list and
  the MCP closer.
- `Name() string` → `"github"`.
- `Tools() []tools.Tool` returns the already-built list.
- `Close() error` closes the MCP client (today's returned `io.Closer`).
- Keep `Config` exactly as-is (`Enabled`, `required_if=Enabled true` validation,
  env tags). `internal/app/config.go` updates its import to
  `integrations/github`.

The MCP adapter (`MCPTool`) currently lives in `modules/tools` and imports
`internal/mcp`; `integrations/github` may NOT import `internal/` or `modules/`.
Move `internal/mcp` to `pkg/mcp` (mechanical move + import updates — it is a
self-contained client) and move the `MCPTool` adapter (`mcp.go` + its telem
helpers + tests) from `modules/tools` into `pkg/tools` where it now only imports
`pkg/mcp`. Update remaining importers.

### Runtime wiring (`internal/app/runtime.go`)

Replace the `r.cfg.GitHub.Enabled { githubtools.Register(...) }` block and the
`githubCloser` field with the generic loop:

```go
func (r *Runtime) initIntegrations(ctx context.Context) ([]integrations.Integration, error)
// builds the enabled integrations from config (this slice: github only)
```

For each integration: if `ToolProvider`, register its tools into the registry at
wiring time; wrap in `integrations.NewHost` and append to supervisor modules.
Delete `r.githubCloser` and its handling in `close()` — teardown now flows
through `Host.Stop`.

Note the ordering constraint: the wiring loop must run where the current
`Register` call runs (tools must be in the registry before the supervisor
starts). Surfaces/reply handlers are NOT part of this slice's loop (Slack lands
in int1-04; the loop only needs ToolProvider + Host here).

## Non-goals

- No changes to tool names, schemas, or behavior.
- No Slack changes.
- No new tools.

## Gate

`mise run test && mise run lint`. Existing github tool tests move with the
package and pass unchanged. Add one runtime-level test (extend the existing
runtime tests in `internal/app`) proving that with `GitHub.Enabled=true` config
the github tools appear in the registry, and with `Enabled=false` they do not.
