package echo_test

import (
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
			{Name: "time.now"},
		},
	})

	require.NoError(t, err)
	require.Equal(t, schema.RoleAssistant, reply.Role)
	require.Equal(t, "last", reply.Content)
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
}

func TestEchoCompleter_EmptyHistory_ReturnsEmptyAssistantReply(t *testing.T) {
	t.Parallel()
	var c echo.Completer

	reply, err := c.Complete(t.Context(), schema.CompletionRequest{})

	require.NoError(t, err)
	require.Equal(t, schema.RoleAssistant, reply.Role)
	require.Empty(t, reply.Content)
}
