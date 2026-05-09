package tui

import (
	"bufio"
	"context"
	"io"
	"log/slog"
	"os"

	"github.com/iamhectordev/hector/pkg/waffle"
)

// Module reads interactive input and publishes messages on the waffle bus.
type Module struct {
	bus    *waffle.EventBus
	in     io.Reader
	logger *slog.Logger
}

// NewModule wires the TUI to bus. If in is nil, stdin is used.
func NewModule(bus *waffle.EventBus, in io.Reader) *Module {
	if in == nil {
		in = os.Stdin
	}
	return &Module{
		bus:    bus,
		in:     in,
		logger: slog.Default().With("component", "module", "module", "tui"),
	}
}

func (m *Module) Name() string {
	return "tui"
}

func (m *Module) Init(context.Context) error { return nil }

// Start runs the input loop until ctx is cancelled.
func (m *Module) Start(ctx context.Context) error {
	m.log(ctx).InfoContext(ctx, "tui input loop starting")
	go m.inputLoop(ctx)
	<-ctx.Done()
	m.log(ctx).InfoContext(ctx, "tui module stopping", "cause", context.Cause(ctx))
	return nil
}

func (m *Module) Stop(context.Context) error {
	return nil
}

func (m *Module) inputLoop(ctx context.Context) {
	scanner := bufio.NewScanner(m.in)
	for scanner.Scan() {
		text := scanner.Text()
		if err := m.bus.Record(ctx, MessageReceived.New(MessageReceivedData{Text: text})); err != nil {
			m.log(ctx).ErrorContext(ctx, "failed to record tui message", "err", err)
			return
		}
	}
	if err := scanner.Err(); err != nil {
		m.log(ctx).ErrorContext(ctx, "tui input scanner failed", "err", err)
	}
}

func (m *Module) log(context.Context) *slog.Logger {
	return m.logger
}
