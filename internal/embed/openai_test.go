package embed

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/doron-cohen/klee/secrets"
	sdkopenai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/stretchr/testify/require"
)

func TestOpenAIEmbedder_Embed_CallsEndpointAndReturnsVector(t *testing.T) {
	t.Parallel()

	type requestBody struct {
		Input          string `json:"input"`
		Model          string `json:"model"`
		EncodingFormat string `json:"encoding_format"`
	}

	var got requestBody
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.True(t, strings.HasSuffix(r.URL.Path, "/embeddings"))
		require.Equal(t, "Bearer sk-test", r.Header.Get("Authorization"))
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))

		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"object": "list",
			"data": []map[string]any{
				{
					"object":    "embedding",
					"index":     0,
					"embedding": []float64{0.1, 0.2, 0.3},
				},
			},
			"model": "text-embedding-3-small",
			"usage": map[string]any{"prompt_tokens": 4, "total_tokens": 4},
		}))
	}))
	t.Cleanup(srv.Close)

	e, err := newOpenAI(OpenAIConfig{APIKey: secrets.Literal("sk-test"), Model: "text-embedding-3-small"})
	require.NoError(t, err)
	e.inner = sdkopenai.NewClient(
		option.WithAPIKey("sk-test"),
		option.WithBaseURL(srv.URL),
	)

	vec, err := e.Embed(t.Context(), "hello world")
	require.NoError(t, err)
	require.Equal(t, []float32{0.1, 0.2, 0.3}, vec)

	require.Equal(t, "hello world", got.Input)
	require.Equal(t, "text-embedding-3-small", got.Model)
}

func TestOpenAIEmbedder_Embed_ErrorsOnEmptyResponse(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"object": "list",
			"data":   []any{},
			"model":  "text-embedding-3-small",
			"usage":  map[string]any{"prompt_tokens": 0, "total_tokens": 0},
		}))
	}))
	t.Cleanup(srv.Close)

	e, err := newOpenAI(OpenAIConfig{APIKey: secrets.Literal("sk-test"), Model: "text-embedding-3-small"})
	require.NoError(t, err)
	e.inner = sdkopenai.NewClient(
		option.WithAPIKey("sk-test"),
		option.WithBaseURL(srv.URL),
	)

	vec, err := e.Embed(t.Context(), "hello")
	require.Error(t, err)
	require.Nil(t, vec)
}

func TestOpenAIEmbedder_Embed_DefaultsModel(t *testing.T) {
	t.Parallel()

	e, err := newOpenAI(OpenAIConfig{APIKey: secrets.Literal("sk-test")})
	require.NoError(t, err)
	require.Equal(t, defaultModel, e.model)
}

func TestNew_UnknownProviderErrors(t *testing.T) {
	t.Parallel()

	_, err := New(Config{Provider: "voyage"})
	require.Error(t, err)
}
