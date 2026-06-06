package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/iamhectordev/hector/pkg/llm"
	"github.com/iamhectordev/hector/pkg/llm/schema"
	"github.com/iamhectordev/hector/pkg/session"
	"github.com/iamhectordev/hector/pkg/telem"
)

// Runner executes one agent turn and returns the assistant reply.
type Runner interface {
	Run(ctx context.Context, agentCtx Context, system string, messages []*schema.Message) (*schema.Message, error)
}

// ToolRuntime provides the model-facing tool catalog and executes tool calls.
type ToolRuntime interface {
	Definitions() []schema.ToolDefinition
	Run(ctx context.Context, name string, args json.RawMessage) (string, error)
}

type sessionProvider interface {
	Session(ctx context.Context) (session.Session, error)
}

// Loop runs the agent turn: calls the model, executes any tool calls, and repeats
// until the model stops or returns a plain text response.
type Loop struct {
	completer llm.Completer
	tools     ToolRuntime
}

// LoopOption configures a Loop.
type LoopOption func(*Loop)

// WithTools attaches tools to the loop.
func WithTools(t ToolRuntime) LoopOption {
	return func(l *Loop) { l.tools = t }
}

func NewLoop(c llm.Completer, opts ...LoopOption) *Loop {
	l := &Loop{completer: c}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

func (l *Loop) Run(ctx context.Context, agentCtx Context, system string, messages []*schema.Message) (reply *schema.Message, err error) {
	if agentCtx == nil {
		return nil, fmt.Errorf("agent: context is required")
	}
	ctx = l.withSession(ctx, agentCtx)
	ctx, span := telem.Trace(ctx, spanTurnRun, turnFields(messages, l.tools)...)
	defer span.End(&err)

	history := l.history(ctx, agentCtx)
	if len(history) > 0 {
		merged := append([]*schema.Message{}, history...)
		merged = append(merged, messages...)
		messages = merged
	}
	newMessagesStart := len(history)

	for {
		req := schema.CompletionRequest{System: system, Messages: messages}
		if l.tools != nil {
			req.Tools = l.tools.Definitions()
		}

		completeCtx, completeSpan := telem.Trace(ctx, llm.SpanComplete, llm.CompleteFields(l.completer, req)...)
		reply, err := l.completer.Complete(completeCtx, req)
		if err != nil {
			completeSpan.End(&err)
			return nil, err
		}
		if reply == nil {
			err = fmt.Errorf("llm: nil reply")
			completeSpan.End(&err)
			return nil, err
		}
		telem.Event(completeCtx, "llm.completed", telem.String("llm.finish_reason", string(reply.FinishReason)))
		completeSpan.End(&err)
		exchange := append([]*schema.Message{}, messages[newMessagesStart:]...)
		exchange = append(exchange, reply)
		l.record(ctx, agentCtx, exchange)

		switch reply.FinishReason {
		case schema.FinishReasonStop:
			return reply, nil
		case schema.FinishReasonToolCalls:
			messages = append(messages, reply)
			newMessagesStart = len(messages)
			for _, call := range reply.ToolCalls {
				if l.tools == nil {
					return nil, fmt.Errorf("agent: tool call %q requested without tools configured", call.Name)
				}
				logFields := toolCallFields(call)
				toolCtx, toolSpan := telem.Trace(ctx, spanToolCall, toolCallFields(call)...)
				l.log(ctx).DebugContext(ctx, "tool call", logFields...)
				output, execErr := l.tools.Run(toolCtx, call.Name, call.Arguments)
				toolSpan.End(&execErr)
				if execErr != nil {
					l.log(ctx).DebugContext(ctx, "tool error", append(logFields, telem.Any("error", execErr))...)
					output = fmt.Sprintf("error: %s", execErr)
				} else {
					l.log(ctx).DebugContext(ctx, "tool result", logFields...)
				}
				messages = append(messages, schema.ToolResultMessage(call.ID, output))
			}
		default:
			return nil, fmt.Errorf("llm: unexpected finish reason %q", reply.FinishReason)
		}
	}
}

func (l *Loop) withSession(ctx context.Context, agentCtx Context) context.Context {
	provider, ok := agentCtx.(sessionProvider)
	if !ok {
		return ctx
	}
	s, err := provider.Session(ctx)
	if err != nil {
		l.log(ctx).WarnContext(ctx, "session metadata unavailable", telem.Any("err", err))
		return ctx
	}
	if s.ID == "" && s.SourceURI == "" {
		return ctx
	}
	ctx = telem.WithBaggage(ctx, sessionFields(s)...)
	return session.With(ctx, s)
}

func (l *Loop) history(ctx context.Context, agentCtx Context) []*schema.Message {
	messages, err := agentCtx.Messages(ctx)
	if err != nil {
		l.log(ctx).WarnContext(ctx, "session history unavailable", telem.Any("err", err))
		return nil
	}
	return messages
}

func (l *Loop) record(ctx context.Context, agentCtx Context, messages []*schema.Message) {
	if len(messages) == 0 {
		return
	}

	if err := agentCtx.Record(ctx, messages); err != nil {
		l.log(ctx).WarnContext(ctx, "session record unavailable", telem.Any("err", err))
	}
}

func (l *Loop) log(ctx context.Context) telem.ContextLogger {
	return telem.Logger(ctx).With(
		telem.String("component", "loop"),
	)
}
