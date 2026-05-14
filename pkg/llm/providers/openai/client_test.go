package openai

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/iamhectordev/hector/pkg/llm/schema"
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

	reply, err := completer.Complete(t.Context(), schema.CompletionRequest{
		Messages: []*schema.Message{
			schema.UserMessage("hello"),
			nil,
			schema.AssistantMessage("prior reply"),
		},
	})
	require.NoError(t, err)
	want := schema.AssistantMessage("hello back")
	want.FinishReason = schema.FinishReasonStop
	require.Equal(t, want, reply)

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

	reply, err := completer.Complete(t.Context(), schema.CompletionRequest{
		Messages: []*schema.Message{
			{Role: schema.Role("developer"), Content: "nope"},
		},
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

	reply, err := completer.Complete(t.Context(), schema.CompletionRequest{
		Messages: []*schema.Message{
			schema.UserMessage("hello"),
		},
	})
	require.Error(t, err)
	require.Nil(t, reply)
}

func TestCompleter_Complete_MapsToolsAndToolCalls(t *testing.T) {
	t.Parallel()

	type requestTool struct {
		Type     string `json:"type"`
		Function struct {
			Name        string         `json:"name"`
			Description string         `json:"description"`
			Parameters  map[string]any `json:"parameters"`
		} `json:"function"`
	}
	type requestMessage struct {
		Role      string `json:"role"`
		Content   any    `json:"content,omitempty"`
		ToolCalls []struct {
			ID       string `json:"id"`
			Type     string `json:"type"`
			Function struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			} `json:"function"`
		} `json:"tool_calls,omitempty"`
		ToolCallID string `json:"tool_call_id,omitempty"`
	}
	type requestBody struct {
		Model    string           `json:"model"`
		Messages []requestMessage `json:"messages"`
		Tools    []requestTool    `json:"tools"`
	}

	var got requestBody
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
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
						"content": "",
						"tool_calls": []map[string]any{
							{
								"id":   "call_123",
								"type": "function",
								"function": map[string]any{
									"name":      "time_now",
									"arguments": `{}`,
								},
							},
						},
					},
					"finish_reason": "tool_calls",
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

	reply, err := completer.Complete(t.Context(), schema.CompletionRequest{
		Messages: []*schema.Message{
			schema.SystemMessage("You are Hector."),
			schema.UserMessage("what time is it?"),
			{
				Role: schema.RoleAssistant,
				ToolCalls: []schema.ToolCall{
					{ID: "call_prior", Name: "time.now", Arguments: json.RawMessage(`{}`)},
				},
			},
			schema.ToolResultMessage("call_prior", "Monday, 2026-05-12 10:00:00 UTC"),
		},
		Tools: []schema.ToolDefinition{
			{
				Name:        "time.now",
				Description: "Returns the current UTC time.",
				Parameters:  json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`),
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, &schema.Message{
		Role:         schema.RoleAssistant,
		FinishReason: schema.FinishReasonToolCalls,
		ToolCalls: []schema.ToolCall{
			{ID: "call_123", Name: "time.now", Arguments: json.RawMessage(`{}`)},
		},
	}, reply)

	require.Len(t, got.Tools, 1)
	require.Equal(t, "function", got.Tools[0].Type)
	require.Equal(t, "time_now", got.Tools[0].Function.Name)
	require.Equal(t, "Returns the current UTC time.", got.Tools[0].Function.Description)
	require.Equal(t, "object", got.Tools[0].Function.Parameters["type"])

	require.Len(t, got.Messages, 4)
	require.Equal(t, "system", got.Messages[0].Role)
	require.Equal(t, "user", got.Messages[1].Role)
	require.Equal(t, "assistant", got.Messages[2].Role)
	require.Len(t, got.Messages[2].ToolCalls, 1)
	require.Equal(t, "call_prior", got.Messages[2].ToolCalls[0].ID)
	require.Equal(t, "time_now", got.Messages[2].ToolCalls[0].Function.Name)
	require.Equal(t, "tool", got.Messages[3].Role)
	require.Equal(t, "call_prior", got.Messages[3].ToolCallID)
}
