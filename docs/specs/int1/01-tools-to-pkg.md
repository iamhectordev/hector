# int1-01 — Move the tool contract to pkg/tools

Design: `docs/designs/001-integration-facets.md`
Golden paths: `flow/paths/tool.md`, `flow/paths/logging.md`, `flow/paths/tracing.md`

## Goal

Relocate the tool contract and its satellites from `modules/tools` to a new
`pkg/tools` package so that future code under `integrations/` can depend on tools
without importing `modules/`. Pure move + import updates. Zero behavior change.

## Changes

Create `pkg/tools` (package name `tools`) by moving these files from
`modules/tools`, with their tests:

- `definition.go` (`Tool`, `Definition`, `validateDefinition` if it lives there)
- `envelope.go` (`OK`, `Fail`, envelope types)
- `errors.go`
- `registry.go` (`Registry`, `NewRegistry`)
- `typed.go` (`New[I, O]`, `SchemaFor`)
- `timenow.go` (`NewTimeNow`)
- The telemetry helpers (span names, field builders) that the moved files use.
  Split `telem.go`/`tracing` helpers between the two packages so each package
  owns exactly the helpers its code calls.

`pkg/tools` MUST NOT import anything under `internal/` or `modules/`. Allowed:
stdlib, `pkg/llm/schema`, `pkg/telem`.

`modules/tools` keeps package name `tools` and retains:

- `module.go` (bus consumer `Module`) and `events.go`
- `mcp.go` (MCP adapter — imports `internal/mcp`, stays here for now)
- `mem_recall.go`

These files now import `pkg/tools` under an alias (suggested:
`pkgtools "github.com/iamhectordev/hector/pkg/tools"`) and reference the moved
identifiers through it (`pkgtools.Registry`, `pkgtools.OK`, …).

Update importers:

- `pkg/comms/reply.go` (and its telem/tests): `modules/tools` → `pkg/tools`.
- `modules/tools/web/*`: envelope/typed/Tool references → `pkg/tools`.
- `modules/tools/github/*`: `Tool`/`Registry` references → `pkg/tools`; the
  `MCPTool` adapter reference stays on `modules/tools`.
- `internal/app/runtime.go`: import both packages — `pkg/tools` for
  `NewRegistry`/`NewTimeNow`/`Tool`, `modules/tools` (aliased, e.g. `toolsmod`)
  for `NewModule`/`NewMemRecall`.

## Non-goals

- No renames of types, functions, or tool names.
- No changes to `mcp.go`, `mem_recall.go`, `module.go` beyond import fixes.
- Do not move `web/` or `github/` packages.
- Do not create `integrations/` — that is int1-02.

## Gate

`mise run test && mise run lint` — all existing tests pass unchanged (moved test
files keep their assertions byte-identical apart from package/import lines).
