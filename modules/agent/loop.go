package agent

import (
	"context"
	"fmt"

	"github.com/iamhectordev/hector/pkg/llm/message"
)

// Runner executes one agent turn and returns the assistant reply.
type Runner interface {
	Run(ctx context.Context, messages []*message.Message) (*message.Message, error)
}

// Loop is a single-turn runner backed by a Completer.
type Loop struct {
	completer Completer
}

func NewLoop(c Completer) *Loop {
	return &Loop{completer: c}
}

func (l *Loop) Run(ctx context.Context, messages []*message.Message) (*message.Message, error) {
	reply, err := l.completer.Complete(ctx, messages)
	if err != nil {
		return nil, err
	}
	if reply == nil {
		return nil, fmt.Errorf("llm: nil reply")
	}
	return reply, nil
}
