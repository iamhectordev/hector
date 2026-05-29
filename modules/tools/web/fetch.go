package web

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"

	readability "codeberg.org/readeck/go-readability"
	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"

	"github.com/iamhectordev/hector/modules/tools"
	"github.com/iamhectordev/hector/pkg/safehttp"
)

type fetchInput struct {
	URL string `json:"url" jsonschema:"the URL of the article to fetch"`
}

type fetchPayload struct {
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

type Fetch struct {
	http   *http.Client
	schema json.RawMessage
}

func NewFetch(client *http.Client) (*Fetch, error) {
	if client == nil {
		return nil, errors.New("web: http client is required")
	}
	schema, err := tools.SchemaFor[fetchInput]()
	if err != nil {
		return nil, err
	}
	return &Fetch{http: client, schema: schema}, nil
}

func (f *Fetch) Definition() tools.Definition {
	return tools.Definition{
		Name:        "web_fetch",
		Description: "Fetches a web page or text document. Returns readable HTML as markdown, raw markdown as markdown, and plain text as text. Returns the final URL after redirects.",
		Parameters:  f.schema,
	}
}

func (f *Fetch) Run(ctx context.Context, args json.RawMessage) (string, error) {
	var in fetchInput
	if err := json.Unmarshal(args, &in); err != nil {
		return tools.Fail("invalid_args: " + err.Error())
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, in.URL, nil)
	if err != nil {
		return tools.Fail("invalid_url: " + err.Error())
	}

	resp, err := f.http.Do(req)
	if err != nil {
		return tools.Fail(classifyRequestErr(err))
	}
	if resp == nil {
		return tools.Fail("fetch_failed: no response")
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return tools.Fail(fmt.Sprintf("fetch_failed: http %d", resp.StatusCode))
	}

	mediaType, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	kind := classifyContent(mediaType)
	if kind == contentKindBlocked {
		return tools.Fail("blocked_content_type: " + mediaType)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		if errors.Is(err, safehttp.ErrOversize) {
			return tools.Fail("oversize: response body exceeds limit")
		}
		return tools.Fail("fetch_failed: " + err.Error())
	}

	if kind == contentKindMarkdown {
		return tools.OK(fetchPayload{
			URL:         in.URL,
			FinalURL:    resp.Request.URL.String(),
			ContentType: "markdown",
			Content:     string(body),
		})
	}

	if kind == contentKindText {
		return tools.OK(fetchPayload{
			URL:         in.URL,
			FinalURL:    resp.Request.URL.String(),
			ContentType: "text",
			Content:     string(body),
		})
	}

	article, err := readability.FromReader(bytes.NewReader(body), resp.Request.URL)
	if err != nil {
		return tools.Fail("extraction_failed: " + err.Error())
	}
	if article.Length == 0 || strings.TrimSpace(article.Content) == "" {
		return tools.Fail("extraction_failed: no content")
	}

	md, err := htmltomarkdown.ConvertString(article.Content)
	if err != nil {
		return tools.Fail("extraction_failed: " + err.Error())
	}

	return tools.OK(fetchPayload{
		URL:         in.URL,
		FinalURL:    resp.Request.URL.String(),
		Title:       article.Title,
		ContentType: "markdown",
		Content:     md,
	})
}

func classifyContent(mediaType string) contentKind {
	switch mediaType {
	case "text/html", "application/xhtml+xml":
		return contentKindHTML
	case "text/markdown":
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
