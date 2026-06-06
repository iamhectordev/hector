package slack

import (
	"net/url"

	"github.com/iamhectordev/hector/pkg/telem"
)

const spanReplySend = "slack.reply.send"

func replyFields(uri *url.URL) []telem.Field {
	channelID, threadTS, err := ParseOriginURI(uri)
	if err != nil {
		return []telem.Field{telem.String("surface.name", "slack")}
	}
	fields := []telem.Field{
		telem.String("surface.name", "slack"),
		telem.String("slack.channel_id", channelID),
	}
	if threadTS != "" {
		fields = append(fields, telem.String("slack.thread_ts", threadTS))
	}
	return fields
}
