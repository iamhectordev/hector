package schema_test

import (
	"encoding/json"
	"testing"

	"github.com/iamhectordev/hector/pkg/llm/schema"
	"github.com/stretchr/testify/require"
)

func TestMessageJSONOmitsEmptyFields(t *testing.T) {
	raw, err := json.Marshal(schema.UserMessage("hello"))
	require.NoError(t, err)
	require.JSONEq(t, `{"Role":"user","Content":"hello"}`, string(raw))
}

func TestMessageJSONKeepsToolFieldsWhenSet(t *testing.T) {
	msg := &schema.Message{
		Role: schema.RoleAssistant,
		ToolCalls: []schema.ToolCall{{
			ID:   "call_1",
			Name: "reply",
		}},
		FinishReason: schema.FinishReasonToolCalls,
	}

	raw, err := json.Marshal(msg)
	require.NoError(t, err)
	require.JSONEq(t, `{
		"Role": "assistant",
		"ToolCalls": [{"ID": "call_1", "Name": "reply", "Arguments": null}],
		"FinishReason": "tool_calls"
	}`, string(raw))
}

func TestMessageJSONPersistsImageParts(t *testing.T) {
	msg := schema.UserMessageWithParts("inspect this", []schema.MessagePart{
		schema.TextPart("inspect this"),
		schema.NewImagePart("F123", "aW1hZ2U=", "image/png"),
	})

	raw, err := json.Marshal(msg)
	require.NoError(t, err)
	require.JSONEq(t, `{
		"Role": "user",
		"Content": "inspect this",
		"Parts": [
			{"Type": "text", "Text": "inspect this"},
			{"Type": "image", "Image": {
				"ID": "F123",
				"Base64Data": "aW1hZ2U=",
				"MIMEType": "image/png"
			}}
		]
	}`, string(raw))
}
