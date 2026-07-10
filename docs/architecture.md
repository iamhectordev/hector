# Architecture

## Structure

- Modular monolith — single binary, modules split into separate services via config when needed
- Go for the runtime and backend
- Each module exposes a `Module` interface (`Name`, `Start(ctx)`, `Stop(ctx)`) and is registered at startup
- Config controls which modules run and how — CLI flags, YAML, or env vars
- SQLite for local development, Postgres for production — same interface, swapped by config
- Root `internal/` for app wiring and shared infrastructure only — no business logic
- `modules/` for core supervised runtime units (agent, tools executor, memory, email, TUI)
- `pkg/` for the public library surface (tools contract, supervisor, waffle, comms, LLM, telem, …)
- `integrations/` for facet-based capability units — one package per external vendor (GitHub, Slack, …)

```
cmd/
  hector/
    main.go             ← init context, logger, tracer → run modules
internal/
  app/                  ← runtime wiring: builds []Module from config, wires deps
  cli/                  ← urfave/cli v3 app and commands (chat, serve, events)
  db/                   ← SQLite connection + migration runner
  tracing/              ← otel setup
  email/                ← shared email infrastructure
  web/                  ← HTTP helpers
  embed/                ← embedded assets
  ulid/                 ← ID generation
modules/
  agent/                ← supervisor.Module: LLM loop, message handling
  tools/                ← supervisor.Module: tool execution via bus consumer, mem_recall
  memory/               ← supervisor.Module: conversation memory
  email/                ← supervisor.Module: email sending
  tui/                  ← supervisor.Module: terminal UI
pkg/
  tools/                ← tool contract: Tool, Definition, Registry, SchemaFor, typed helper, MCPTool
  supervisor/           ← Module interface, lifecycle manager
  comms/                ← reply routing: ReplyHandler, ReplyRouter
  llm/                  ← LLM client abstraction
  waffle/               ← event bus and workflow primitives
  telem/                ← telemetry and tracing helpers
  session/              ← session types
  memory/               ← memory store contract
  mcp/                  ← MCP client: wraps the official MCP SDK
  migrations/           ← database migration files
  safehttp/             ← safe HTTP utilities
integrations/
  integration.go        ← facet contracts: Integration, ToolProvider, EventSource, Surface, Initializer
  host.go               ← generic Host adapter: turns any Integration into a supervisor.Module
  github/               ← GitHub integration (tool provider)
  slack/                ← Slack integration (event source, surface)
```

### Modules and integrations

A **module** is a runtime unit supervised by `pkg/supervisor`. Core modules are hand-written and live under `modules/`. An **integration** is a capability unit exposing orthogonal facets — tools, event source, surface — and lives under `integrations/`. The generic `Host` adapter (`integrations/host.go`) adapts an integration into a supervisor module, bridging the two layers.

Grouping modules and integrations into processes is a deployment decision. The codebase is structured as a modular monolith today, but the boundary preserves the path to split services when needed.

### Import discipline

- Integrations import only `pkg/...` (and stdlib / vendor SDKs). They never import `internal/`, `modules/`, or each other.
- Modules never import integrations — except where explicitly flagged as debt: the agent module imports `integrations/slack` for Slack-shaped event handling (see design doc "Known deferred issues").

## DDD and interfaces

- Ports (interfaces) are defined by the consumer and live next to the consumer
- Implementations import the interface only for the compiler assertion: `var _ agent.LLMClient = (*Client)(nil)`
- Modules never import each other's internals — cross-module communication through declared interfaces only
- In-process: interface implemented directly. Split service: same interface, gRPC client behind it
- `main` wires everything — injects implementations into consumers at startup

## Code organisation

- Files named after what they contain, not the pattern — no `domain.go`, no `service.go`
- Aggregates named after the concept: `session.go`, `entity.go`, `episode.go`
- Logic named after what it does: `extractor.go`, `matcher.go`, `compactor.go`, `linker.go`
- `store.go` and `events.go` are consistent across packages
- Start flat within a package, split into subdirectories only when it gets unwieldy

## Runtime

- Three primitives: events, effects, workflows
- Events are immutable, append-only, ordered — the event log is the source of truth for a workflow run
- Workflows are DOT graphs, versioned, loaded at startup — runs pinned to the version they started on
- Effects are internal — steps produce effects, effects complete and emit new events
- Fan-out: one step produces multiple concurrent effects
- Fan-in: checks — validate that the correct jobs completed and passed
- Crash safety: effects persisted before any worker picks them up — timeout and retry on failure
- Sessions loaded from store, not replayed from events

## Configuration — 12-factor

- Config from environment variables, YAML, and CLI flags — in that order of precedence
- All backing services (DB, LLM, Slack) are attached resources — injectable and swappable
- Stateless processes — state lives in the DB, never in-process between requests
- Logs as streams — slog to stdout, nothing else
- Fast startup and graceful shutdown on every module

## NFRs

- **Crash safety** — no work is ever lost. An effect must be persisted before a worker picks it up
- **Auditability** — every action, tool call, approval, and policy evaluation is logged and traceable
- **Modularity** — modules never reach into each other's internals
- **Tenant isolation** — no data, memory, or context crosses tenant boundaries under any circumstances
- **Security** — credentials never appear in logs, events, or model context
