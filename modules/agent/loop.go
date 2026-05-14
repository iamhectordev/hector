package agent

import (
	"context"
	"fmt"

	"github.com/iamhectordev/hector/pkg/llm"
	"github.com/iamhectordev/hector/pkg/llm/schema"
)

// Runner executes one agent turn and returns the assistant reply.
type Runner interface {
	Run(ctx context.Context, messages []*schema.Message) (*schema.Message, error)
}

// Loop runs the agent turn: calls the model, executes any tool calls, and repeats
// until the model calls 'reply' or returns a plain text response.
type Loop struct {
	completer llm.Completer
	catalog   *Catalog
	system    string
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

func NewLoop(c llm.Completer, opts ...LoopOption) *Loop {
	l := &Loop{completer: c}
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
		if len(reply.ToolCalls) == 0 {
			return reply, nil
		}

		messages = append(messages, reply)
		replied := false
		for _, call := range reply.ToolCalls {
			output, err := l.catalog.Execute(ctx, call.Name, call.Arguments)
			if err != nil {
				output = fmt.Sprintf("error: %s", err)
			}
			messages = append(messages, schema.ToolResultMessage(call.ID, output))
			if call.Name == "reply" {
				replied = true
			}
		}
		if replied {
			return nil, nil
		}
	}
}
