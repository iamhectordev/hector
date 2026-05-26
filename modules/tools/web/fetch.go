package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"strings"

	readability "codeberg.org/readeck/go-readability"
	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"

	"github.com/iamhectordev/hector/modules/tools"
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
		Description: "Fetches a web page and returns the readable article as markdown. Use for articles, blog posts, and documentation pages. Returns the page title, the final URL after redirects, and the extracted content. Returns an error if the page is not HTML or no article can be extracted.",
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
		return tools.Fail("fetch_failed: " + err.Error())
	}
	if resp == nil {
		return tools.Fail("fetch_failed: no response")
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return tools.Fail(fmt.Sprintf("fetch_failed: http %d", resp.StatusCode))
	}

	mediaType, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if mediaType != "text/html" && mediaType != "application/xhtml+xml" {
		return tools.Fail("blocked_content_type: " + mediaType)
	}

	article, err := readability.FromReader(resp.Body, resp.Request.URL)
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
