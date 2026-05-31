package fetch_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iamhectordev/hector/internal/web/fetch"
	"github.com/iamhectordev/hector/pkg/safehttp"
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

func articleHTMLWithHead(extraHead string) string {
	return `<!doctype html>
<html><head>
<title>The Quick Brown Fox</title>` + extraHead + `
</head><body><article>
<h1>The Quick Brown Fox</h1>
<p>The quick brown fox jumps over the lazy dog. The quick brown fox jumps over the lazy dog. The quick brown fox jumps over the lazy dog.</p>
<p>Sphinx of black quartz, judge my vow. Sphinx of black quartz, judge my vow. Sphinx of black quartz, judge my vow.</p>
<p>Pack my box with five dozen liquor jugs. Pack my box with five dozen liquor jugs. Pack my box with five dozen liquor jugs.</p>
</article></body></html>`
}

func safeClient(t *testing.T, opts ...safehttp.Option) *http.Client {
	t.Helper()
	opts = append([]safehttp.Option{safehttp.WithAllowLoopback()}, opts...)
	client, err := safehttp.Client(opts...)
	require.NoError(t, err)
	return client
}

func newServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

func newFetcher(t *testing.T, client *http.Client) *fetch.Fetcher {
	t.Helper()
	f, err := fetch.NewFetcher(client)
	require.NoError(t, err)
	return f
}

// fakeResolver implements safehttp.Resolver for injecting controlled DNS responses.
type fakeResolver struct {
	ip string
}

func (r *fakeResolver) LookupHost(_ context.Context, _ string) ([]string, error) {
	return []string{r.ip}, nil
}

func TestNewFetcher_NilClient(t *testing.T) {
	_, err := fetch.NewFetcher(nil)
	require.Error(t, err)
}

func TestFetcher_HappyPath(t *testing.T) {
	srv := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(articleHTML))
	})

	result, err := newFetcher(t, safeClient(t)).Fetch(t.Context(), srv.URL)
	require.NoError(t, err)
	require.Equal(t, srv.URL, result.URL)
	require.Equal(t, srv.URL, result.FinalURL)
	require.Equal(t, "The Quick Brown Fox", result.Title)
	require.Equal(t, "markdown", result.ContentType)
	require.Contains(t, result.Content, "quick brown fox")
}

func TestFetcher_UsesMarkdownAlternate(t *testing.T) {
	const alternateContent = "# Alternate article\n\nThis came from markdown.\n"

	mux := http.NewServeMux()
	mux.HandleFunc("/article", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html>
<html><head>
<title>HTML Article</title>
<link rel="alternate" type="text/markdown" href="/article.md">
</head><body><article><p>This came from HTML.</p></article></body></html>`))
	})
	mux.HandleFunc("/article.md", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		_, _ = w.Write([]byte(alternateContent))
	})
	srv := newServer(t, mux.ServeHTTP)

	result, err := newFetcher(t, safeClient(t)).Fetch(t.Context(), srv.URL+"/article")
	require.NoError(t, err)
	require.Equal(t, srv.URL+"/article", result.URL)
	require.Equal(t, srv.URL+"/article.md", result.FinalURL)
	require.Equal(t, "markdown", result.ContentType)
	require.Equal(t, alternateContent, result.Content)
}

func TestFetcher_UsesApplicationMarkdownAlternateCaseInsensitively(t *testing.T) {
	const alternateContent = "# Application markdown\n"

	mux := http.NewServeMux()
	mux.HandleFunc("/article", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html>
<html><head>
<title>HTML Article</title>
<link rel="canonical ALTERNATE" type="Application/Markdown" href="/article.md">
</head><body><article><p>This came from HTML.</p></article></body></html>`))
	})
	mux.HandleFunc("/article.md", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "Application/Markdown; charset=utf-8")
		_, _ = w.Write([]byte(alternateContent))
	})
	srv := newServer(t, mux.ServeHTTP)

	result, err := newFetcher(t, safeClient(t)).Fetch(t.Context(), srv.URL+"/article")
	require.NoError(t, err)
	require.Equal(t, srv.URL+"/article.md", result.FinalURL)
	require.Equal(t, "markdown", result.ContentType)
	require.Equal(t, alternateContent, result.Content)
}

func TestFetcher_FallsBackWhenMarkdownAlternateReturnsHTML(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/article", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(articleHTMLWithHead(`
<link rel="alternate" type="text/markdown" href="/article.md">`)))
	})
	mux.HandleFunc("/article.md", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<html><body><p>This is HTML.</p></body></html>"))
	})
	srv := newServer(t, mux.ServeHTTP)

	result, err := newFetcher(t, safeClient(t)).Fetch(t.Context(), srv.URL+"/article")
	require.NoError(t, err)
	require.Equal(t, srv.URL+"/article", result.FinalURL)
	require.Equal(t, "markdown", result.ContentType)
	require.Contains(t, result.Content, "quick brown fox")
	require.NotContains(t, result.Content, "This is HTML")
}

func TestFetcher_FallsBackWhenMarkdownAlternate404s(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/article", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(articleHTMLWithHead(`
<link rel="alternate" type="text/markdown" href="/missing.md">`)))
	})
	srv := newServer(t, mux.ServeHTTP)

	result, err := newFetcher(t, safeClient(t)).Fetch(t.Context(), srv.URL+"/article")
	require.NoError(t, err)
	require.Equal(t, srv.URL+"/article", result.FinalURL)
	require.Equal(t, "markdown", result.ContentType)
	require.Contains(t, result.Content, "quick brown fox")
}

func TestFetcher_FallsBackWhenMarkdownAlternateIsBlocked(t *testing.T) {
	srv := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(articleHTMLWithHead(`
<link rel="alternate" type="text/markdown" href="http://169.254.169.254/article.md">`)))
	})

	result, err := newFetcher(t, safeClient(t)).Fetch(t.Context(), srv.URL)
	require.NoError(t, err)
	require.Equal(t, srv.URL, result.FinalURL)
	require.Equal(t, "markdown", result.ContentType)
	require.Contains(t, result.Content, "quick brown fox")
}

func TestFetcher_FallsBackWhenMarkdownAlternatePointsBackToOriginal(t *testing.T) {
	var articleRequests atomic.Int32
	srv := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		articleRequests.Add(1)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(articleHTMLWithHead(`
<link rel="alternate" type="text/markdown" href="/article">`)))
	})

	result, err := newFetcher(t, safeClient(t)).Fetch(t.Context(), srv.URL+"/article")
	require.NoError(t, err)
	require.Equal(t, srv.URL+"/article", result.FinalURL)
	require.Equal(t, "markdown", result.ContentType)
	require.Contains(t, result.Content, "quick brown fox")
	require.EqualValues(t, 2, articleRequests.Load())
}

func TestFetcher_MarkdownPassThrough(t *testing.T) {
	const content = "# Release notes\n\n- Shipped raw markdown support.\n"
	srv := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		_, _ = w.Write([]byte(content))
	})

	result, err := newFetcher(t, safeClient(t)).Fetch(t.Context(), srv.URL)
	require.NoError(t, err)
	require.Equal(t, srv.URL, result.URL)
	require.Equal(t, srv.URL, result.FinalURL)
	require.Equal(t, "markdown", result.ContentType)
	require.Equal(t, content, result.Content)
}

func TestFetcher_PlainTextPassThrough(t *testing.T) {
	const content = "plain status report\nline two\n"
	srv := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte(content))
	})

	result, err := newFetcher(t, safeClient(t)).Fetch(t.Context(), srv.URL)
	require.NoError(t, err)
	require.Equal(t, srv.URL, result.URL)
	require.Equal(t, srv.URL, result.FinalURL)
	require.Equal(t, "text", result.ContentType)
	require.Equal(t, content, result.Content)
}

func TestFetcher_Redirect(t *testing.T) {
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

	result, err := newFetcher(t, safeClient(t)).Fetch(t.Context(), srv.URL+"/start")
	require.NoError(t, err)
	require.Equal(t, srv.URL+"/start", result.URL)
	require.Equal(t, articleURL, result.FinalURL)
}

func TestFetcher_NonHTML(t *testing.T) {
	srv := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write([]byte("%PDF-1.4\n..."))
	})

	_, err := newFetcher(t, safeClient(t)).Fetch(t.Context(), srv.URL)
	require.Error(t, err)
	require.True(t, strings.HasPrefix(err.Error(), "blocked_content_type:"),
		"got error: %q", err)
}

func TestFetcher_EmptyExtraction(t *testing.T) {
	srv := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html><body></body></html>"))
	})

	_, err := newFetcher(t, safeClient(t)).Fetch(t.Context(), srv.URL)
	require.Error(t, err)
	require.True(t, strings.HasPrefix(err.Error(), "extraction_failed:"),
		"got error: %q", err)
}

func TestFetcher_404(t *testing.T) {
	srv := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "nope", http.StatusNotFound)
	})

	_, err := newFetcher(t, safeClient(t)).Fetch(t.Context(), srv.URL)
	require.Error(t, err)
	require.True(t, strings.HasPrefix(err.Error(), "fetch_failed:"),
		"got error: %q", err)
	require.Contains(t, err.Error(), "404")
}

func TestFetcher_InvalidURL(t *testing.T) {
	_, err := newFetcher(t, safeClient(t)).Fetch(t.Context(), "://not-a-url")
	require.Error(t, err)
	require.True(t,
		strings.HasPrefix(err.Error(), "invalid_url:") ||
			strings.HasPrefix(err.Error(), "fetch_failed:"),
		"got error: %q", err)
}

func TestFetcher_BlockedAddress(t *testing.T) {
	r := &fakeResolver{ip: "10.0.0.1"}
	client, err := safehttp.Client(safehttp.WithResolver(r))
	require.NoError(t, err)

	_, err = newFetcher(t, client).Fetch(t.Context(), "http://internal.corp/article")
	require.Error(t, err)
	require.True(t, strings.HasPrefix(err.Error(), "blocked_address:"),
		"got error: %q", err)
}

func TestFetcher_BlockedScheme(t *testing.T) {
	_, err := newFetcher(t, safeClient(t)).Fetch(t.Context(), "ftp://example.com/resource")
	require.Error(t, err)
	require.True(t, strings.HasPrefix(err.Error(), "blocked_scheme:"),
		"got error: %q", err)
}

func TestFetcher_Oversize(t *testing.T) {
	const limit = 1024
	srv := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = io.Copy(w, io.LimitReader(strings.NewReader(strings.Repeat("x", limit+1)), limit+1))
	})

	client := safeClient(t, safehttp.WithMaxBodyBytes(limit))
	_, err := newFetcher(t, client).Fetch(t.Context(), srv.URL)
	require.Error(t, err)
	require.True(t, strings.HasPrefix(err.Error(), "oversize:"),
		"got error: %q", err)
}

func TestFetcher_MarkdownPassThroughOversize(t *testing.T) {
	const limit = 1024
	srv := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/markdown")
		_, _ = io.Copy(w, io.LimitReader(strings.NewReader(strings.Repeat("x", limit+1)), limit+1))
	})

	client := safeClient(t, safehttp.WithMaxBodyBytes(limit))
	_, err := newFetcher(t, client).Fetch(t.Context(), srv.URL)
	require.Error(t, err)
	require.True(t, strings.HasPrefix(err.Error(), "oversize:"),
		"got error: %q", err)
}

func TestFetcher_Timeout(t *testing.T) {
	srv := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
	})

	client := safeClient(t, safehttp.WithTimeout(10*time.Millisecond))
	_, err := newFetcher(t, client).Fetch(t.Context(), srv.URL)
	require.Error(t, err)
	require.True(t, strings.HasPrefix(err.Error(), "timeout:"),
		"got error: %q", err)
}
