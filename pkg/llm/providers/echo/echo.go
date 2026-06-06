package echo

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/iamhectordev/hector/pkg/llm/schema"
	"github.com/iamhectordev/hector/pkg/telem"
)

// Completer returns the last user message as an assistant reply.
// If the 'reply' tool is provided in the request tools, it will
// generate a tool call instead of plain text, and then stop on the
// subsequent tool result.
// Used in tests and dev mode (no API key required).
type Completer struct{}

func (Completer) TelemetryFields() []telem.Field {
	return []telem.Field{
		telem.String("llm.provider", "echo"),
		telem.String("llm.model", "echo"),
	}
}

func (Completer) Complete(_ context.Context, req schema.CompletionRequest) (*schema.Message, error) {
	stop := func(content string) *schema.Message {
		m := schema.AssistantMessage(content)
		m.FinishReason = schema.FinishReasonStop
		return m
	}
	if len(req.Messages) == 0 {
		return stop(""), nil
	}

	// Stop if the last message was a tool result to prevent infinite loops.
	lastMsg := req.Messages[len(req.Messages)-1]
	if lastMsg != nil && lastMsg.Role == schema.RoleTool {
		return stop(""), nil
	}

	hasReplyTool := false
	for _, t := range req.Tools {
		if t.Name == "reply" {
			hasReplyTool = true
			break
		}
	}

	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i] != nil && req.Messages[i].Role == schema.RoleUser {
			content := req.Messages[i].Content
			if hasReplyTool {
				args, _ := json.Marshal(map[string]string{"text": content})
				m := schema.AssistantMessage("")
				m.FinishReason = schema.FinishReasonToolCalls
				m.ToolCalls = []schema.ToolCall{{
					ID:        "call_" + uuid.NewString()[:8],
					Name:      "reply",
					Arguments: args,
				}}
				return m, nil
			}
			return stop(content), nil
		}
	}
	return stop(""), nil
}
