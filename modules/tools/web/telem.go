package web

import (
	"net/url"

	internalfetch "github.com/iamhectordev/hector/internal/web/fetch"
	internalsearch "github.com/iamhectordev/hector/internal/web/search"
	"github.com/iamhectordev/hector/pkg/telem"
)

const (
	spanSearch = "web.search"
	spanFetch  = "web.fetch"
)

func searchFields(provider internalsearch.ProviderName, query string, resultCount int) []telem.Field {
	fields := []telem.Field{
		telem.Int("web.query_length", len(query)),
		telem.Int("web.result_count", resultCount),
	}
	if provider != "" {
		fields = append(fields, telem.String("web.provider", string(provider)))
	}
	return fields
}

func fetchFields(rawURL string, result internalfetch.Result) []telem.Field {
	contentBytes := result.ContentBytes
	if contentBytes == 0 {
		contentBytes = len(result.Content)
	}
	fields := []telem.Field{telem.Int("web.content_bytes", contentBytes)}
	if result.ContentType != "" {
		fields = append(fields, telem.String("web.content_type", result.ContentType))
	}
	if result.HTTPStatusCode > 0 {
		fields = append(fields, telem.Int("http.status_code", result.HTTPStatusCode))
	}
	if u, err := url.Parse(rawURL); err == nil && u.Host != "" {
		fields = append(fields, telem.String("url.host", u.Host))
	}
	return fields
}
