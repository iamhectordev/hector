# Waffle Roadmap

This roadmap is organized around what a Waffle user can do with the library. It intentionally avoids implementation-only work unless it creates a visible capability.

## Already Useful

### Define typed facts

Status: done

As a producer, I can define facts with stable names, schema versions, and typed payloads, so consumers have a clear contract.

### Record facts

Status: done in memory

As a producer, I can record that something happened without knowing who will react.

### React to facts

Status: done in memory

As a consumer, I can register typed handlers and react independently from other consumers.

## Make It Durable

### Use an app-owned SQLite database

Status: done

As an app owner, I can give Waffle the SQLite database my app already owns, so Waffle can share storage cleanly with the rest of the process.

The app owns opening, configuring, pooling, and closing the database. Waffle owns its own tables and migrations inside that database (`sqlite.NewStore`, `sqlite.Migrations()` with `pkg/migrations`).

### Test with disposable SQLite databases

Status: partly done

As a developer, I can test Waffle-backed code with either an in-memory database or a temporary file database that is cleaned up automatically.

The SQLite package tests use a temp-file database under `t.TempDir()`. There is not yet a small exported helper for apps to copy-paste-free testing.

### Write the event trail

Status: done

As an app owner, I can record facts durably, so the facts still exist after the process restarts.

Use a `sqlite.Store` with `EventBus` and run migrations. Listing and lookup use the same store.

## Make It Inspectable

### Review the event trail

Status: done

As an app owner, I can see which facts happened, when they happened, and the metadata Waffle recorded for them.

`waffle.Reader` lists events newest-first with limit and optional cursor (`Before`). Implemented for the in-memory store and `sqlite.Store`.

### Inspect one event

Status: done

As an operator or developer, I can look up a specific event and understand what happened.

`waffle.Reader.Get` and store `Get` load one record by id (SQLite and memory).

## Make It Safe

### Run safely when handlers fail

Status: partly done

As an app owner, handler errors or panics do not crash my process.

We now have a reusable process supervisor in `pkg/supervisor` that captures panics and errors, reports the stop reason, and shuts modules down gracefully. Waffle still needs to expose the right runtime boundaries so this protection applies cleanly to Waffle handler execution paths.

### Choose what happens on handler failure

Status: not done

As an app owner, I can plug in error handling and decide how handler failures are reported or handled.

## Make It Controllable

### Start the runtime intentionally

Status: partly done

As an app owner, I can register handlers and start processing when the app is ready.

The supervisor primitives are in place (`Start(ctx)` / `Stop(ctx)` style lifecycle, context cancellation with cause, optional signal handling), but Waffle runtime start is not yet wired as a first-class integration path.

### Stop accepting new work

Status: partly done

As an app owner, I can stop new records during shutdown.

### Wait for in-flight work

Status: done in memory

As an app owner, I can wait for current work to finish before the process exits.

### Surface shutdown reason clearly

Status: partly done

As an app owner, I can tell whether the runtime stopped because of a signal, a panic, an error, or context cancellation.

The supervisor report already carries this reason model. Waffle still needs to expose/report those reasons in its own runtime-facing API.

## Make It Resilient

### Resume undelivered work

Status: not done

As an app owner, if the process crashes after recording facts, Waffle can continue delivery after restart.

### Avoid duplicate side effects

Status: not done

As a consumer, I can make handler execution idempotent using Waffle's event and handler identity.

### Retry transient failures

Status: not done

As an app owner, temporary failures can be retried with limits and visibility.

### Set failed work aside

Status: not done

As an app owner, permanently failing work can be moved out of the hot path and inspected later.

## Make It Operable

### Observe runtime behavior

Status: not done

As an operator, I can see records, handler runs, failures, latency, and retries in OpenTelemetry.

## Make It A Workflow Runtime

### Run workflows from facts

Status: not done

As a workflow author, I can start and advance a workflow from recorded events.

### Track external work

Status: not done

As a workflow author, I can request effects, record completion, and continue consistently.
