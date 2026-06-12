package agent

import (
	"context"
	"fmt"

	"github.com/iamhectordev/hector/pkg/llm/schema"
	"github.com/iamhectordev/hector/pkg/session"
	"github.com/iamhectordev/hector/pkg/telem"
	"github.com/iamhectordev/hector/pkg/waffle"
)

type Processor struct {
	bus       *waffle.EventBus
	runner    Runner
	sessions  session.Store
	perceiver Perceiver
	cfg       Config
}

func NewProcessor(bus *waffle.EventBus, runner Runner, sessions session.Store, perceiver Perceiver, cfg Config) *Processor {
	return &Processor{
		bus:       bus,
		runner:    runner,
		sessions:  sessions,
		perceiver: perceiver,
		cfg:       cfg,
	}
}

func (p *Processor) Handle(ctx context.Context, sourceURI, system string, incoming []*schema.Message) error {
	agentCtx, err := NewSessionContext(p.sessions, sourceURI)
	if err != nil {
		return err
	}

	if err := p.assess(ctx, agentCtx, incoming); err != nil {
		if err == errIgnoredByPerception {
			return nil
		}
		return err
	}

	history, _ := agentCtx.Messages(ctx)
	turnOffset := len(history)

	_, err = p.runner.Run(ctx, agentCtx, system, incoming)
	if err != nil {
		return err
	}

	sess, _ := session.From(ctx)
	var sessionID string
	if p.sessions != nil {
		if stored, err := p.sessions.GetOrCreate(ctx, sess.SourceURI); err == nil {
			sessionID = stored.ID
		}
	}
	if recordErr := p.bus.Record(ctx, TurnEnd.New(TurnEndData{
		SessionID:  sessionID,
		SourceURI:  sess.SourceURI,
		TurnOffset: turnOffset,
	})); recordErr != nil {
		p.log(ctx).WarnContext(ctx, "failed to record turn_end event", telem.Any("err", recordErr))
	}
	return nil
}

func (p *Processor) assess(ctx context.Context, agentCtx Context, incoming []*schema.Message) error {
	if !p.cfg.Perception.Enabled || p.perceiver == nil {
		return nil
	}

	history, err := agentCtx.Messages(ctx)
	if err != nil {
		return fmt.Errorf("agent: load history for perception: %w", err)
	}

	var spanErr error
	ctx, span := telem.Trace(ctx, spanPerceptionAssess, perceptionFields(history, incoming)...)
	defer span.End(&spanErr)

	result, err := p.perceiver.Assess(ctx, history, incoming)
	if err != nil {
		spanErr = err
		return fmt.Errorf("agent: perception failed: %w", err)
	}
	span.AddFields(perceptionResultFields(result)...)

	switch result.Action {
	case PerceptionActionIgnore:
		p.log(ctx).DebugContext(ctx, "agent message ignored by perception", telem.String("agent.perception.reason", result.Reason))
		return errIgnoredByPerception
	case PerceptionActionQueue:
		return nil
	default:
		err := fmt.Errorf("agent: unknown perception action %q", result.Action)
		spanErr = err
		return err
	}
}

var errIgnoredByPerception = fmt.Errorf("agent: ignored by perception")

func (p *Processor) log(ctx context.Context) telem.ContextLogger {
	return telem.Logger(ctx).With(
		telem.String("component", "processor"),
		telem.String("module", "agent"),
	)
}
