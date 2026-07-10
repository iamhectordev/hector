package slack

import (
	"net/url"

	"github.com/iamhectordev/hector/pkg/telem"
	"github.com/slack-go/slack/slackevents"
)

const spanReplySend = "slack.reply.send"
const spanMessageReceive = "slack.message.receive"

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

func messageFields(e *slackevents.MessageEvent) []telem.Field {
	fields := []telem.Field{
		telem.String("surface.name", "slack"),
		telem.String("slack.channel_id", e.Channel),
		telem.String("slack.channel_type", string(e.ChannelType)),
		telem.String("slack.message_ts", e.TimeStamp),
	}
	if e.ThreadTimeStamp != "" {
		fields = append(fields, telem.String("slack.thread_ts", e.ThreadTimeStamp))
	}
	if e.User != "" {
		fields = append(fields, telem.String("slack.user_id", e.User))
	}
	if e.SubType != "" {
		fields = append(fields, telem.String("slack.message_subtype", e.SubType))
	}
	return fields
}

func receivedBaggage(data MessageReceivedData) []telem.Field {
	return []telem.Field{
		telem.String("surface.name", "slack"),
		telem.String("session.source_uri", NewOriginURI(data.Channel.ID, data.ThreadTS)),
	}
}
