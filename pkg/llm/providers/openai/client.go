package openai

import (
	"context"
	"fmt"
	"strings"

	"github.com/iamhectordev/hector/modules/agent"
	"github.com/iamhectordev/hector/pkg/llm/message"
	sdkopenai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

const defaultModel = "gpt-4o-mini"

// Completer sends chat history to OpenAI Chat Completions.
type Completer struct {
	inner sdkopenai.Client
	model string
}

var _ agent.Completer = (*Completer)(nil)

func New(apiKey, model string) *Completer {
	model = strings.TrimSpace(model)
	if model == "" {
		model = defaultModel
	}

	return &Completer{
		inner: sdkopenai.NewClient(option.WithAPIKey(strings.TrimSpace(apiKey))),
		model: model,
	}
}

func (c *Completer) Complete(ctx context.Context, messages []*message.Message) (*message.Message, error) {
	params := make([]sdkopenai.ChatCompletionMessageParamUnion, 0, len(messages))
	for _, msg := range messages {
		if msg == nil {
			continue
		}

		switch msg.Role {
		case message.User:
			params = append(params, sdkopenai.UserMessage(msg.Content))
		case message.Assistant:
			params = append(params, sdkopenai.AssistantMessage(msg.Content))
		default:
			return nil, fmt.Errorf("llm: unsupported role %q", msg.Role)
		}
	}

	if len(params) == 0 {
		return nil, fmt.Errorf("llm: no messages")
	}

	completion, err := c.inner.Chat.Completions.New(ctx, sdkopenai.ChatCompletionNewParams{
		Model:    sdkopenai.ChatModel(c.model),
		Messages: params,
	})
	if err != nil {
		return nil, err
	}
	if completion == nil {
		return nil, fmt.Errorf("llm: nil completion")
	}
	if len(completion.Choices) == 0 {
		return nil, fmt.Errorf("llm: no choices returned")
	}

	return message.AssistantMessage(completion.Choices[0].Message.Content), nil
}
