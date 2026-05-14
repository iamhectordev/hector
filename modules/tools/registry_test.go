package tools_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iamhectordev/hector/modules/tools"
)

func TestRegistryRun(t *testing.T) {
	registry, err := tools.NewRegistry(echoTool{})
	require.NoError(t, err)

	output, err := registry.Run(t.Context(), "test.echo", json.RawMessage(`{"value":"hello"}`))
	require.NoError(t, err)
	require.Equal(t, `{"value":"hello"}`, output)
}

func TestRegistryRunUnknownTool(t *testing.T) {
	registry, err := tools.NewRegistry()
	require.NoError(t, err)

	output, err := registry.Run(t.Context(), "missing", json.RawMessage(`{}`))
	require.Error(t, err)
	require.Empty(t, output)
	require.Contains(t, err.Error(), `unknown tool "missing"`)
}

func TestRegistryDefinitionsAreSorted(t *testing.T) {
	registry, err := tools.NewRegistry(
		namedTool{name: "z.last"},
		namedTool{name: "a.first"},
		namedTool{name: "m.middle"},
	)
	require.NoError(t, err)

	defs := registry.Definitions()
	require.Len(t, defs, 3)
	require.Equal(t, "a.first", defs[0].Name)
	require.Equal(t, "m.middle", defs[1].Name)
	require.Equal(t, "z.last", defs[2].Name)
}

func TestRegistryRunsTimeNow(t *testing.T) {
	registry, err := tools.NewRegistry(tools.TimeNow{})
	require.NoError(t, err)

	output, err := registry.Run(t.Context(), "time.now", nil)
	require.NoError(t, err)
	require.Contains(t, output, "UTC")
}

type namedTool struct {
	name string
}

func (n namedTool) Definition() tools.Definition {
	return tools.Definition{
		Name:        n.name,
		Description: "A named test tool.",
		Parameters:  json.RawMessage(`{"type":"object"}`),
	}
}

func (namedTool) Run(_ context.Context, _ json.RawMessage) (string, error) {
	return "ok", nil
}
