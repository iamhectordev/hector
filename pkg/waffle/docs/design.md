# Waffle Design

Waffle is a small event-driven runtime for Hector.

Its job is to record facts and let independent consumers react to them in a consistent way. It should stay simple enough to run in-process, but disciplined enough that durable storage and workflow execution do not change the mental model.

## Goals

### Events are facts

Events describe something that happened. They are named in past tense and do not tell consumers what to do.

Good:

- `MessageReceived`
- `ToolCallCompleted`
- `ApprovalGranted`

Avoid:

- `ProcessMessage`
- `SendReply`
- `UpdateMemory`

### Producers own schemas

The producer that emits an event owns its name, payload shape, schema version, and meaning at the time of emission.

Consumers may react to an event, but they do not get to redefine what the event means. The event schema is the contract between producer and consumer.

This is the most important boundary. If ownership gets blurry, event-driven systems become hard to change.

### Consumers own effects

Consumers decide what to do with an event.

The same event can have multiple consumers, and each consumer may interpret the event differently for its own purpose. Producers should not know who consumes their events.

### Recording comes before work

An event must be recorded before handlers act on it.

This keeps the fact separate from the work triggered by the fact. It also gives Waffle a stable place to add durability, retries, observability, and workflow state without changing how producers emit events.

### Delivery is not meaning

Dispatching a handler is runtime mechanics. It should not leak into the event schema.

Events should not contain fields just to satisfy a specific consumer or delivery mechanism. If a consumer needs state, it owns that state.

### Idempotency is expected

Consumers should be designed as if they may see the same event more than once.

Waffle should make idempotency easy to enforce, but event handlers should still be written with duplicate delivery in mind.

### Ordering is local

Waffle should not promise global ordering.

When ordering matters, it should be scoped to a clear boundary, such as a workflow run. Outside that boundary, consumers should tolerate relaxed ordering and converge on the correct state.

### Consistency lives inside boundaries

Strong consistency belongs inside a single aggregate or workflow run.

Across bounded contexts, Waffle should favor asynchronous updates and eventual consistency. That is the model, not a fallback.

### Schema evolution is normal

Event schemas will change.

Waffle should make versioning explicit and keep old event shapes understandable. Consumers should be able to handle older event versions when needed.

### Storage is a runtime detail

The event model should not depend on a specific backing store.

In-memory and durable storage should preserve the same core semantics: record facts, dispatch consumers, and keep producer/consumer ownership separate.

## Non-Goals

Waffle is not a general message broker.

It does not aim to provide global ordering, exactly-once delivery, distributed consensus, or cross-service transactionality.

Waffle should stay small enough to understand and embed.
