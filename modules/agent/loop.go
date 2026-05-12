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

// Loop is a single-turn runner backed by a Completer.
type Loop struct {
	completer llm.Completer
}

func NewLoop(c llm.Completer) *Loop {
	return &Loop{completer: c}
}

func (l *Loop) Run(ctx context.Context, messages []*schema.Message) (*schema.Message, error) {
	reply, err := l.completer.Complete(ctx, schema.CompletionRequest{Messages: messages})
	if err != nil {
		return nil, err
	}
	if reply == nil {
		return nil, fmt.Errorf("llm: nil reply")
	}
	return reply, nil
}
