package structured

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/iamhectordev/hector/pkg/llm"
	"github.com/iamhectordev/hector/pkg/llm/schema"
)

const toolName = "produce_result"
const toolDesc = "Call this tool to produce the result required by the system prompt."

// Extractor forces a model to call a single tool and unmarshals the arguments into T.
// The schema for T is computed once at construction time.
type Extractor[T any] struct {
	completer llm.Completer
	system    string
	tool      schema.ToolDefinition
}

// NewExtractor creates an Extractor that extracts T from model responses.
// system is the prompt that tells the model what to extract.
func NewExtractor[T any](completer llm.Completer, system string) (*Extractor[T], error) {
	params, err := schemaFor[T]()
	if err != nil {
		return nil, fmt.Errorf("structured: schema for output type: %w", err)
	}
	return &Extractor[T]{
		completer: completer,
		system:    system,
		tool: schema.ToolDefinition{
			Name:        toolName,
			Description: toolDesc,
			Parameters:  params,
		},
	}, nil
}

// Extract sends messages to the model, forces a tool call, and returns the unmarshalled result.
func (e *Extractor[T]) Extract(ctx context.Context, messages []*schema.Message) (T, error) {
	var zero T
	req := schema.CompletionRequest{
		System:     e.system,
		Messages:   messages,
		Tools:      []schema.ToolDefinition{e.tool},
		ToolChoice: &schema.ToolChoice{Name: toolName},
	}
	reply, err := e.completer.Complete(ctx, req)
	if err != nil {
		return zero, err
	}
	if reply == nil || len(reply.ToolCalls) == 0 {
		return zero, fmt.Errorf("structured: model did not call %q", toolName)
	}
	var out T
	if err := json.Unmarshal(reply.ToolCalls[0].Arguments, &out); err != nil {
		return zero, fmt.Errorf("structured: unmarshal result: %w", err)
	}
	return out, nil
}

func schemaFor[T any]() (json.RawMessage, error) {
	s, err := jsonschema.For[T](nil)
	if err != nil {
		return nil, err
	}
	b, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	return normalizeSchema(b)
}

func normalizeSchema(s json.RawMessage) (json.RawMessage, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(s, &m); err != nil {
		return s, nil
	}
	typeVal, ok := m["type"]
	if !ok {
		return s, nil
	}
	var typStr string
	if err := json.Unmarshal(typeVal, &typStr); err != nil || typStr != "object" {
		return s, nil
	}
	if _, hasProps := m["properties"]; !hasProps {
		m["properties"] = json.RawMessage(`{}`)
	}
	return json.Marshal(m)
}

