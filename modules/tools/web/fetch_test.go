package web_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iamhectordev/hector/modules/tools/web"
)

const articleHTML = `<!doctype html>
<html><head><title>The Quick Brown Fox</title></head>
<body>
<header><nav>nav links</nav></header>
<article>
<h1>The Quick Brown Fox</h1>
<p>The quick brown fox jumps over the lazy dog. The quick brown fox jumps over the lazy dog. The quick brown fox jumps over the lazy dog.</p>
<p>Sphinx of black quartz, judge my vow. Sphinx of black quartz, judge my vow. Sphinx of black quartz, judge my vow.</p>
<p>Pack my box with five dozen liquor jugs. Pack my box with five dozen liquor jugs. Pack my box with five dozen liquor jugs.</p>
</article>
<footer>footer</footer>
</body></html>`

type envelope struct {
	Status  string          `json:"status"`
	Result  json.RawMessage `json:"result,omitempty"`
	Message string          `json:"message,omitempty"`
}

type payload struct {
	URL         string `json:"url"`
	FinalURL    string `json:"final_url"`
	Title       string `json:"title"`
	ContentType string `json:"content_type"`
	Content     string `json:"content"`
}

func newServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

func runFetch(t *testing.T, url string) envelope {
	t.Helper()
	tool, err := web.NewFetch(http.DefaultClient)
	require.NoError(t, err)
	args, err := json.Marshal(map[string]string{"url": url})
	require.NoError(t, err)
	out, err := tool.Run(t.Context(), args)
	require.NoError(t, err)
	var env envelope
	require.NoError(t, json.Unmarshal([]byte(out), &env))
	return env
}

func TestFetch_Definition(t *testing.T) {
	tool, err := web.NewFetch(http.DefaultClient)
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

func TestFetch_HappyPath(t *testing.T) {
	srv := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(articleHTML))
	})

	env := runFetch(t, srv.URL)
	require.Equal(t, "ok", env.Status, "message=%s", env.Message)

	var p payload
	require.NoError(t, json.Unmarshal(env.Result, &p))
	require.Equal(t, srv.URL, p.URL)
	require.Equal(t, srv.URL, p.FinalURL)
	require.Equal(t, "The Quick Brown Fox", p.Title)
	require.Equal(t, "markdown", p.ContentType)
	require.Contains(t, p.Content, "quick brown fox")
}

func TestFetch_Redirect(t *testing.T) {
	var articleURL string
	mux := http.NewServeMux()
	mux.HandleFunc("/article", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(articleHTML))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, articleURL, http.StatusFound)
	})
	srv := newServer(t, mux.ServeHTTP)
	articleURL = srv.URL + "/article"

	env := runFetch(t, srv.URL+"/start")
	require.Equal(t, "ok", env.Status, "message=%s", env.Message)

	var p payload
	require.NoError(t, json.Unmarshal(env.Result, &p))
	require.Equal(t, srv.URL+"/start", p.URL)
	require.Equal(t, articleURL, p.FinalURL)
}

func TestFetch_NonHTML(t *testing.T) {
	srv := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write([]byte("%PDF-1.4\n..."))
	})

	env := runFetch(t, srv.URL)
	require.Equal(t, "error", env.Status)
	require.True(t, strings.HasPrefix(env.Message, "blocked_content_type:"),
		"got message: %q", env.Message)
}

func TestFetch_EmptyExtraction(t *testing.T) {
	srv := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html><body></body></html>"))
	})

	env := runFetch(t, srv.URL)
	require.Equal(t, "error", env.Status)
	require.True(t, strings.HasPrefix(env.Message, "extraction_failed:"),
		"got message: %q", env.Message)
}

func TestFetch_404(t *testing.T) {
	srv := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "nope", http.StatusNotFound)
	})

	env := runFetch(t, srv.URL)
	require.Equal(t, "error", env.Status)
	require.True(t, strings.HasPrefix(env.Message, "fetch_failed:"),
		"got message: %q", env.Message)
	require.Contains(t, env.Message, "404")
}

func TestFetch_InvalidURL(t *testing.T) {
	env := runFetch(t, "://not-a-url")
	require.Equal(t, "error", env.Status)
	require.True(t,
		strings.HasPrefix(env.Message, "invalid_url:") ||
			strings.HasPrefix(env.Message, "fetch_failed:"),
		"got message: %q", env.Message)
}
