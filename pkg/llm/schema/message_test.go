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
