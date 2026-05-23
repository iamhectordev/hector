package echo_test

import (
	"encoding/json"
	"testing"

	"github.com/iamhectordev/hector/pkg/llm/providers/echo"
	"github.com/iamhectordev/hector/pkg/llm/schema"
	"github.com/stretchr/testify/require"
)

func TestEchoCompleter_ReturnsLastUserMessageAsAssistant(t *testing.T) {
	t.Parallel()
	var c echo.Completer

	reply, err := c.Complete(t.Context(), schema.CompletionRequest{
		Messages: []*schema.Message{
			schema.UserMessage("first"),
			schema.AssistantMessage("ignored"),
			schema.UserMessage("last"),
		},
		Tools: []schema.ToolDefinition{
			{Name: "time_now"},
		},
	})

	require.NoError(t, err)
	require.Equal(t, schema.RoleAssistant, reply.Role)
	require.Equal(t, "last", reply.Content)
	require.Equal(t, schema.FinishReasonStop, reply.FinishReason)
}

func TestEchoCompleter_WithReplyTool_ReturnsToolCall(t *testing.T) {
	t.Parallel()
	var c echo.Completer

	reply, err := c.Complete(t.Context(), schema.CompletionRequest{
		Messages: []*schema.Message{
			schema.UserMessage("first"),
			schema.AssistantMessage("ignored"),
			schema.UserMessage("use tool"),
		},
		Tools: []schema.ToolDefinition{
			{Name: "reply"},
			{Name: "time_now"},
		},
	})

	require.NoError(t, err)
	require.Equal(t, schema.RoleAssistant, reply.Role)
	require.Empty(t, reply.Content)
	require.Equal(t, schema.FinishReasonToolCalls, reply.FinishReason)
	require.Len(t, reply.ToolCalls, 1)

	call := reply.ToolCalls[0]
	require.Equal(t, "reply", call.Name)

	var args map[string]string
	require.NoError(t, json.Unmarshal(call.Arguments, &args))
	require.Equal(t, "use tool", args["text"])
}

func TestEchoCompleter_AfterToolResult_Stops(t *testing.T) {
	t.Parallel()
	var c echo.Completer

	reply, err := c.Complete(t.Context(), schema.CompletionRequest{
		Messages: []*schema.Message{
			schema.UserMessage("use tool"),
			schema.ToolResultMessage("call_123", "ok"),
		},
		Tools: []schema.ToolDefinition{
			{Name: "reply"},
		},
	})

	require.NoError(t, err)
	require.Equal(t, schema.RoleAssistant, reply.Role)
	require.Empty(t, reply.Content)
	require.Equal(t, schema.FinishReasonStop, reply.FinishReason)
}

func TestEchoCompleter_NoUserMessage_ReturnsEmptyAssistantReply(t *testing.T) {
	t.Parallel()
	var c echo.Completer

	reply, err := c.Complete(t.Context(), schema.CompletionRequest{
		Messages: []*schema.Message{
			schema.AssistantMessage("only assistant"),
		},
	})

	require.NoError(t, err)
	require.Equal(t, schema.RoleAssistant, reply.Role)
	require.Empty(t, reply.Content)
	require.Equal(t, schema.FinishReasonStop, reply.FinishReason)
}

func TestEchoCompleter_EmptyHistory_ReturnsEmptyAssistantReply(t *testing.T) {
	t.Parallel()
	var c echo.Completer

	reply, err := c.Complete(t.Context(), schema.CompletionRequest{})

	require.NoError(t, err)
	require.Equal(t, schema.RoleAssistant, reply.Role)
	require.Empty(t, reply.Content)
	require.Equal(t, schema.FinishReasonStop, reply.FinishReason)
}
