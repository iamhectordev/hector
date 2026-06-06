package agent

import (
	"context"

	islack "github.com/iamhectordev/hector/internal/slack"
	"github.com/iamhectordev/hector/modules/tui"
	"github.com/iamhectordev/hector/pkg/llm/schema"
	"github.com/iamhectordev/hector/pkg/session"
	"github.com/iamhectordev/hector/pkg/telem"
	"github.com/iamhectordev/hector/pkg/waffle"
)

// Module receives messages from surfaces and dispatches them to the runner.
type Module struct {
	bus        *waffle.EventBus
	runner     Runner
	sessions   session.Store
	baseSystem string
}

// Option configures a Module.
type Option func(*Module)

// WithBaseSystem sets the base system prompt text.
func WithBaseSystem(prompt string) Option {
	return func(m *Module) { m.baseSystem = prompt }
}

// WithSessionStore sets the store used to build per-turn agent contexts.
func WithSessionStore(store session.Store) Option {
	return func(m *Module) { m.sessions = store }
}

func NewModule(bus *waffle.EventBus, runner Runner, opts ...Option) *Module {
	m := &Module{
		bus:    bus,
		runner: runner,
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

func (m *Module) Name() string { return "agent" }

func (m *Module) Init(ctx context.Context) error {
	if err := waffle.On(m.bus, tui.MessageReceived).Handle("agent.tui", m.onTUIMessage); err != nil {
		m.log(ctx).ErrorContext(ctx, "failed to register tui listener", telem.Any("err", err))
		return err
	}
	if err := waffle.On(m.bus, islack.MessageReceived).Handle("agent.slack", m.onSlackMessage); err != nil {
		m.log(ctx).ErrorContext(ctx, "failed to register slack listener", telem.Any("err", err))
		return err
	}
	if err := waffle.On(m.bus, islack.MessageUpdated).Handle("agent.slack.update", m.onSlackMessageUpdated); err != nil {
		m.log(ctx).ErrorContext(ctx, "failed to register slack update listener", telem.Any("err", err))
		return err
	}
	m.log(ctx).InfoContext(ctx, "agent listeners registered")
	return nil
}

func (m *Module) Start(ctx context.Context) error {
	<-ctx.Done()
	m.log(ctx).InfoContext(ctx, "agent module stopping", telem.Any("cause", context.Cause(ctx)))
	return nil
}

func (m *Module) Stop(context.Context) error { return nil }

func (m *Module) newAgentContext(sourceURI string) (Context, error) {
	return NewSessionContext(m.sessions, sourceURI)
}

func (m *Module) handle(ctx context.Context, agentCtx Context, system string, messages []*schema.Message) error {
	history, _ := agentCtx.Messages(ctx)
	turnOffset := len(history)

	_, err := m.runner.Run(ctx, agentCtx, system, messages)
	if err != nil {
		return err
	}

	sess, _ := session.From(ctx)
	if recordErr := m.bus.Record(ctx, TurnEnd.New(TurnEndData{
		SourceURI:  sess.SourceURI,
		TurnOffset: turnOffset,
	})); recordErr != nil {
		m.log(ctx).WarnContext(ctx, "failed to record turn_end event", telem.Any("err", recordErr))
	}
	return nil
}

func (m *Module) log(ctx context.Context) telem.ContextLogger {
	return telem.Logger(ctx).With(
		telem.String("component", "module"),
		telem.String("module", "agent"),
	)
}
