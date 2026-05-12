package echo

import (
	"context"

	"github.com/iamhectordev/hector/pkg/llm/schema"
)

// Completer returns the last user message as an assistant reply.
// Used in tests and dev mode (no API key required).
type Completer struct{}

func (Completer) Complete(_ context.Context, req schema.CompletionRequest) (*schema.Message, error) {
	if req.Messages == nil {
		return schema.AssistantMessage(""), nil
	}
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i] != nil && req.Messages[i].Role == schema.RoleUser {
			return schema.AssistantMessage(req.Messages[i].Content), nil
		}
	}
	return schema.AssistantMessage(""), nil
}
