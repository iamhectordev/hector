package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	internalfetch "github.com/iamhectordev/hector/internal/web/fetch"
	"github.com/iamhectordev/hector/modules/tools"
	"github.com/iamhectordev/hector/pkg/telem"
)

// Fetcher is the interface for core fetch logic.
type Fetcher interface {
	Fetch(ctx context.Context, url string) (internalfetch.Result, error)
}

type fetchInput struct {
	URL string `json:"url" jsonschema:"the URL of the article to fetch"`
}

type Fetch struct {
	fetcher Fetcher
	schema  json.RawMessage
}

func NewFetch(client *http.Client) (*Fetch, error) {
	f, err := internalfetch.NewFetcher(client)
	if err != nil {
		return nil, err
	}
	return newFetch(f)
}

// NewFetchWithFetcher creates a Fetch tool with an injected fetcher, useful for testing.
func NewFetchWithFetcher(f Fetcher) (*Fetch, error) {
	if f == nil {
		return nil, errors.New("web: fetcher is required")
	}
	return newFetch(f)
}

func newFetch(f Fetcher) (*Fetch, error) {
	schema, err := tools.SchemaFor[fetchInput]()
	if err != nil {
		return nil, err
	}
	return &Fetch{fetcher: f, schema: schema}, nil
}

func (f *Fetch) Definition() tools.Definition {
	return tools.Definition{
		Name:        "web_fetch",
		Description: "Fetches a web page or text document. Returns readable HTML as markdown, raw markdown as markdown, and plain text as text. Returns the final URL after redirects.",
		Parameters:  f.schema,
	}
}

func (f *Fetch) Run(ctx context.Context, args json.RawMessage) (string, error) {
	var err error
	var in fetchInput
	if err = json.Unmarshal(args, &in); err != nil {
		err = errors.New("invalid_args: " + err.Error())
		return tools.Fail("invalid_args: " + err.Error())
	}
	ctx, span := telem.Trace(ctx, spanFetch)
	defer span.End(&err)
	result, err := f.fetcher.Fetch(ctx, in.URL)
	if err != nil {
		return tools.Fail(err.Error())
	}
	span.AddFields(fetchFields(in.URL, result)...)
	return tools.OK(result)
}
