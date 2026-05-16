package tools

import (
	"context"
	"encoding/json"

	"github.com/google/jsonschema-go/jsonschema"
)

// TypedTool[I, O] implements Tool with auto-inferred schema from I and enveloped output.
type TypedTool[I, O any] struct {
	name        string
	description string
	schema      json.RawMessage
	fn          func(context.Context, I) (O, error)
}

// New constructs a TypedTool, inferring the JSON schema from I.
func New[I, O any](name, description string, fn func(context.Context, I) (O, error)) (*TypedTool[I, O], error) {
	schema, err := SchemaFor[I]()
	if err != nil {
		return nil, err
	}
	return &TypedTool[I, O]{
		name:        name,
		description: description,
		schema:      schema,
		fn:          fn,
	}, nil
}

func (t *TypedTool[I, O]) Definition() Definition {
	return Definition{
		Name:        t.name,
		Description: t.description,
		Parameters:  t.schema,
	}
}

func (t *TypedTool[I, O]) Run(ctx context.Context, args json.RawMessage) (string, error) {
	if len(args) == 0 {
		args = json.RawMessage(`{}`)
	}
	var input I
	if err := json.Unmarshal(args, &input); err != nil {
		return Fail(err.Error())
	}
	out, err := t.fn(ctx, input)
	if err != nil {
		return Fail(err.Error())
	}
	return OK(out)
}

// SchemaFor infers a JSON Schema from T using struct field types and json tags.
func SchemaFor[T any]() (json.RawMessage, error) {
	s, err := jsonschema.For[T](nil)
	if err != nil {
		return nil, err
	}
	return json.Marshal(s)
}
