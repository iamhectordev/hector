# Waffle

Waffle is a tiny in-memory event runtime for typed Go events.

Producers define event names, schema versions, and payload structs. Consumers register typed handlers and decide what an event means for them.

```go
messageReceived, err := waffle.Define[MessageReceivedData]("tui.message_received", 1)
if err != nil {
	return err
}

type MessageReceivedData struct {
	MessageID string `json:"message_id"`
	Text      string `json:"text"`
}

bus, err := waffle.NewEventBus(waffle.WithWorkers(4))
if err != nil {
	return err
}

err = waffle.On(bus, messageReceived).Handle("agent.reply", func(ctx context.Context, event waffle.Event[MessageReceivedData]) error {
	return nil
})
if err != nil {
	return err
}

err = bus.Record(ctx, messageReceived.New(MessageReceivedData{
	MessageID: "msg_123",
	Text:      "hello",
}))
```

This first version is intentionally small. It keeps events in memory, runs handlers in a worker pool, and exposes `Drain` for tests and graceful shutdown.

It does not provide persistence, retries, idempotency, or ordering guarantees yet.
