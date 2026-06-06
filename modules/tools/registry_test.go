package tools_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/iamhectordev/hector/pkg/telem"
	"github.com/stretchr/testify/require"

	"github.com/iamhectordev/hector/modules/tools"
)

func TestRegistryRun(t *testing.T) {
	registry, err := tools.NewRegistry(echoTool{})
	require.NoError(t, err)

	output, err := registry.Run(t.Context(), "test_echo", json.RawMessage(`{"value":"hello"}`))
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
		namedTool{name: "z_last"},
		namedTool{name: "a_first"},
		namedTool{name: "m_middle"},
	)
	require.NoError(t, err)

	defs := registry.Definitions()
	require.Len(t, defs, 3)
	require.Equal(t, "a_first", defs[0].Name)
	require.Equal(t, "m_middle", defs[1].Name)
	require.Equal(t, "z_last", defs[2].Name)
}

func TestRegistryRunsTimeNow(t *testing.T) {
	tn, err := tools.NewTimeNow()
	require.NoError(t, err)

	registry, err := tools.NewRegistry(tn)
	require.NoError(t, err)

	output, err := registry.Run(t.Context(), "time_now", nil)
	require.NoError(t, err)
	require.Contains(t, output, "UTC")
}

func TestRegistryRunTracesExecution(t *testing.T) {
	recorder := newSpanRecorder(t)
	registry, err := tools.NewRegistry(echoTool{})
	require.NoError(t, err)

	ctx, parent := telem.Trace(t.Context(), "tool.call")
	output, err := registry.Run(ctx, "test_echo", json.RawMessage(`{"value":"hello"}`))
	parent.End(nil)

	require.NoError(t, err)
	require.Equal(t, `{"value":"hello"}`, output)

	span := findSpan(t, recorder.Ended(), "tool.registry.run")
	require.Equal(t, parent.SpanContext().SpanID(), span.Parent().SpanID())
	require.Equal(t, "test_echo", requireSpanAttr(t, span, "tool.name"))
	require.True(t, requireSpanAttrBool(t, span, "tool.found"))
}

func TestRegistryRejectsNonSnakeCaseToolName(t *testing.T) {
	_, err := tools.NewRegistry(namedTool{name: "bad.name"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "snake_case")
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
