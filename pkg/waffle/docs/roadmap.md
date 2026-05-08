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

## Make It Inspectable

### Keep an event trail

Status: not done

As an app owner, I can see which facts happened, when they happened, and the metadata Waffle recorded for them.

### Inspect one event

Status: not done

As an operator or developer, I can look up a specific event and understand what happened.

## Make It Safe

### Run safely when handlers fail

Status: not done

As an app owner, handler errors or panics do not crash my process.

### Choose what happens on handler failure

Status: not done

As an app owner, I can plug in error handling and decide how handler failures are reported or handled.

## Make It Controllable

### Start the runtime intentionally

Status: not done

As an app owner, I can register handlers and start processing when the app is ready.

### Stop accepting new work

Status: partly done

As an app owner, I can stop new records during shutdown.

### Wait for in-flight work

Status: done in memory

As an app owner, I can wait for current work to finish before the process exits.

## Make It Durable

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
