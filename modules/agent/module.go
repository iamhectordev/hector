package agent

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/iamhectordev/hector/modules/slack"
	"github.com/iamhectordev/hector/modules/tui"
	"github.com/iamhectordev/hector/pkg/llm/message"
	"github.com/iamhectordev/hector/pkg/waffle"
)

// Completer produces an assistant reply from a message history.
// Implemented by LLM provider clients in pkg/llm/providers/.
type Completer interface {
	Complete(ctx context.Context, messages []*message.Message) (*message.Message, error)
}

// Module receives messages from surfaces and dispatches them to the completer.
type Module struct {
	bus       *waffle.EventBus
	completer Completer
	out       io.Writer
	logger    *slog.Logger
}

// Option configures a Module.
type Option func(*Module)

// WithWriter sets where replies are printed. Defaults to stdout.
func WithWriter(w io.Writer) Option {
	return func(m *Module) {
		if w != nil {
			m.out = w
		}
	}
}

func NewModule(bus *waffle.EventBus, completer Completer, opts ...Option) *Module {
	m := &Module{
		bus:       bus,
		completer: completer,
		out:       os.Stdout,
		logger:    slog.Default().With("component", "module", "module", "agent"),
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

func (m *Module) Name() string { return "agent" }

func (m *Module) Init(ctx context.Context) error {
	if err := waffle.On(m.bus, tui.MessageReceived).Handle("agent.tui", m.onTUIMessage); err != nil {
		m.log(ctx).ErrorContext(ctx, "failed to register tui listener", "err", err)
		return err
	}
	if err := waffle.On(m.bus, slack.MessageReceived).Handle("agent.slack", m.onSlackMessage); err != nil {
		m.log(ctx).ErrorContext(ctx, "failed to register slack listener", "err", err)
		return err
	}
	m.log(ctx).InfoContext(ctx, "agent listeners registered")
	return nil
}

func (m *Module) Start(ctx context.Context) error {
	<-ctx.Done()
	m.log(ctx).InfoContext(ctx, "agent module stopping", "cause", context.Cause(ctx))
	return nil
}

func (m *Module) Stop(context.Context) error { return nil }

func (m *Module) handle(ctx context.Context, text string) error {
	reply, err := m.completer.Complete(ctx, []*message.Message{message.UserMessage(text)})
	if err != nil {
		return err
	}
	if reply == nil {
		return fmt.Errorf("llm: nil reply")
	}
	_, err = fmt.Fprintln(m.out, reply.Content)
	return err
}

func (m *Module) log(context.Context) *slog.Logger { return m.logger }
