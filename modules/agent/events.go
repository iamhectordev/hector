package agent

import "github.com/iamhectordev/hector/pkg/waffle"

// TurnEnd is emitted after the agent successfully completes a turn.
var TurnEnd = mustDefine[TurnEndData]("agent.turn_end", 1)

// TurnEndData is the payload for TurnEnd.
type TurnEndData struct {
	SessionID  string
	SourceURI  string
	TurnOffset int // index into session history where this turn's new messages start
}

func mustDefine[T any](eventType string, schemaVersion int) waffle.Definition[T] {
	def, err := waffle.Define[T](eventType, schemaVersion)
	if err != nil {
		panic(err)
	}
	return def
}
