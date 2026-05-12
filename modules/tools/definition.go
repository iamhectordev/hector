package tools

import (
	"context"
	"encoding/json"
)

// Definition describes a tool that can be exposed to agents.
type Definition struct {
	Name        string
	Description string
	Parameters  json.RawMessage
}

// Tool is an executable capability with a JSON input schema.
type Tool interface {
	Definition() Definition
	Run(ctx context.Context, args json.RawMessage) (string, error)
}
