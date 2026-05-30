# Tracing

OpenTelemetry traces for understanding what Hector did, why it did it, and where time or failures happened.

## Principles
- Tracing is config-driven and can be disabled.
- Propagate trace context through `context.Context` and across durable events.
- Spans represent meaningful operations, not log lines.
- Do not record secrets or raw user/model content by default.

## What to trace
- Operations that do I/O.
- Operations that are important to product or runtime flow.
- Operations that make decisions we may need to inspect later.
- Errors, degraded behavior, retries, and fallback paths.

Avoid spans around small pure functions unless they explain meaningful behavior.

## What to tag
- Stable IDs, such as session, event, reaction, tool call, or provider request IDs.
- Operation context, such as module, handler, surface, provider, model, tool, or mode.
- Outcome fields, such as status, error type, error message, counts, or response codes.

Do not tag secrets, raw prompts, raw completions, large payloads, or arbitrary free-form text.

## Events
Use span events for notable moments inside an operation, such as retries, fallback choices, tool selection, errors, or degraded behavior.

Prompt, response, tool argument, and tool output capture must be explicit opt-in configuration.
