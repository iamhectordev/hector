# Waffle event

Typed events on the in-process bus via `pkg/waffle`.

## Principles
- Definitions are producer-owned, live in the producer's public file
- Handlers are consumer-owned, registered in `Start()`
- `CausedBy(parent)` propagates baggage through causal chains (correlation_id, causation_id)

## Outline
```go
// producer — e.g. modules/tui/tui.go
var MessageReceived = waffle.Define[MessageReceivedData]("tui.message_received", 1)
type MessageReceivedData struct { ... }

bus.Record(ctx, MessageReceived.New(data))
bus.Record(ctx, MessageReceived.New(data), waffle.CausedBy(parent))

// consumer — registered in Start()
waffle.On(bus, tui.MessageReceived).
    Handle("agent.tui", func(ctx context.Context, e waffle.Event[tui.MessageReceivedData]) error {
        ...
    })
```
