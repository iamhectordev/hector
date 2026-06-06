package llm

import (
	"github.com/iamhectordev/hector/pkg/llm/schema"
	"github.com/iamhectordev/hector/pkg/telem"
)

const SpanComplete = "llm.complete"

type telemetryFields interface {
	TelemetryFields() []telem.Field
}

func CompleteFields(completer Completer, req schema.CompletionRequest) []telem.Field {
	fields := []telem.Field{
		telem.Int("llm.message_count", len(req.Messages)),
		telem.Int("llm.tool_count", len(req.Tools)),
	}
	if provider, ok := completer.(telemetryFields); ok {
		fields = append(fields, provider.TelemetryFields()...)
	}
	return fields
}
