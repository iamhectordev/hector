package agent

import (
	"context"

	islack "github.com/iamhectordev/hector/internal/slack"
	"github.com/iamhectordev/hector/modules/tui"
	"github.com/iamhectordev/hector/pkg/session"
	"github.com/iamhectordev/hector/pkg/telem"
	"github.com/iamhectordev/hector/pkg/waffle"
)

// Module receives messages from surfaces and dispatches them to the runner.
type Module struct {
	bus        *waffle.EventBus
	sessions   session.Store
	perceiver  Perceiver
	baseSystem string
	cfg        Config
	processor  *Processor
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

// WithPerceiver sets the pre-turn assessor.
func WithPerceiver(perceiver Perceiver) Option {
	return func(m *Module) { m.perceiver = perceiver }
}

// WithConfig sets agent module config.
func WithConfig(cfg Config) Option {
	return func(m *Module) { m.cfg = cfg }
}

func NewModule(bus *waffle.EventBus, runner Runner, opts ...Option) *Module {
	m := &Module{
		bus: bus,
	}
	for _, opt := range opts {
		opt(m)
	}
	m.processor = NewProcessor(bus, runner, m.sessions, m.perceiver, m.cfg)
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

func (m *Module) log(ctx context.Context) telem.ContextLogger {
	return telem.Logger(ctx).With(
		telem.String("component", "module"),
		telem.String("module", "agent"),
	)
}
