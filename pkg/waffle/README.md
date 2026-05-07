# Waffle

Waffle is a tiny in-memory event runtime for typed Go events.

Producers define event names, schema versions, and payload structs. Consumers register typed handlers and decide what an event means for them.

```go
var SlackMessageReceived = waffle.Define[SlackMessageReceivedData](
	"gateway.slack_message_received",
	1,
)

type SlackMessageReceivedData struct {
	MessageID string `json:"message_id"`
	Text      string `json:"text"`
}

bus := waffle.NewEventBus(waffle.WithWorkers(4))

waffle.On(bus, SlackMessageReceived).Handle("agent.reply", func(ctx context.Context, event waffle.Event[SlackMessageReceivedData]) error {
	return nil
})

err := bus.Record(ctx, SlackMessageReceived.New(SlackMessageReceivedData{
	MessageID: "msg_123",
	Text:      "hello",
}))
```

This first version is intentionally small. It keeps events in memory, runs handlers in a worker pool, and exposes `Drain` for tests and graceful shutdown.

It does not provide persistence, retries, idempotency, or ordering guarantees yet.
