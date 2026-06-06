# Tracing

OpenTelemetry traces for understanding what Hector did, why it did it, and where time or failures happened.

## Principles
- Tracing is config-driven and can be disabled.
- Propagate trace context through `context.Context` and across durable events.
- Use `pkg/telem` at call sites; do not call raw OpenTelemetry APIs directly unless the package is implementing telemetry plumbing.
- Domains own their span names and field helpers in local `telem.go` files.
- Spans represent meaningful operations, not log lines.
- Do not record secrets or raw user/model content by default.

## Naming
- Span names are lowercase dot-separated operations: `slack.message.receive`, `waffle.event.record`, `agent.turn.run`.
- Attribute names are lowercase dot-separated fields: `waffle.event.id`, `tool.call_id`, `llm.model`.
- Keep names low-cardinality. Put IDs and operation context in attributes, never in span names.
- Use domain-owned constants/helpers for repeated names and fields.

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
- Propagated context such as `session.id` belongs in baggage when it should flow through async/durable boundaries and appear in logs.

Do not tag secrets, raw prompts, raw completions, large payloads, or arbitrary free-form text.

## Events
Use span events for notable moments inside an operation, such as retries, fallback choices, tool selection, errors, or degraded behavior.

Prompt, response, tool argument, and tool output capture must be explicit opt-in configuration.

## Outline
```go
ctx = telem.WithBaggage(ctx, sessionFields...)

ctx, span := telem.Trace(ctx, spanMessageReceive, messageFields(e)...)
defer span.End(&err)

telem.Event(ctx, "retry", telem.Int("attempt", attempt))
```
