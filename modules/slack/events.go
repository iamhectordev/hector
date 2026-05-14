package slack

import (
	"time"

	"github.com/iamhectordev/hector/pkg/waffle"
)

// MessageReceived is emitted when a Slack direct message is received.
var MessageReceived = mustDefine[MessageReceivedData]("slack.message_received", 1)

// Origin identifies where a Slack message came from.
type Origin struct {
	ChannelID string
	ThreadTS  string // non-empty when the message is part of a thread
}

// MessageReceivedData is the payload for [MessageReceived].
type MessageReceivedData struct {
	Origin     Origin
	SenderID   string
	Text       string
	SentAt     time.Time
	ReceivedAt time.Time
}

func mustDefine[T any](eventType string, schemaVersion int) waffle.Definition[T] {
	def, err := waffle.Define[T](eventType, schemaVersion)
	if err != nil {
		panic(err)
	}
	return def
}
