package web_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	internalsearch "github.com/iamhectordev/hector/internal/web/search"
	"github.com/iamhectordev/hector/modules/tools"
	"github.com/iamhectordev/hector/modules/tools/web"
)

type fakeSearcher struct {
	results []internalsearch.Result
	err     error
	query   string
	calls   int
}

func (f *fakeSearcher) Search(_ context.Context, query string) ([]internalsearch.Result, error) {
	f.calls++
	f.query = query
	return f.results, f.err
}

type searchResultPayload struct {
	Query   string                  `json:"query"`
	Results []internalsearch.Result `json:"results"`
}

func TestSearchRunReturnsResults(t *testing.T) {
	score := 0.91
	searcher := &fakeSearcher{results: []internalsearch.Result{{
		Provider: internalsearch.ProviderTavily,
		URL:      "https://example.com/article",
		Title:    "Example Article",
		Snippet:  "A useful result snippet.",
		Score:    &score,
	}}}
	tool, err := web.NewSearch(searcher)
	require.NoError(t, err)

	out, err := tool.Run(t.Context(), json.RawMessage(`{"query":"example query"}`))
	require.NoError(t, err)

	var env envelope
	require.NoError(t, json.Unmarshal([]byte(out), &env))
	require.Equal(t, "ok", env.Status, "message=%s", env.Message)
	require.Equal(t, "example query", searcher.query)

	var payload searchResultPayload
	require.NoError(t, json.Unmarshal(env.Result, &payload))
	require.Equal(t, "example query", payload.Query)
	require.Equal(t, searcher.results, payload.Results)
}

func TestSearchRunRejectsBlankQuery(t *testing.T) {
	searcher := &fakeSearcher{}
	tool, err := web.NewSearch(searcher)
	require.NoError(t, err)

	out, err := tool.Run(t.Context(), json.RawMessage(`{"query":"   "}`))
	require.NoError(t, err)

	var env envelope
	require.NoError(t, json.Unmarshal([]byte(out), &env))
	require.Equal(t, "error", env.Status)
	require.Equal(t, "invalid_query: query is required", env.Message)
	require.Equal(t, 0, searcher.calls)
}

func TestSearchRunReturnsErrorEnvelopeOnClientFailure(t *testing.T) {
	tool, err := web.NewSearch(&fakeSearcher{err: errors.New("tavily: search: rate limited")})
	require.NoError(t, err)

	out, err := tool.Run(t.Context(), json.RawMessage(`{"query":"example query"}`))
	require.NoError(t, err)

	var env envelope
	require.NoError(t, json.Unmarshal([]byte(out), &env))
	require.Equal(t, "error", env.Status)
	require.Equal(t, "tavily: search: rate limited", env.Message)
}

func TestSearchRunSanitizesTypedProviderErrors(t *testing.T) {
	tool, err := web.NewSearch(&fakeSearcher{err: &internalsearch.Error{
		Provider:  internalsearch.ProviderTavily,
		Operation: "search",
		Kind:      internalsearch.ErrorUnauthorized,
		Cause:     errors.New("provider body included test-key"),
	}})
	require.NoError(t, err)

	out, err := tool.Run(t.Context(), json.RawMessage(`{"query":"example query"}`))
	require.NoError(t, err)

	var env envelope
	require.NoError(t, json.Unmarshal([]byte(out), &env))
	require.Equal(t, "error", env.Status)
	require.Equal(t, "tavily: search: unauthorized", env.Message)
	require.NotContains(t, env.Message, "test-key")
	require.NotContains(t, out, "provider body")
}

func TestSearchDefinitionRegistersInRegistry(t *testing.T) {
	tool, err := web.NewSearch(&fakeSearcher{})
	require.NoError(t, err)

	def := tool.Definition()
	require.Equal(t, "web_search", def.Name)
	require.NotEmpty(t, def.Description)
	require.NotEmpty(t, def.Parameters)

	registry, err := tools.NewRegistry(tool)
	require.NoError(t, err)
	defs := registry.Definitions()
	require.Len(t, defs, 1)
	require.Equal(t, "web_search", defs[0].Name)
}

func TestSearchRunsThroughRegistry(t *testing.T) {
	tool, err := web.NewSearch(&fakeSearcher{results: []internalsearch.Result{{
		Provider: internalsearch.ProviderTavily,
		URL:      "https://example.com/result",
	}}})
	require.NoError(t, err)
	registry, err := tools.NewRegistry(tool)
	require.NoError(t, err)

	out, err := registry.Run(t.Context(), "web_search", json.RawMessage(`{"query":"registered search"}`))
	require.NoError(t, err)

	var env envelope
	require.NoError(t, json.Unmarshal([]byte(out), &env))
	require.Equal(t, "ok", env.Status, "message=%s", env.Message)
}
