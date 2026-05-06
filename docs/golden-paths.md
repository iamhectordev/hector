# Golden Paths

Each golden path is a canonical pattern that must exist before it is used in a slice.
If a pattern you need is missing, design it with the user before building.

## CLI command

Adding a subcommand to the Hector CLI. Uses klee (wraps urfave/cli v3).

- Commands live in `internal/cli/`, registered in `Commands()` in `commands.go`
- Split into a constructor (`fooCommand() *cli.Command`) and an action (`fooAction(ctx, cmd) error`)
- Flags declared in the constructor's `Flags` field
- Access config via `klee.Config[T](ctx)`, global flags via `klee.GetRunFlags(ctx)`
- Return errors bare — error kind conventions to be defined
- Example: `internal/cli/chat.go`

## Config field

Adding a typed config field loaded from env vars, YAML, and CLI flags.
- Config struct lives in `internal/config/`
- Precedence: env vars → YAML → CLI flags
- All backing services configured as attached resources (DSN, API keys, URLs)
- Example: _not yet defined_

## Module

Defining a service module that can be started and stopped by the server.
- Implements `Module` interface from `internal/server/`
- Registered at startup based on config
- Example: _not yet defined_

## Logging

Emitting a structured log line.
- Use `slog` from `internal/logger/`
- Always log with context: `slog.InfoContext(ctx, ...)`
- Fields: snake_case keys, no sensitive values
- Example: _not yet defined_

## DB migration

Adding a database table or schema change.
- Migration files live in `internal/db/migrations/`
- Sequential numbered files: `0001_create_sessions.sql`
- Runner applies pending migrations at startup
- Example: _not yet defined_

## Store

Implementing a repository for an aggregate.
- One `store.go` per aggregate, in the same package
- Interface defined in the service's public file (port owned by the consumer)
- Two implementations: real (pg/sqlite) and in-memory for tests
- Example: _not yet defined_

## Domain event

Defining and dispatching a domain event.
- Shared event types in `internal/events/`
- Events are immutable structs: past-tense names, all fields set at construction
- Example: _not yet defined_

## Blackbox test

Writing a test that validates a slice from the outside.
- Tests live in `tests/scenarios/`
- Drive the system through its public interface (CLI, HTTP, etc.) — no internal imports
- Assert on observable output only
- Example: _not yet defined_
