# Architecture

## General

- Modular monolith — single deployable, internal module boundaries enforced by Go's `internal/` package
- Go for the runtime and backend
- DDD with ports and adapters — bounded contexts, domain events, repositories, anti-corruption layers for integrations
- Cross-context communication prefers events but not everything is required to be async
- DB: PG or SQLite behind a store interface abstraction — both supported

## Runtime

- Three primitives: events, effects, workflows
- Events are immutable, append-only, ordered — event log is the source of truth for workflow runs
- Workflows are DOT graph definitions, loaded at startup, versioned — runs are pinned to the version they started on
- Effects are internal — steps produce effects, effects complete and emit events
- Fan-out: one step emits multiple concurrent effects
- Fan-in: checks — validate that the correct jobs completed and passed, not a countdown
- Crash safety: effects persisted before any worker picks them up — timeout + retry on crash
- Sessions loaded from store, not replayed from events

## NFRs

- **Crash safety** — no work is ever lost. An effect must be persisted before a worker picks it up.
- **Auditability** — every action, tool call, approval, and policy evaluation is logged and traceable.
- **Modularity** — bounded contexts don't reach into each other's internals.
- **Tenant isolation** — no data, memory, or context crosses tenant boundaries under any circumstances.
- **Security** — credentials never appear in logs, events, or model context.
