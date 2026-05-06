# Architecture

## Structure

- Modular monolith — single binary, modules split into separate services via config when needed
- Go for the runtime and backend
- Each module exposes a `Module` interface (`Name`, `Start(ctx)`, `Stop(ctx)`) and is registered at startup
- Config controls which modules run and how — CLI flags, YAML, or env vars
- SQLite for local development, Postgres for production — same interface, swapped by config
- `services/` for bounded contexts, each with a compiler-enforced `internal/`
- Root `internal/` for shared infrastructure only: logger, config, db, tracing, events, CLI — no business logic

```
services/
  agent/
    module.go           ← implements Module
    agent.go            ← public API + ports (interfaces this service needs)
    internal/
      session.go
      session_store.go
      ...
  memory/
    module.go
    memory.go
    internal/
      entity.go
      entity_store.go
      ...
internal/
  server/               ← Module interface, lifecycle
  config/               ← env + yaml + flags
  logger/               ← slog setup
  tracing/              ← otel setup
  db/                   ← connection + migration runner
  events/               ← shared event types
  cli/                  ← urfave/cli v3 app and commands
cmd/
  hector/
    main.go             ← init context, logger, tracer → run modules
```

## DDD and interfaces

- Ports (interfaces) are defined by the consumer and live next to the consumer
- Implementations import the interface only for the compiler assertion: `var _ agent.LLMClient = (*Client)(nil)`
- Services never import each other's internals — cross-service communication through declared interfaces only
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
- **Modularity** — services never reach into each other's internals
- **Tenant isolation** — no data, memory, or context crosses tenant boundaries under any circumstances
- **Security** — credentials never appear in logs, events, or model context
