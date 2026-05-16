package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/iamhectordev/hector/pkg/llm"
	"github.com/iamhectordev/hector/pkg/llm/schema"
	"github.com/iamhectordev/hector/pkg/session"
)

// Runner executes one agent turn and returns the assistant reply.
type Runner interface {
	Run(ctx context.Context, system string, messages []*schema.Message) (*schema.Message, error)
}

// ToolRuntime provides the model-facing tool catalog and executes tool calls.
type ToolRuntime interface {
	Definitions() []schema.ToolDefinition
	Run(ctx context.Context, name string, args json.RawMessage) (string, error)
}

// SessionStore records model-facing messages for a source session.
type SessionStore interface {
	Record(ctx context.Context, sourceURI string, messages []*schema.Message) error
}

// Loop runs the agent turn: calls the model, executes any tool calls, and repeats
// until the model stops or returns a plain text response.
type Loop struct {
	completer llm.Completer
	tools     ToolRuntime
	log       *slog.Logger
	sessions  SessionStore
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

// WithSessionStore records the model-facing transcript for each source session.
func WithSessionStore(store SessionStore) LoopOption {
	return func(l *Loop) { l.sessions = store }
}

func NewLoop(c llm.Completer, opts ...LoopOption) *Loop {
	l := &Loop{completer: c, log: slog.Default()}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

func (l *Loop) Run(ctx context.Context, system string, messages []*schema.Message) (*schema.Message, error) {
	recordedThrough := 0

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
		exchange := append([]*schema.Message{}, messages[recordedThrough:]...)
		exchange = append(exchange, reply)
		if err := l.record(ctx, exchange); err != nil {
			return nil, err
		}

		switch reply.FinishReason {
		case schema.FinishReasonStop:
			return reply, nil
		case schema.FinishReasonToolCalls:
			messages = append(messages, reply)
			recordedThrough = len(messages)
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

func (l *Loop) record(ctx context.Context, messages []*schema.Message) error {
	if l.sessions == nil || len(messages) == 0 {
		return nil
	}

	sess, ok := session.From(ctx)
	if !ok {
		return fmt.Errorf("agent: record session: no session in context")
	}

	return l.sessions.Record(ctx, sess.SourceURI, messages)
}
