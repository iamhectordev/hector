package agent

import (
	"context"

	"github.com/iamhectordev/hector/pkg/llm/schema"
)

type PerceptionAction string

const (
	PerceptionActionIgnore PerceptionAction = "ignore"
	PerceptionActionQueue  PerceptionAction = "queue"
)

type PerceptionResult struct {
	Action PerceptionAction `json:"action"`
	Reason string           `json:"reason"`
}

type Perceiver interface {
	Assess(
		ctx context.Context,
		history []*schema.Message,
		incoming []*schema.Message,
	) (PerceptionResult, error)
}
