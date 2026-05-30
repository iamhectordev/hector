package search_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/iamhectordev/hector/internal/web/search"
	"github.com/stretchr/testify/require"
)

func TestConfigEnabled(t *testing.T) {
	t.Parallel()

	require.False(t, search.Config{}.Enabled())
	require.True(t, search.Config{Provider: search.ProviderTavily}.Enabled())
	require.False(t, search.Config{Tavily: search.TavilyConfig{APIKey: "test-key"}}.Enabled())
	require.False(t, search.Config{Tavily: search.TavilyConfig{APIURL: "https://example.com"}}.Enabled())
}

func TestNewTavilyValidatesConfig(t *testing.T) {
	t.Parallel()

	_, err := search.NewTavily(search.TavilyConfig{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "tavily: invalid config")

	_, err = search.NewTavily(search.TavilyConfig{
		APIKey: "test-key",
		APIURL: "not a url",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "tavily: invalid config")

	client, err := search.NewTavily(search.TavilyConfig{
		APIKey: "test-key",
		APIURL: "https://api.tavily.com",
	})
	require.NoError(t, err)
	require.NotNil(t, client)
}

func TestTavilyVerifyUsesBearerToken(t *testing.T) {
	t.Parallel()

	var authHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/usage", r.URL.Path)
		authHeader = r.Header.Get("Authorization")
		_, err := fmt.Fprint(w, `{"total_credits_used":1}`)
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	client, err := search.NewTavily(search.TavilyConfig{
		APIKey: "test-key",
		APIURL: server.URL,
	})
	require.NoError(t, err)

	require.NoError(t, client.Verify(t.Context()))
	require.Equal(t, "Bearer test-key", authHeader)
}

func TestTavilyVerifyReportsUnauthorizedWithoutLeakingAPIKey(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, err := fmt.Fprint(w, `{"error":"bad key test-key"}`)
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	client, err := search.NewTavily(search.TavilyConfig{
		APIKey: "test-key",
		APIURL: server.URL,
	})
	require.NoError(t, err)

	err = client.Verify(t.Context())
	require.Error(t, err)
	require.Contains(t, err.Error(), "tavily: verify: unauthorized")
	require.NotContains(t, err.Error(), "test-key")
}

func TestTavilyVerifyReportsRateLimited(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	t.Cleanup(server.Close)

	client, err := search.NewTavily(search.TavilyConfig{
		APIKey: "test-key",
		APIURL: server.URL,
	})
	require.NoError(t, err)

	err = client.Verify(t.Context())
	require.Error(t, err)
	require.Contains(t, err.Error(), "tavily: verify: rate limited")
}

func TestTavilyVerifyReportsUnexpectedStatusWithoutBody(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, err := fmt.Fprint(w, `{"error":"server saw test-key"}`)
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	client, err := search.NewTavily(search.TavilyConfig{
		APIKey: "test-key",
		APIURL: server.URL,
	})
	require.NoError(t, err)

	err = client.Verify(t.Context())
	require.Error(t, err)
	require.Contains(t, err.Error(), "tavily: verify: unexpected status 500")
	require.NotContains(t, err.Error(), "test-key")
	require.NotContains(t, err.Error(), "server saw")
}
