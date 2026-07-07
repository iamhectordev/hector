package anthropic_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	sdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/iamhectordev/hector/pkg/llm/providers/anthropic"
	"github.com/iamhectordev/hector/pkg/llm/schema"
	"github.com/stretchr/testify/require"
)

func TestCompleter_Complete_MapsUserMessageAndReturnsText(t *testing.T) {
	t.Parallel()

	var gotBody map[string]any
	srv := newAnthropicTestServer(t, &gotBody, "Hello back")
	c := newTestCompleter(t, srv.URL)

	reply, err := c.Complete(t.Context(), schema.CompletionRequest{
		Messages: []*schema.Message{schema.UserMessage("Hello")},
	})
	require.NoError(t, err)
	require.Equal(t, schema.FinishReasonStop, reply.FinishReason)
	require.Equal(t, "Hello back", reply.Content)

	msgs := gotBody["messages"].([]any)
	require.Len(t, msgs, 1)
	msg := msgs[0].(map[string]any)
	require.Equal(t, "user", msg["role"])
}

func TestCompleter_Complete_MapsSystemPrompt(t *testing.T) {
	t.Parallel()

	var gotBody map[string]any
	srv := newAnthropicTestServer(t, &gotBody, "ok")
	c := newTestCompleter(t, srv.URL)

	_, err := c.Complete(t.Context(), schema.CompletionRequest{
		System:   "You are Hector.",
		Messages: []*schema.Message{schema.UserMessage("hi")},
	})
	require.NoError(t, err)
	require.Equal(t, "You are Hector.", extractSystemText(t, gotBody))

	// system role messages in the messages list are skipped
	msgs := gotBody["messages"].([]any)
	require.Len(t, msgs, 1)
}

func TestCompleter_Complete_SkipsInlineSystemMessages(t *testing.T) {
	t.Parallel()

	var gotBody map[string]any
	srv := newAnthropicTestServer(t, &gotBody, "ok")
	c := newTestCompleter(t, srv.URL)

	_, err := c.Complete(t.Context(), schema.CompletionRequest{
		Messages: []*schema.Message{
			schema.SystemMessage("You are Hector."),
			schema.UserMessage("hi"),
		},
	})
	require.NoError(t, err)

	msgs := gotBody["messages"].([]any)
	require.Len(t, msgs, 1)
	require.Equal(t, "user", msgs[0].(map[string]any)["role"])
}

func TestCompleter_Complete_PacksToolResultsIntoUserMessage(t *testing.T) {
	t.Parallel()

	var gotBody map[string]any
	srv := newAnthropicTestServer(t, &gotBody, "ok")
	c := newTestCompleter(t, srv.URL)

	_, err := c.Complete(t.Context(), schema.CompletionRequest{
		Messages: []*schema.Message{
			schema.UserMessage("what time is it?"),
			{
				Role: schema.RoleAssistant,
				ToolCalls: []schema.ToolCall{
					{ID: "call_1", Name: "time_now", Arguments: json.RawMessage(`{}`)},
				},
			},
			schema.ToolResultMessage("call_1", "2026-07-02T10:00:00Z"),
		},
	})
	require.NoError(t, err)

	msgs := gotBody["messages"].([]any)
	// user, assistant, user (tool results)
	require.Len(t, msgs, 3)

	last := msgs[2].(map[string]any)
	require.Equal(t, "user", last["role"])

	content := last["content"].([]any)
	require.Len(t, content, 1)
	block := content[0].(map[string]any)
	require.Equal(t, "tool_result", block["type"])
	require.Equal(t, "call_1", block["tool_use_id"])
	// SDK serializes string content as [{type:text, text:...}]
	blockContent := block["content"].([]any)
	require.Len(t, blockContent, 1)
	require.Equal(t, "2026-07-02T10:00:00Z", blockContent[0].(map[string]any)["text"])
}

func TestCompleter_Complete_PacksMultipleToolResultsIntoOneUserMessage(t *testing.T) {
	t.Parallel()

	var gotBody map[string]any
	srv := newAnthropicTestServer(t, &gotBody, "ok")
	c := newTestCompleter(t, srv.URL)

	_, err := c.Complete(t.Context(), schema.CompletionRequest{
		Messages: []*schema.Message{
			schema.UserMessage("hi"),
			{
				Role: schema.RoleAssistant,
				ToolCalls: []schema.ToolCall{
					{ID: "call_1", Name: "tool_a", Arguments: json.RawMessage(`{}`)},
					{ID: "call_2", Name: "tool_b", Arguments: json.RawMessage(`{}`)},
				},
			},
			schema.ToolResultMessage("call_1", "result_a"),
			schema.ToolResultMessage("call_2", "result_b"),
		},
	})
	require.NoError(t, err)

	msgs := gotBody["messages"].([]any)
	require.Len(t, msgs, 3) // user, assistant, user(2 tool_results)

	last := msgs[2].(map[string]any)
	require.Equal(t, "user", last["role"])
	content := last["content"].([]any)
	require.Len(t, content, 2)
}

func TestCompleter_Complete_HandlesEmptyAssistantMessageInHistory(t *testing.T) {
	t.Parallel()

	var gotBody map[string]any
	srv := newAnthropicTestServer(t, &gotBody, "ok")
	c := newTestCompleter(t, srv.URL)

	// Empty assistant messages can appear in history from the echo completer.
	// They must be tolerated rather than hard-errored so sessions remain usable.
	reply, err := c.Complete(t.Context(), schema.CompletionRequest{
		Messages: []*schema.Message{
			schema.UserMessage("hi"),
			schema.AssistantMessage(""),
			schema.UserMessage("what?"),
		},
	})
	require.NoError(t, err)
	require.NotNil(t, reply)

	// Empty assistant message should be sent as a single-space text block.
	msgs := gotBody["messages"].([]any)
	assistantMsg := msgs[1].(map[string]any)
	require.Equal(t, "assistant", assistantMsg["role"])
	blocks := assistantMsg["content"].([]any)
	require.Len(t, blocks, 1)
	require.Equal(t, " ", blocks[0].(map[string]any)["text"])
}

func TestCompleter_Complete_MapsToolCallsInResponse(t *testing.T) {
	t.Parallel()

	srv := newAnthropicServerWithToolUse(t, "call_123", "time_now", `{"tz":"UTC"}`)
	c := newTestCompleter(t, srv.URL)

	reply, err := c.Complete(t.Context(), schema.CompletionRequest{
		Messages: []*schema.Message{schema.UserMessage("what time?")},
		Tools: []schema.ToolDefinition{
			{Name: "time_now", Description: "Returns current time.", Parameters: json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`)},
		},
	})
	require.NoError(t, err)
	require.Equal(t, schema.FinishReasonToolCalls, reply.FinishReason)
	require.Len(t, reply.ToolCalls, 1)
	require.Equal(t, "call_123", reply.ToolCalls[0].ID)
	require.Equal(t, "time_now", reply.ToolCalls[0].Name)
	require.JSONEq(t, `{"tz":"UTC"}`, string(reply.ToolCalls[0].Arguments))
}

func TestCompleter_Complete_NormalizesAnthropicAPIError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		statusCode   int
		providerType string
		message      string
		wantKind     schema.ErrorKind
		wantRetry    bool
	}{
		{
			name:         "billing not retryable",
			statusCode:   http.StatusBadRequest,
			providerType: "invalid_request_error",
			message:      "Your credit balance is too low to access the Anthropic API.",
			wantKind:     schema.ErrorBilling,
			wantRetry:    false,
		},
		{
			name:         "overloaded retryable",
			statusCode:   529,
			providerType: "overloaded_error",
			message:      "Overloaded.",
			wantKind:     schema.ErrorOverloaded,
			wantRetry:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("request-id", "req_123")
				w.WriteHeader(tt.statusCode)
				require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
					"type": "error",
					"error": map[string]any{
						"type":    tt.providerType,
						"message": tt.message,
					},
					"request_id": "req_123",
				}))
			}))
			t.Cleanup(srv.Close)

			c := newTestCompleter(t, srv.URL)

			reply, err := c.Complete(t.Context(), schema.CompletionRequest{
				Messages: []*schema.Message{schema.UserMessage("hello")},
			})
			require.Error(t, err)
			require.Nil(t, reply)

			var llmErr *schema.Error
			require.True(t, errors.As(err, &llmErr))
			require.Equal(t, "anthropic", llmErr.Provider)
			require.Equal(t, "complete", llmErr.Operation)
			require.Equal(t, tt.wantKind, llmErr.Kind)
			require.Equal(t, tt.statusCode, llmErr.StatusCode)
			require.Equal(t, tt.providerType, llmErr.ProviderType)
			require.Equal(t, "req_123", llmErr.RequestID)
			require.Equal(t, tt.wantRetry, llmErr.Retryable())
		})
	}
}

func TestCompleter_Complete_MapsToolChoice(t *testing.T) {
	t.Parallel()

	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&gotBody))
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"id":   "msg_123",
			"type": "message",
			"role": "assistant",
			"content": []map[string]any{{
				"type":  "tool_use",
				"id":    "c1",
				"name":  "produce_result",
				"input": json.RawMessage(`{"content":"x"}`),
			}},
			"stop_reason": "tool_use",
			"usage":       map[string]any{"input_tokens": 10, "output_tokens": 5},
		}))
	}))
	t.Cleanup(srv.Close)

	c := newTestCompleter(t, srv.URL)
	_, err := c.Complete(t.Context(), schema.CompletionRequest{
		Messages: []*schema.Message{schema.UserMessage("extract")},
		Tools: []schema.ToolDefinition{{
			Name:       "produce_result",
			Parameters: json.RawMessage(`{"type":"object","properties":{}}`),
		}},
		ToolChoice: &schema.ToolChoice{Name: "produce_result"},
	})
	require.NoError(t, err)

	tc := gotBody["tool_choice"].(map[string]any)
	require.Equal(t, "tool", tc["type"])
	require.Equal(t, "produce_result", tc["name"])
}

// helpers

func newTestCompleter(t *testing.T, baseURL string) *anthropic.Completer {
	t.Helper()
	c := anthropic.New("sk-ant-test", "claude-opus-4-7")
	c.WithClientOption(option.WithBaseURL(baseURL))
	return c
}

func newAnthropicTestServer(t *testing.T, gotBody *map[string]any, replyText string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(gotBody))
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(textResponse(replyText)))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newAnthropicServerWithToolUse(t *testing.T, id, name, inputJSON string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"id":    "msg_123",
			"type":  "message",
			"role":  "assistant",
			"model": "claude-opus-4-7",
			"content": []map[string]any{{
				"type":  "tool_use",
				"id":    id,
				"name":  name,
				"input": json.RawMessage(inputJSON),
			}},
			"stop_reason": "tool_use",
			"usage":       map[string]any{"input_tokens": 10, "output_tokens": 5},
		}))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func textResponse(text string) map[string]any {
	return map[string]any{
		"id":    "msg_123",
		"type":  "message",
		"role":  "assistant",
		"model": "claude-opus-4-7",
		"content": []map[string]any{{
			"type": "text",
			"text": text,
		}},
		"stop_reason": string(sdk.StopReasonEndTurn),
		"usage":       map[string]any{"input_tokens": 10, "output_tokens": 5},
	}
}

func extractSystemText(t *testing.T, body map[string]any) string {
	t.Helper()
	sys, ok := body["system"]
	require.True(t, ok, "system field missing")
	blocks := sys.([]any)
	require.NotEmpty(t, blocks)
	return blocks[0].(map[string]any)["text"].(string)
}
