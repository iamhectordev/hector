package tools_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iamhectordev/hector/modules/tools"
	"github.com/iamhectordev/hector/pkg/waffle"
)

func TestModuleEmitsCallCompleted(t *testing.T) {
	ctx := t.Context()
	store := waffle.NewMemoryStore()
	bus, err := waffle.NewEventBus(waffle.WithStore(store))
	require.NoError(t, err)
	module := tools.NewModule(bus, echoTool{})
	require.NoError(t, module.Init(ctx))

	require.NoError(t, bus.Record(ctx, tools.CallRequested.New(tools.CallRequestedData{
		CallID: "call_123",
		Name:   "test.echo",
		Args:   `{"value":"hello"}`,
	})))
	require.NoError(t, bus.Drain(ctx))

	completed := completedEvents(t, store)
	require.Len(t, completed, 1)
	require.Equal(t, "call_123", completed[0].CallID)
	require.Equal(t, `{"value":"hello"}`, completed[0].Output)
	require.Empty(t, completed[0].Error)
}

func TestModuleEmitsErrorForUnknownTool(t *testing.T) {
	ctx := t.Context()
	store := waffle.NewMemoryStore()
	bus, err := waffle.NewEventBus(waffle.WithStore(store))
	require.NoError(t, err)
	module := tools.NewModule(bus)
	require.NoError(t, module.Init(ctx))

	require.NoError(t, bus.Record(ctx, tools.CallRequested.New(tools.CallRequestedData{
		CallID: "call_missing",
		Name:   "missing",
		Args:   `{}`,
	})))
	require.NoError(t, bus.Drain(ctx))

	completed := completedEvents(t, store)
	require.Len(t, completed, 1)
	require.Equal(t, "call_missing", completed[0].CallID)
	require.Empty(t, completed[0].Output)
	require.Contains(t, completed[0].Error, "unknown tool")
}

func TestModuleEmitsErrorForToolFailure(t *testing.T) {
	ctx := t.Context()
	store := waffle.NewMemoryStore()
	bus, err := waffle.NewEventBus(waffle.WithStore(store))
	require.NoError(t, err)
	module := tools.NewModule(bus, failingTool{})
	require.NoError(t, module.Init(ctx))

	require.NoError(t, bus.Record(ctx, tools.CallRequested.New(tools.CallRequestedData{
		CallID: "call_failed",
		Name:   "test.fail",
		Args:   `{}`,
	})))
	require.NoError(t, bus.Drain(ctx))

	completed := completedEvents(t, store)
	require.Len(t, completed, 1)
	require.Equal(t, "call_failed", completed[0].CallID)
	require.Empty(t, completed[0].Output)
	require.Equal(t, "tool failed", completed[0].Error)
}

func completedEvents(t *testing.T, store *waffle.MemoryStore) []tools.CallCompletedData {
	t.Helper()

	var out []tools.CallCompletedData
	for _, event := range store.Events() {
		if event.Type != tools.CallCompleted.Type() {
			continue
		}
		var data tools.CallCompletedData
		require.NoError(t, json.Unmarshal(event.Payload, &data))
		out = append(out, data)
	}
	return out
}

type echoTool struct{}

func (echoTool) Definition() tools.Definition {
	return tools.Definition{
		Name:        "test.echo",
		Description: "Echoes the input.",
		Parameters:  json.RawMessage(`{"type":"object"}`),
	}
}

func (echoTool) Run(_ context.Context, args json.RawMessage) (string, error) {
	return string(args), nil
}

type failingTool struct{}

func (failingTool) Definition() tools.Definition {
	return tools.Definition{
		Name:        "test.fail",
		Description: "Always fails.",
		Parameters:  json.RawMessage(`{"type":"object"}`),
	}
}

func (failingTool) Run(context.Context, json.RawMessage) (string, error) {
	return "", errors.New("tool failed")
}
