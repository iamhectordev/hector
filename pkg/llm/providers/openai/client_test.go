package openai

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/iamhectordev/hector/pkg/llm/message"
	sdkopenai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/stretchr/testify/require"
)

func TestCompleter_Complete_MapsMessagesAndReturnsAssistantReply(t *testing.T) {
	t.Parallel()

	type requestMessage struct {
		Role    string `json:"role"`
		Content any    `json:"content"`
	}
	type requestBody struct {
		Model    string           `json:"model"`
		Messages []requestMessage `json:"messages"`
	}

	var got requestBody
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.True(t, strings.HasSuffix(r.URL.Path, "/chat/completions"))
		require.Equal(t, "Bearer sk-test", r.Header.Get("Authorization"))
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))

		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"id":      "chatcmpl_123",
			"object":  "chat.completion",
			"created": 1,
			"model":   "gpt-4o-mini",
			"choices": []map[string]any{
				{
					"index": 0,
					"message": map[string]any{
						"role":    "assistant",
						"content": "hello back",
					},
					"finish_reason": "stop",
				},
			},
		}))
	}))
	t.Cleanup(srv.Close)

	completer := New("sk-test", "")
	completer.inner = sdkopenai.NewClient(
		option.WithAPIKey("sk-test"),
		option.WithBaseURL(srv.URL),
	)

	reply, err := completer.Complete(t.Context(), []*message.Message{
		message.UserMessage("hello"),
		nil,
		message.AssistantMessage("prior reply"),
	})
	require.NoError(t, err)
	require.Equal(t, message.AssistantMessage("hello back"), reply)

	require.Equal(t, defaultModel, got.Model)
	require.Len(t, got.Messages, 2)
	require.Equal(t, "user", got.Messages[0].Role)
	require.Equal(t, "hello", got.Messages[0].Content)
	require.Equal(t, "assistant", got.Messages[1].Role)
	require.Equal(t, "prior reply", got.Messages[1].Content)
}

func TestCompleter_Complete_RejectsUnknownRole(t *testing.T) {
	t.Parallel()

	completer := New("sk-test", "")

	reply, err := completer.Complete(t.Context(), []*message.Message{
		{Role: message.RoleType("system"), Content: "nope"},
	})
	require.Error(t, err)
	require.Nil(t, reply)
}

func TestCompleter_Complete_RequiresChoice(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"id":      "chatcmpl_123",
			"object":  "chat.completion",
			"created": 1,
			"model":   "gpt-4o-mini",
			"choices": []map[string]any{},
		}))
	}))
	t.Cleanup(srv.Close)

	completer := New("sk-test", "")
	completer.inner = sdkopenai.NewClient(
		option.WithAPIKey("sk-test"),
		option.WithBaseURL(srv.URL),
	)

	reply, err := completer.Complete(t.Context(), []*message.Message{
		message.UserMessage("hello"),
	})
	require.Error(t, err)
	require.Nil(t, reply)
}
