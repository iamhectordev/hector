package schema

import "encoding/json"

// ToolCall is a model-requested executable capability invocation.
type ToolCall struct {
	ID        string
	Name      string
	Arguments json.RawMessage
}

// ToolDefinition describes a tool that can be exposed to the model.
type ToolDefinition struct {
	Name        string
	Description string
	Parameters  json.RawMessage
}
