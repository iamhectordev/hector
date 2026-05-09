package echo

import (
	"context"

	"github.com/iamhectordev/hector/modules/agent"
	"github.com/iamhectordev/hector/pkg/llm/message"
)

// Completer returns the last user message as an assistant reply.
// Used in tests and dev mode (no API key required).
type Completer struct{}

var _ agent.Completer = (*Completer)(nil)

func (Completer) Complete(_ context.Context, messages []*message.Message) (*message.Message, error) {
	if messages == nil {
		return message.AssistantMessage(""), nil
	}
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i] != nil && messages[i].Role == message.User {
			return message.AssistantMessage(messages[i].Content), nil
		}
	}
	return message.AssistantMessage(""), nil
}
