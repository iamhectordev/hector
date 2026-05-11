package tools

import "github.com/iamhectordev/hector/pkg/waffle"

// CallRequested is emitted when a tool should be executed.
var CallRequested = mustDefine[CallRequestedData]("tool.call_requested", 1)

// CallCompleted is emitted when tool execution finishes.
var CallCompleted = mustDefine[CallCompletedData]("tool.call_completed", 1)

// CallRequestedData is the payload for [CallRequested].
type CallRequestedData struct {
	CallID string
	Name   string
	Args   string // JSON
}

// CallCompletedData is the payload for [CallCompleted].
type CallCompletedData struct {
	CallID string
	Output string
	Error  string
}

func mustDefine[T any](eventType string, schemaVersion int) waffle.Definition[T] {
	def, err := waffle.Define[T](eventType, schemaVersion)
	if err != nil {
		panic(err)
	}
	return def
}
