package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/iamhectordev/hector/pkg/llm"
	"github.com/iamhectordev/hector/pkg/llm/schema"
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

// Loop runs the agent turn: calls the model, executes any tool calls, and repeats
// until the model stops or returns a plain text response.
type Loop struct {
	completer llm.Completer
	tools     ToolRuntime
	log       *slog.Logger
}

// LoopOption configures a Loop.
type LoopOption func(*Loop)

// WithTools attaches tools to the loop.
func WithTools(t ToolRuntime) LoopOption {
	return func(l *Loop) { l.tools = t }
}

// WithLogger sets the logger used for debug output.
func WithLogger(logger *slog.Logger) LoopOption {
	return func(l *Loop) { l.log = logger }
}

func NewLoop(c llm.Completer, opts ...LoopOption) *Loop {
	l := &Loop{completer: c, log: slog.Default()}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

func (l *Loop) Run(ctx context.Context, agentCtx Context, system string, messages []*schema.Message) (*schema.Message, error) {
	if agentCtx == nil {
		return nil, fmt.Errorf("agent: context is required")
	}

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

		reply, err := l.completer.Complete(ctx, req)
		if err != nil {
			return nil, err
		}
		if reply == nil {
			return nil, fmt.Errorf("llm: nil reply")
		}
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
				l.log.DebugContext(ctx, "tool call", "tool", call.Name, "args", string(call.Arguments))
				if l.tools == nil {
					return nil, fmt.Errorf("agent: tool call %q requested without tools configured", call.Name)
				}
				output, execErr := l.tools.Run(ctx, call.Name, call.Arguments)
				if execErr != nil {
					l.log.DebugContext(ctx, "tool error", "tool", call.Name, "error", execErr)
					output = fmt.Sprintf("error: %s", execErr)
				} else {
					l.log.DebugContext(ctx, "tool result", "tool", call.Name, "output", output)
				}
				messages = append(messages, schema.ToolResultMessage(call.ID, output))
			}
		default:
			return nil, fmt.Errorf("llm: unexpected finish reason %q", reply.FinishReason)
		}
	}
}

func (l *Loop) history(ctx context.Context, agentCtx Context) []*schema.Message {
	messages, err := agentCtx.Messages(ctx)
	if err != nil {
		l.log.WarnContext(ctx, "session history unavailable", "err", err)
		return nil
	}
	return messages
}

func (l *Loop) record(ctx context.Context, agentCtx Context, messages []*schema.Message) {
	if len(messages) == 0 {
		return
	}

	if err := agentCtx.Record(ctx, messages); err != nil {
		l.log.WarnContext(ctx, "session record unavailable", "err", err)
	}
}
