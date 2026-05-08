# Listener / processor split

Decouples surface-specific event shapes from surface-agnostic processing.

## Principles
- `listeners.go` knows external event shapes, adapts to internal interface
- `internal/processor.go` knows nothing about concrete input surfaces
- One listener function per external event type

## Outline
```go
// listeners.go
func (m *Module) onTUIMessage(ctx context.Context, e waffle.Event[tui.MessageReceivedData]) error {
    return m.processor.Handle(ctx, e.Data().Text)
}

// internal/processor.go
func (p *Processor) Handle(ctx context.Context, text string) error { ... }

// module.go — registered in Start()
waffle.On(m.bus, tui.MessageReceived).Handle("agent.tui", m.onTUIMessage)
```
