package tui

import (
	"bufio"
	"context"
	"io"
	"os"

	"github.com/iamhectordev/hector/pkg/session"
	"github.com/iamhectordev/hector/pkg/telem"
	"github.com/iamhectordev/hector/pkg/waffle"
)

// Module reads interactive input and publishes messages on the waffle bus.
type Module struct {
	bus *waffle.EventBus
	in  io.Reader
}

// Option configures a Module.
type Option func(*Module)

// WithReader sets the input source. Defaults to stdin.
func WithReader(r io.Reader) Option {
	return func(m *Module) {
		if r != nil {
			m.in = r
		}
	}
}

func NewModule(bus *waffle.EventBus, opts ...Option) *Module {
	m := &Module{
		bus: bus,
		in:  os.Stdin,
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

func (m *Module) Name() string { return "tui" }

func (m *Module) Init(context.Context) error { return nil }

// Start runs the input loop until ctx is cancelled.
func (m *Module) Start(ctx context.Context) error {
	m.log(ctx).InfoContext(ctx, "tui input loop starting")
	go m.inputLoop(ctx)
	<-ctx.Done()
	m.log(ctx).InfoContext(ctx, "tui module stopping", telem.Any("cause", context.Cause(ctx)))
	return nil
}

func (m *Module) Stop(context.Context) error { return nil }

func (m *Module) inputLoop(ctx context.Context) {
	scanner := bufio.NewScanner(m.in)
	for scanner.Scan() {
		text := scanner.Text()
		eventCtx := session.With(ctx, session.Session{SourceURI: NewOriginURI()})
		if err := m.bus.Record(eventCtx, MessageReceived.New(MessageReceivedData{Text: text})); err != nil {
			m.log(ctx).ErrorContext(ctx, "failed to record tui message", telem.Any("err", err))
			return
		}
	}
	if err := scanner.Err(); err != nil {
		m.log(ctx).ErrorContext(ctx, "tui input scanner failed", telem.Any("err", err))
	}
}

func (m *Module) log(ctx context.Context) telem.ContextLogger {
	return telem.Logger(ctx).With(
		telem.String("component", "module"),
		telem.String("module", "tui"),
	)
}
