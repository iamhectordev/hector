package comms

import (
	"net/url"

	"github.com/iamhectordev/hector/pkg/session"
	"github.com/iamhectordev/hector/pkg/telem"
)

const spanReplyRoute = "tool.reply.route"

func replyRouteFields(sourceURI string, uri *url.URL) []telem.Field {
	fields := []telem.Field{}
	if uri != nil && uri.Scheme != "" {
		fields = append(fields, telem.String("surface.name", uri.Scheme))
	} else if parsed, err := session.ParseSourceURI(sourceURI); err == nil {
		fields = append(fields, telem.String("surface.name", parsed.Scheme))
	}
	return fields
}
