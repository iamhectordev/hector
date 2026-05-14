package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/iamhectordev/hector/pkg/waffle"
)

// Module executes registered tools in response to tool call events.
type Module struct {
	bus    *waffle.EventBus
	tools  map[string]Tool
	logger *slog.Logger
}

func NewModule(bus *waffle.EventBus, tools ...Tool) (*Module, error) {
	m := &Module{
		bus:    bus,
		tools:  make(map[string]Tool, len(tools)),
		logger: slog.Default().With("component", "module", "module", "tools"),
	}
	for _, tool := range tools {
		if err := m.Register(tool); err != nil {
			return nil, err
		}
	}
	return m, nil
}

func (m *Module) Name() string { return "tools" }

func (m *Module) Init(ctx context.Context) error {
	if err := waffle.On(m.bus, CallRequested).Handle("tools.call_requested", m.onCallRequested); err != nil {
		m.log(ctx).ErrorContext(ctx, "failed to register tool call listener", "err", err)
		return err
	}
	m.log(ctx).InfoContext(ctx, "tools listener registered")
	return nil
}

func (m *Module) Start(ctx context.Context) error {
	<-ctx.Done()
	m.log(ctx).InfoContext(ctx, "tools module stopping", "cause", context.Cause(ctx))
	return nil
}

func (m *Module) Stop(context.Context) error { return nil }

func (m *Module) Register(tool Tool) error {
	if tool == nil {
		return nil
	}
	def := tool.Definition()

	if def.Name == "" {
		return fmt.Errorf("tools: cannot register tool with empty name")
	}
	if def.Description == "" {
		return fmt.Errorf("tools: tool %q has empty description", def.Name)
	}
	if def.Parameters == nil {
		return fmt.Errorf("tools: tool %q has nil parameters", def.Name)
	}

	if _, exists := m.tools[def.Name]; exists {
		return fmt.Errorf("tools: duplicate tool name %q", def.Name)
	}

	m.tools[def.Name] = tool
	return nil
}

func (m *Module) onCallRequested(ctx context.Context, e waffle.Event[CallRequestedData]) error {
	d := e.Data()

	tool, ok := m.tools[d.Name]
	if !ok {
		return m.complete(ctx, d.CallID, "", fmt.Sprintf("unknown tool %q", d.Name))
	}

	output, err := tool.Run(ctx, json.RawMessage(d.Args))
	if err != nil {
		return m.complete(ctx, d.CallID, "", err.Error())
	}
	return m.complete(ctx, d.CallID, output, "")
}

func (m *Module) complete(ctx context.Context, callID, output, errorText string) error {
	return m.bus.Record(ctx, CallCompleted.New(CallCompletedData{
		CallID: callID,
		Output: output,
		Error:  errorText,
	}))
}

func (m *Module) log(context.Context) *slog.Logger { return m.logger }
