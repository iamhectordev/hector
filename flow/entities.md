# Flow Tree Entities

The flow tree tracks product intent. Keep entries short, concrete, and separate by kind.

## Domain

A root product area that groups related features.

Example: `web`

## Feature

A user-facing capability inside a domain.

Example: `web.search`

## Story

A short user-visible capability or outcome. Stories say what Hector can do or what a user/admin can configure. They are not implementation choices, internal architecture, sequencing notes, or deferred work.

Good:
- Hector searches the web for current information.
- Admins configure the web search provider.

Not stories:
- V1 uses one provider.
- Implement a provider interface.
- Multi-provider ranking is deferred.

Do not use the full "As a..." format unless it makes the actor or benefit clearer.

## NFR

A non-functional requirement: a quality or constraint the feature must satisfy. Use NFRs for security, privacy, reliability, latency, observability, auditability, and similar properties.

Examples:
- Provider credentials never appear in logs, events, tool output, or model context.
- Tool output uses a provider-neutral result shape.

## Open Question

A decision needed before the feature can be considered ready.

Example: Which provider should be implemented first?
