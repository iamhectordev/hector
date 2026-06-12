package agent_test

import (
	"testing"

	"github.com/iamhectordev/hector/modules/agent"
	"github.com/iamhectordev/hector/pkg/llm/schema"
	llmtest "github.com/iamhectordev/hector/pkg/llm/testing"
	"github.com/stretchr/testify/require"
)

func TestStructuredPerceiver_AssessSeparatesHistoryAndIncoming(t *testing.T) {
	completer := llmtest.NewCompleter(t, llmtest.ToolCalls(
		llmtest.Call("call_1", "produce_result", `{"action":"queue","reason":"direct ask"}`),
	))
	perceiver, err := agent.NewStructuredPerceiver(completer)
	require.NoError(t, err)

	result, err := perceiver.Assess(t.Context(),
		[]*schema.Message{schema.UserMessage("earlier context")},
		[]*schema.Message{schema.UserMessage("new question")},
	)
	require.NoError(t, err)
	require.Equal(t, agent.PerceptionResult{
		Action: agent.PerceptionActionQueue,
		Reason: "direct ask",
	}, result)

	require.Len(t, completer.Requests, 1)
	require.Len(t, completer.Requests[0].Messages, 2)
	require.Contains(t, completer.Requests[0].Messages[0].Content, "<history>")
	require.Contains(t, completer.Requests[0].Messages[0].Content, "earlier context")
	require.Contains(t, completer.Requests[0].Messages[1].Content, "<incoming>")
	require.Contains(t, completer.Requests[0].Messages[1].Content, "new question")
}
