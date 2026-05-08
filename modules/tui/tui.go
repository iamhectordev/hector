package tui

import "github.com/iamhectordev/hector/pkg/waffle"

// MessageReceived is emitted when the user submits a line from the TUI input loop.
var MessageReceived = mustDefine[MessageReceivedData]("tui.message_received", 1)

// MessageReceivedData is the payload for [MessageReceived].
type MessageReceivedData struct {
	Text string
}

func mustDefine[T any](eventType string, schemaVersion int) waffle.Definition[T] {
	def, err := waffle.Define[T](eventType, schemaVersion)
	if err != nil {
		panic(err)
	}
	return def
}
