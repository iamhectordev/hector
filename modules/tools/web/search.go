package web

import (
	"context"
	"errors"
	"fmt"
	"strings"

	internalsearch "github.com/iamhectordev/hector/internal/web/search"
	"github.com/iamhectordev/hector/modules/tools"
)

type Searcher interface {
	Search(ctx context.Context, query string) ([]internalsearch.Result, error)
}

type searchInput struct {
	Query string `json:"query" jsonschema:"the web search query"`
}

type searchPayload struct {
	Query   string                  `json:"query"`
	Results []internalsearch.Result `json:"results"`
}

func NewSearch(searcher Searcher) (tools.Tool, error) {
	if searcher == nil {
		return nil, errors.New("web: searcher is required")
	}
	return tools.New[searchInput, searchPayload](
		"web_search",
		"Searches the web through the configured provider. Returns source candidates with URL, title, snippet, provider, and score; does not fetch full pages or summarize results.",
		func(ctx context.Context, in searchInput) (searchPayload, error) {
			query := strings.TrimSpace(in.Query)
			if query == "" {
				return searchPayload{}, fmt.Errorf("invalid_query: query is required")
			}
			results, err := searcher.Search(ctx, query)
			if err != nil {
				return searchPayload{}, err
			}
			return searchPayload{Query: query, Results: results}, nil
		},
	)
}
