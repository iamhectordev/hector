package web_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	internalfetch "github.com/iamhectordev/hector/internal/web/fetch"
	"github.com/iamhectordev/hector/modules/tools/web"
	"github.com/iamhectordev/hector/pkg/safehttp"
)

// envelope is shared with search_test.go (same package web_test).
type envelope struct {
	Status  string          `json:"status"`
	Result  json.RawMessage `json:"result,omitempty"`
	Message string          `json:"message,omitempty"`
}

type fakeFetcher struct {
	result internalfetch.Result
	err    error
}

func (f *fakeFetcher) Fetch(_ context.Context, _ string) (internalfetch.Result, error) {
	return f.result, f.err
}

func safeClient(t *testing.T, opts ...safehttp.Option) *http.Client {
	t.Helper()
	opts = append([]safehttp.Option{safehttp.WithAllowLoopback()}, opts...)
	client, err := safehttp.Client(opts...)
	require.NoError(t, err)
	return client
}

func TestFetch_Definition(t *testing.T) {
	tool, err := web.NewFetch(safeClient(t))
	require.NoError(t, err)
	def := tool.Definition()
	require.Equal(t, "web_fetch", def.Name)
	require.NotEmpty(t, def.Description)
	require.NotEmpty(t, def.Parameters)
}

func TestNewFetch_NilClient(t *testing.T) {
	tool, err := web.NewFetch(nil)
	require.Error(t, err)
	require.Nil(t, tool)
}

func TestFetch_RunMapsSuccessToOKEnvelope(t *testing.T) {
	tool, err := web.NewFetchWithFetcher(&fakeFetcher{result: internalfetch.Result{
		URL:         "https://example.com",
		FinalURL:    "https://example.com/final",
		Title:       "Example",
		ContentType: "markdown",
		Content:     "# Example",
	}})
	require.NoError(t, err)

	out, err := tool.Run(t.Context(), json.RawMessage(`{"url":"https://example.com"}`))
	require.NoError(t, err)

	var env envelope
	require.NoError(t, json.Unmarshal([]byte(out), &env))
	require.Equal(t, "ok", env.Status)

	var result internalfetch.Result
	require.NoError(t, json.Unmarshal(env.Result, &result))
	require.Equal(t, "https://example.com/final", result.FinalURL)
	require.Equal(t, "markdown", result.ContentType)
	require.Equal(t, "# Example", result.Content)
}

func TestFetch_RunMapsErrorToErrorEnvelope(t *testing.T) {
	tool, err := web.NewFetchWithFetcher(&fakeFetcher{err: errors.New("fetch_failed: http 404")})
	require.NoError(t, err)

	out, err := tool.Run(t.Context(), json.RawMessage(`{"url":"https://example.com"}`))
	require.NoError(t, err)

	var env envelope
	require.NoError(t, json.Unmarshal([]byte(out), &env))
	require.Equal(t, "error", env.Status)
	require.Equal(t, "fetch_failed: http 404", env.Message)
}

func TestFetch_RunRejectsInvalidArgs(t *testing.T) {
	tool, err := web.NewFetchWithFetcher(&fakeFetcher{})
	require.NoError(t, err)

	out, err := tool.Run(t.Context(), json.RawMessage(`not json`))
	require.NoError(t, err)

	var env envelope
	require.NoError(t, json.Unmarshal([]byte(out), &env))
	require.Equal(t, "error", env.Status)
	require.Contains(t, env.Message, "invalid_args:")
}

func TestFetchRunTracesFetchMetadata(t *testing.T) {
	recorder := newSpanRecorder(t)
	tool, err := web.NewFetchWithFetcher(&fakeFetcher{result: internalfetch.Result{
		URL:            "https://example.com",
		FinalURL:       "https://example.com/final",
		ContentType:    "markdown",
		Content:        "# Example",
		HTTPStatusCode: 200,
		ContentBytes:   321,
	}})
	require.NoError(t, err)

	out, err := tool.Run(t.Context(), json.RawMessage(`{"url":"https://example.com/path?q=1"}`))
	require.NoError(t, err)
	require.NotEmpty(t, out)

	span := findSpan(t, recorder.Ended(), "web.fetch")
	require.Equal(t, "example.com", requireSpanAttr(t, span, "url.host"))
	require.Equal(t, "markdown", requireSpanAttr(t, span, "web.content_type"))
	require.Equal(t, int64(200), requireSpanAttrInt(t, span, "http.status_code"))
	require.Equal(t, int64(321), requireSpanAttrInt(t, span, "web.content_bytes"))
}
