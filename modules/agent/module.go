package agent

import (
	"context"
	"log/slog"

	"github.com/iamhectordev/hector/modules/agent/internal/processor"
	"github.com/iamhectordev/hector/modules/tui"
	"github.com/iamhectordev/hector/pkg/waffle"
)

// Module receives messages from surfaces and dispatches them to the processor.
type Module struct {
	bus       *waffle.EventBus
	processor *processor.Processor
	logger    *slog.Logger
}

// NewModule registers waffle handlers on bus. If proc is nil, a stdout processor is used.
func NewModule(bus *waffle.EventBus, proc *processor.Processor) *Module {
	if proc == nil {
		proc = processor.New(nil)
	}
	return &Module{
		bus:       bus,
		processor: proc,
		logger:    slog.Default().With("module", "agent"),
	}
}

func (m *Module) Name() string {
	return "agent"
}

// Start registers listeners and blocks until ctx is cancelled.
func (m *Module) Start(ctx context.Context) error {
	if err := waffle.On(m.bus, tui.MessageReceived).Handle("agent.tui", m.onTUIMessage); err != nil {
		m.logger.ErrorContext(ctx, "failed to register tui listener", "err", err)
		return err
	}
	m.logger.InfoContext(ctx, "agent listeners registered")
	<-ctx.Done()
	m.logger.InfoContext(ctx, "agent module stopping", "cause", context.Cause(ctx))
	return nil
}

func (m *Module) Stop(context.Context) error {
	return nil
}
