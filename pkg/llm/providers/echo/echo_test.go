package echo_test

import (
	"testing"

	"github.com/iamhectordev/hector/pkg/llm/message"
	"github.com/iamhectordev/hector/pkg/llm/providers/echo"
	"github.com/stretchr/testify/require"
)

func TestEchoCompleter_ReturnsLastUserMessageAsAssistant(t *testing.T) {
	t.Parallel()
	var c echo.Completer

	reply, err := c.Complete(t.Context(), []*message.Message{
		message.UserMessage("first"),
		message.AssistantMessage("ignored"),
		message.UserMessage("last"),
	})

	require.NoError(t, err)
	require.Equal(t, message.Assistant, reply.Role)
	require.Equal(t, "last", reply.Content)
}

func TestEchoCompleter_NoUserMessage_ReturnsEmptyAssistantReply(t *testing.T) {
	t.Parallel()
	var c echo.Completer

	reply, err := c.Complete(t.Context(), []*message.Message{
		message.AssistantMessage("only assistant"),
	})

	require.NoError(t, err)
	require.Equal(t, message.Assistant, reply.Role)
	require.Empty(t, reply.Content)
}

func TestEchoCompleter_EmptyHistory_ReturnsEmptyAssistantReply(t *testing.T) {
	t.Parallel()
	var c echo.Completer

	reply, err := c.Complete(t.Context(), nil)

	require.NoError(t, err)
	require.Equal(t, message.Assistant, reply.Role)
	require.Empty(t, reply.Content)
}
