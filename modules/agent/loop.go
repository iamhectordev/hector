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
	Run(ctx context.Context, messages []*schema.Message) (*schema.Message, error)
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
	system    string
	log       *slog.Logger
}

// LoopOption configures a Loop.
type LoopOption func(*Loop)

// WithTools attaches tools to the loop.
func WithTools(t ToolRuntime) LoopOption {
	return func(l *Loop) { l.tools = t }
}

// WithSystem sets the system prompt for every turn.
func WithSystem(prompt string) LoopOption {
	return func(l *Loop) { l.system = prompt }
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

func (l *Loop) Run(ctx context.Context, messages []*schema.Message) (*schema.Message, error) {
	for {
		req := schema.CompletionRequest{System: l.system, Messages: messages}
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
		switch reply.FinishReason {
		case schema.FinishReasonStop:
			return reply, nil
		case schema.FinishReasonToolCalls:
			messages = append(messages, reply)
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
