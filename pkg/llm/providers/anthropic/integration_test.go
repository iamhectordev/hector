package anthropic_test

import (
	"os"
	"testing"

	"github.com/iamhectordev/hector/pkg/llm/providers/anthropic"
	"github.com/iamhectordev/hector/pkg/llm/schema"
	"github.com/stretchr/testify/require"
)

func TestCompleter_Complete_LiveAPI(t *testing.T) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		t.Skip("ANTHROPIC_API_KEY not set")
	}

	c := anthropic.New(apiKey, "claude-haiku-4-5-20251001") // cheapest model for integration test

	reply, err := c.Complete(t.Context(), schema.CompletionRequest{
		Messages: []*schema.Message{
			schema.UserMessage("Reply with exactly the word PONG and nothing else."),
		},
	})
	require.NoError(t, err)
	require.Equal(t, schema.FinishReasonStop, reply.FinishReason)
	require.Equal(t, "PONG", reply.Content)
}
