package agent

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/iamhectordev/hector/pkg/llm"
	"github.com/iamhectordev/hector/pkg/llm/schema"
)

// Runner executes one agent turn and returns the assistant reply.
type Runner interface {
	Run(ctx context.Context, messages []*schema.Message) (*schema.Message, error)
}

// Loop runs the agent turn: calls the model, executes any tool calls, and repeats
// until the model stops or returns a plain text response.
type Loop struct {
	completer llm.Completer
	catalog   *Catalog
	system    string
	log       *slog.Logger
}

// LoopOption configures a Loop.
type LoopOption func(*Loop)

// WithCatalog attaches a tool catalog to the loop.
func WithCatalog(c *Catalog) LoopOption {
	return func(l *Loop) { l.catalog = c }
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
		if l.catalog != nil {
			req.Tools = l.catalog.Definitions()
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
				output, execErr := l.catalog.Execute(ctx, call.Name, call.Arguments)
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
