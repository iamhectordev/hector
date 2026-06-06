package agent

import (
	"github.com/iamhectordev/hector/pkg/llm/schema"
	"github.com/iamhectordev/hector/pkg/session"
	"github.com/iamhectordev/hector/pkg/telem"
)

const (
	spanTurnRun  = "agent.turn.run"
	spanToolCall = "tool.call"
)

func turnFields(messages []*schema.Message, tools ToolRuntime) []telem.Field {
	fields := []telem.Field{
		telem.Int("agent.input_message_count", len(messages)),
	}
	if tools != nil {
		fields = append(fields, telem.Int("agent.tool_count", len(tools.Definitions())))
	}
	return fields
}

func sessionFields(s session.Session) []telem.Field {
	fields := []telem.Field{}
	if s.ID != "" {
		fields = append(fields, telem.String("session.id", s.ID))
	}
	if s.SourceURI != "" {
		fields = append(fields, telem.String("session.source_uri", s.SourceURI))
	}
	return fields
}

func toolCallFields(call schema.ToolCall) []telem.Field {
	return []telem.Field{
		telem.String("tool.name", call.Name),
		telem.String("tool.call_id", call.ID),
	}
}
