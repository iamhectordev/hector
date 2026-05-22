package openai

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/iamhectordev/hector/pkg/llm/schema"
	"github.com/iamhectordev/hector/pkg/session"
	sdkopenai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/stretchr/testify/require"
)

func TestCompleter_Complete_LogsRequestAndResponseBodiesForSession(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"id":      "chatcmpl_123",
			"object":  "chat.completion",
			"created": 1,
			"model":   "gpt-4o-mini",
			"choices": []map[string]any{{
				"index": 0,
				"message": map[string]any{
					"role":    "assistant",
					"content": "ok",
				},
				"finish_reason": "stop",
			}},
		}))
	}))
	t.Cleanup(srv.Close)

	logDir := t.TempDir()
	completer := New("sk-test", "", WithBodyLog(BodyLogConfig{
		Enabled: true,
		Dir:     logDir,
	}))
	completer.inner = sdkopenai.NewClient(
		option.WithAPIKey("sk-test"),
		option.WithBaseURL(srv.URL),
		option.WithMiddleware(completer.bodyLogMiddleware()),
	)

	ctx := session.With(t.Context(), session.Session{ID: "sess_123", SourceURI: "slack://D123/1"})
	reply, err := completer.Complete(ctx, schema.CompletionRequest{
		Messages: []*schema.Message{schema.UserMessage("hello")},
	})
	require.NoError(t, err)
	require.Equal(t, schema.FinishReasonStop, reply.FinishReason)

	records := readBodyLogRecords(t, filepath.Join(logDir, "sess_123.jsonl"))
	require.Len(t, records, 2)
	require.Equal(t, "request", records[0].Direction)
	require.Equal(t, "gpt-4o-mini", records[0].Body["model"])
	require.Equal(t, "response", records[1].Direction)
	require.Equal(t, "chatcmpl_123", records[1].Body["id"])
}

func TestCompleter_Complete_BodyLogIncludesImageContentParts(t *testing.T) {
	t.Parallel()

	srv := newOpenAITestServer(t)
	logDir := t.TempDir()
	completer := New("sk-test", "", WithBodyLog(BodyLogConfig{
		Enabled: true,
		Dir:     logDir,
	}))
	completer.inner = sdkopenai.NewClient(
		option.WithAPIKey("sk-test"),
		option.WithBaseURL(srv.URL),
		option.WithMiddleware(completer.bodyLogMiddleware()),
	)

	ctx := session.With(t.Context(), session.Session{ID: "sess_123"})
	_, err := completer.Complete(ctx, schema.CompletionRequest{
		Messages: []*schema.Message{
			schema.UserMessageWithParts("fallback", []schema.MessagePart{
				schema.TextPart("<msg><img id=\"F123\"></img></msg>"),
				schema.TextPart(`<image_data id="F123"/>`),
				schema.NewImagePart("F123", "aW1hZ2U=", "image/png"),
			}),
		},
	})
	require.NoError(t, err)

	records := readBodyLogRecords(t, filepath.Join(logDir, "sess_123.jsonl"))
	messages := records[0].Body["messages"].([]any)
	user := messages[0].(map[string]any)
	content := user["content"].([]any)
	image := content[2].(map[string]any)
	require.Equal(t, "image_url", image["type"])
	require.Equal(t, map[string]any{
		"url": "data:image/png;base64,aW1hZ2U=",
	}, image["image_url"])
}

func TestCompleter_Complete_DoesNotLogBodiesWhenDisabledOrSessionMissing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		bodyLog BodyLogConfig
		ctx     func(*testing.T) context.Context
	}{
		{
			name:    "disabled",
			bodyLog: BodyLogConfig{Enabled: false},
			ctx: func(t *testing.T) context.Context {
				return session.With(t.Context(), session.Session{ID: "sess_123"})
			},
		},
		{
			name:    "missing session id",
			bodyLog: BodyLogConfig{Enabled: true},
			ctx: func(t *testing.T) context.Context {
				return session.With(t.Context(), session.Session{SourceURI: "slack://D123/1"})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			srv := newOpenAITestServer(t)
			logDir := t.TempDir()
			tt.bodyLog.Dir = logDir
			completer := New("sk-test", "", WithBodyLog(tt.bodyLog))
			completer.inner = sdkopenai.NewClient(
				option.WithAPIKey("sk-test"),
				option.WithBaseURL(srv.URL),
				option.WithMiddleware(completer.bodyLogMiddleware()),
			)

			reply, err := completer.Complete(tt.ctx(t), schema.CompletionRequest{
				Messages: []*schema.Message{schema.UserMessage("hello")},
			})
			require.NoError(t, err)
			require.Equal(t, schema.FinishReasonStop, reply.FinishReason)

			entries, err := os.ReadDir(logDir)
			require.NoError(t, err)
			require.Empty(t, entries)
		})
	}
}

type bodyLogRecord struct {
	Direction string         `json:"direction"`
	Body      map[string]any `json:"body"`
}

func readBodyLogRecords(t *testing.T, path string) []bodyLogRecord {
	t.Helper()

	file, err := os.Open(path)
	require.NoError(t, err)
	defer func() { require.NoError(t, file.Close()) }()

	records := make([]bodyLogRecord, 0)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var record bodyLogRecord
		require.NoError(t, json.Unmarshal(scanner.Bytes(), &record))
		records = append(records, record)
	}
	require.NoError(t, scanner.Err())
	return records
}

func newOpenAITestServer(t *testing.T) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"id":      "chatcmpl_123",
			"object":  "chat.completion",
			"created": 1,
			"model":   "gpt-4o-mini",
			"choices": []map[string]any{{
				"index": 0,
				"message": map[string]any{
					"role":    "assistant",
					"content": "ok",
				},
				"finish_reason": "stop",
			}},
		}))
	}))
	t.Cleanup(srv.Close)
	return srv
}
