package structured_test

import (
	"errors"
	"testing"

	"github.com/iamhectordev/hector/pkg/llm/schema"
	llmtest "github.com/iamhectordev/hector/pkg/llm/testing"
	"github.com/iamhectordev/hector/pkg/llm/structured"
	"github.com/stretchr/testify/require"
)

type fact struct {
	Content string `json:"content"`
}

func TestExtractor_ExtractsTypedResult(t *testing.T) {
	completer := llmtest.NewCompleter(t,
		llmtest.ToolCalls(llmtest.Call("c1", "produce_result", `{"content":"auth service uses go"}`)),
	)
	extractor, err := structured.NewExtractor[fact](completer, "extract the key fact")
	require.NoError(t, err)

	got, err := extractor.Extract(t.Context(), []*schema.Message{schema.UserMessage("what does auth use?")})
	require.NoError(t, err)
	require.Equal(t, "auth service uses go", got.Content)
}

func TestExtractor_ForcesToolChoiceOnRequest(t *testing.T) {
	completer := llmtest.NewCompleter(t,
		llmtest.ToolCalls(llmtest.Call("c1", "produce_result", `{"content":"x"}`)),
	)
	extractor, err := structured.NewExtractor[fact](completer, "test")
	require.NoError(t, err)

	_, err = extractor.Extract(t.Context(), []*schema.Message{schema.UserMessage("hi")})
	require.NoError(t, err)

	req := completer.Requests[0]
	require.NotNil(t, req.ToolChoice)
	require.Equal(t, "produce_result", req.ToolChoice.Name)
}

func TestExtractor_ErrorsWhenNoToolCall(t *testing.T) {
	completer := llmtest.NewCompleter(t, llmtest.Stop("I cannot extract that"))
	extractor, err := structured.NewExtractor[fact](completer, "test")
	require.NoError(t, err)

	_, err = extractor.Extract(t.Context(), []*schema.Message{schema.UserMessage("hi")})
	require.Error(t, err)
	require.ErrorContains(t, err, "produce_result")
}

func TestExtractor_PropagatesCompleterError(t *testing.T) {
	boom := errors.New("llm down")
	completer := llmtest.NewCompleter(t, llmtest.Error(boom))
	extractor, err := structured.NewExtractor[fact](completer, "test")
	require.NoError(t, err)

	_, err = extractor.Extract(t.Context(), []*schema.Message{schema.UserMessage("hi")})
	require.ErrorIs(t, err, boom)
}

func TestExtractor_SendsSystemPromptAndMessages(t *testing.T) {
	completer := llmtest.NewCompleter(t,
		llmtest.ToolCalls(llmtest.Call("c1", "produce_result", `{"content":"x"}`)),
	)
	extractor, err := structured.NewExtractor[fact](completer, "my system prompt")
	require.NoError(t, err)

	msgs := []*schema.Message{schema.UserMessage("hello"), schema.AssistantMessage("world")}
	_, err = extractor.Extract(t.Context(), msgs)
	require.NoError(t, err)

	req := completer.Requests[0]
	require.Equal(t, "my system prompt", req.System)
	require.Equal(t, msgs, req.Messages)
}
