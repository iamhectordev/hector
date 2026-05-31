package fetch

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"

	readability "codeberg.org/readeck/go-readability"
	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"golang.org/x/net/html"

	"github.com/iamhectordev/hector/pkg/safehttp"
)

type Result struct {
	URL         string `json:"url"`
	FinalURL    string `json:"final_url"`
	Title       string `json:"title"`
	ContentType string `json:"content_type"`
	Content     string `json:"content"`
}

type contentKind string

const (
	contentKindHTML     contentKind = "html"
	contentKindMarkdown contentKind = "markdown"
	contentKindText     contentKind = "text"
	contentKindBlocked  contentKind = "blocked"
)

type Fetcher struct {
	http *http.Client
}

func NewFetcher(client *http.Client) (*Fetcher, error) {
	if client == nil {
		return nil, errors.New("fetch: http client is required")
	}
	return &Fetcher{http: client}, nil
}

func (f *Fetcher) Fetch(ctx context.Context, rawURL string) (Result, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return Result{}, fmt.Errorf("invalid_url: %w", err)
	}

	resp, err := f.http.Do(req)
	if err != nil {
		return Result{}, errors.New(classifyRequestErr(err))
	}
	if resp == nil {
		return Result{}, errors.New("fetch_failed: no response")
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return Result{}, fmt.Errorf("fetch_failed: http %d", resp.StatusCode)
	}

	mediaType, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	kind := classifyContent(mediaType)
	if kind == contentKindBlocked {
		return Result{}, fmt.Errorf("blocked_content_type: %s", mediaType)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		if errors.Is(err, safehttp.ErrOversize) {
			return Result{}, errors.New("oversize: response body exceeds limit")
		}
		return Result{}, fmt.Errorf("fetch_failed: %w", err)
	}

	if kind == contentKindMarkdown {
		return Result{
			URL:         rawURL,
			FinalURL:    resp.Request.URL.String(),
			ContentType: "markdown",
			Content:     string(body),
		}, nil
	}

	if kind == contentKindText {
		return Result{
			URL:         rawURL,
			FinalURL:    resp.Request.URL.String(),
			ContentType: "text",
			Content:     string(body),
		}, nil
	}

	if alternateURL, ok := findMarkdownAlternate(body, resp.Request.URL); ok {
		if result, ok := f.fetchMarkdownAlternate(ctx, rawURL, alternateURL); ok {
			return result, nil
		}
	}

	article, err := readability.FromReader(bytes.NewReader(body), resp.Request.URL)
	if err != nil {
		return Result{}, fmt.Errorf("extraction_failed: %w", err)
	}
	if article.Length == 0 || strings.TrimSpace(article.Content) == "" {
		return Result{}, errors.New("extraction_failed: no content")
	}

	md, err := htmltomarkdown.ConvertString(article.Content)
	if err != nil {
		return Result{}, fmt.Errorf("extraction_failed: %w", err)
	}

	return Result{
		URL:         rawURL,
		FinalURL:    resp.Request.URL.String(),
		Title:       article.Title,
		ContentType: "markdown",
		Content:     md,
	}, nil
}

func (f *Fetcher) fetchMarkdownAlternate(ctx context.Context, originalURL string, alternateURL *url.URL) (Result, bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, alternateURL.String(), nil)
	if err != nil {
		return Result{}, false
	}

	resp, err := f.http.Do(req)
	if err != nil || resp == nil {
		return Result{}, false
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return Result{}, false
	}

	mediaType, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if classifyContent(mediaType) != contentKindMarkdown {
		return Result{}, false
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Result{}, false
	}

	return Result{
		URL:         originalURL,
		FinalURL:    resp.Request.URL.String(),
		ContentType: "markdown",
		Content:     string(body),
	}, true
}

func findMarkdownAlternate(body []byte, baseURL *url.URL) (*url.URL, bool) {
	tokenizer := html.NewTokenizer(bytes.NewReader(body))
	inHead := false

	for {
		switch tokenizer.Next() {
		case html.ErrorToken:
			return nil, false
		case html.StartTagToken, html.SelfClosingTagToken:
			token := tokenizer.Token()
			if strings.EqualFold(token.Data, "head") {
				inHead = true
				continue
			}
			if !inHead || !strings.EqualFold(token.Data, "link") {
				continue
			}
			if alternateURL, ok := markdownAlternateFromLink(token, baseURL); ok {
				return alternateURL, true
			}
		case html.EndTagToken:
			token := tokenizer.Token()
			if strings.EqualFold(token.Data, "head") {
				return nil, false
			}
		}
	}
}

func markdownAlternateFromLink(token html.Token, baseURL *url.URL) (*url.URL, bool) {
	var rel, mediaType, href string
	for _, attr := range token.Attr {
		switch strings.ToLower(attr.Key) {
		case "rel":
			rel = attr.Val
		case "type":
			mediaType = attr.Val
		case "href":
			href = attr.Val
		}
	}
	if href == "" || !isMarkdownAlternate(rel, mediaType) {
		return nil, false
	}
	alternateURL, err := baseURL.Parse(href)
	if err != nil {
		return nil, false
	}
	return alternateURL, true
}

func isMarkdownAlternate(rel string, mediaType string) bool {
	if !hasRelToken(rel, "alternate") {
		return false
	}
	parsedType, _, err := mime.ParseMediaType(mediaType)
	if err != nil {
		return false
	}
	return classifyContent(strings.ToLower(parsedType)) == contentKindMarkdown
}

func hasRelToken(rel string, token string) bool {
	for _, field := range strings.Fields(rel) {
		if strings.EqualFold(field, token) {
			return true
		}
	}
	return false
}

func classifyContent(mediaType string) contentKind {
	switch strings.ToLower(mediaType) {
	case "text/html", "application/xhtml+xml":
		return contentKindHTML
	case "text/markdown", "application/markdown":
		return contentKindMarkdown
	case "text/plain":
		return contentKindText
	default:
		return contentKindBlocked
	}
}

// classifyRequestErr maps safehttp sentinel errors to structured error codes.
func classifyRequestErr(err error) string {
	switch {
	case errors.Is(err, safehttp.ErrBlockedScheme):
		return "blocked_scheme: " + err.Error()
	case errors.Is(err, safehttp.ErrBlockedAddress):
		return "blocked_address: " + err.Error()
	case isTimeout(err):
		return "timeout: request timed out"
	default:
		return "fetch_failed: " + err.Error()
	}
}

func isTimeout(err error) bool {
	var netErr interface{ Timeout() bool }
	if errors.As(err, &netErr) {
		return netErr.Timeout()
	}
	return errors.Is(err, context.DeadlineExceeded)
}
