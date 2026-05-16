package tools_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iamhectordev/hector/modules/tools"
)

type addInput struct {
	A int `json:"a" jsonschema:"first operand"`
	B int `json:"b" jsonschema:"second operand"`
}

func TestTypedToolNew(t *testing.T) {
	tool, err := tools.New(
		"math.add",
		"Returns the sum of two integers.",
		func(_ context.Context, in addInput) (int, error) {
			return in.A + in.B, nil
		},
	)
	require.NoError(t, err)

	def := tool.Definition()
	require.Equal(t, "math.add", def.Name)
	require.NotEmpty(t, def.Parameters)

	var schema map[string]any
	require.NoError(t, json.Unmarshal(def.Parameters, &schema))
	require.Equal(t, "object", schema["type"])
}

func TestTypedToolRunOK(t *testing.T) {
	tool, err := tools.New(
		"math.add",
		"Returns the sum of two integers.",
		func(_ context.Context, in addInput) (int, error) {
			return in.A + in.B, nil
		},
	)
	require.NoError(t, err)

	out, err := tool.Run(t.Context(), json.RawMessage(`{"a":3,"b":4}`))
	require.NoError(t, err)

	var env map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &env))
	require.Equal(t, "ok", env["status"])
	require.InDelta(t, 7, env["result"], 0)
}

func TestTypedToolRunError(t *testing.T) {
	tool, err := tools.New(
		"math.add",
		"Returns the sum of two integers.",
		func(_ context.Context, in addInput) (int, error) {
			return 0, errors.New("overflow")
		},
	)
	require.NoError(t, err)

	out, err := tool.Run(t.Context(), json.RawMessage(`{"a":1,"b":2}`))
	require.NoError(t, err) // Run never returns a Go error

	var env map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &env))
	require.Equal(t, "error", env["status"])
	require.Equal(t, "overflow", env["message"])
	require.Nil(t, env["result"])
}

func TestTypedToolRunInvalidArgs(t *testing.T) {
	tool, err := tools.New(
		"math.add",
		"Returns the sum of two integers.",
		func(_ context.Context, in addInput) (int, error) {
			return in.A + in.B, nil
		},
	)
	require.NoError(t, err)

	out, err := tool.Run(t.Context(), json.RawMessage(`not json`))
	require.NoError(t, err)

	var env map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &env))
	require.Equal(t, "error", env["status"])
	require.NotEmpty(t, env["message"])
}

func TestTypedToolEmptyInputSchemaHasProperties(t *testing.T) {
	tool, err := tools.New(
		"no.input",
		"A tool with no inputs.",
		func(_ context.Context, _ struct{}) (string, error) { return "ok", nil },
	)
	require.NoError(t, err)

	var schema map[string]any
	require.NoError(t, json.Unmarshal(tool.Definition().Parameters, &schema))
	_, hasProps := schema["properties"]
	require.True(t, hasProps, "object schema must include properties key for OpenAI compatibility")
}

func TestTypedToolSatisfiesTool(t *testing.T) {
	tool, err := tools.New(
		"math.add",
		"Returns the sum of two integers.",
		func(_ context.Context, in addInput) (int, error) { return 0, nil },
	)
	require.NoError(t, err)

	var _ tools.Tool = tool
}

func TestTypedToolRegistersInRegistry(t *testing.T) {
	tool, err := tools.New(
		"math.add",
		"Returns the sum of two integers.",
		func(_ context.Context, in addInput) (int, error) { return in.A + in.B, nil },
	)
	require.NoError(t, err)

	registry, err := tools.NewRegistry(tool)
	require.NoError(t, err)

	out, err := registry.Run(t.Context(), "math.add", json.RawMessage(`{"a":2,"b":3}`))
	require.NoError(t, err)

	var env map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &env))
	require.Equal(t, "ok", env["status"])
	require.InDelta(t, 5, env["result"], 0)
}
