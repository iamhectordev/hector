package tools_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	pkgtools "github.com/iamhectordev/hector/pkg/tools"
	"github.com/iamhectordev/hector/pkg/waffle"
	"github.com/stretchr/testify/require"

	"github.com/iamhectordev/hector/modules/tools"
)

func TestModuleEmitsCallCompleted(t *testing.T) {
	ctx := t.Context()
	store := waffle.NewMemoryStore()
	bus, err := waffle.NewEventBus(waffle.WithStore(store))
	require.NoError(t, err)
	registry, err := pkgtools.NewRegistry(echoTool{})
	require.NoError(t, err)
	module, err := tools.NewModule(bus, registry)
	require.NoError(t, err)
	require.NoError(t, module.Init(ctx))
	require.NoError(t, bus.Start(ctx))

	require.NoError(t, bus.Record(ctx, tools.CallRequested.New(tools.CallRequestedData{
		CallID: "call_123",
		Name:   "test_echo",
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
	registry, err := pkgtools.NewRegistry()
	require.NoError(t, err)
	module, err := tools.NewModule(bus, registry)
	require.NoError(t, err)
	require.NoError(t, module.Init(ctx))
	require.NoError(t, bus.Start(ctx))

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
	registry, err := pkgtools.NewRegistry(failingTool{})
	require.NoError(t, err)
	module, err := tools.NewModule(bus, registry)
	require.NoError(t, err)
	require.NoError(t, module.Init(ctx))
	require.NoError(t, bus.Start(ctx))

	require.NoError(t, bus.Record(ctx, tools.CallRequested.New(tools.CallRequestedData{
		CallID: "call_failed",
		Name:   "test_fail",
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

func (echoTool) Definition() pkgtools.Definition {
	return pkgtools.Definition{
		Name:        "test_echo",
		Description: "Echoes the input.",
		Parameters:  json.RawMessage(`{"type":"object"}`),
	}
}

func (echoTool) Run(_ context.Context, args json.RawMessage) (string, error) {
	return string(args), nil
}

type failingTool struct{}

func (failingTool) Definition() pkgtools.Definition {
	return pkgtools.Definition{
		Name:        "test_fail",
		Description: "Always fails.",
		Parameters:  json.RawMessage(`{"type":"object"}`),
	}
}

func (failingTool) Run(context.Context, json.RawMessage) (string, error) {
	return "", errors.New("tool failed")
}

func TestModuleRejectsNilRegistry(t *testing.T) {
	bus, err := waffle.NewEventBus()
	require.NoError(t, err)
	_, err = tools.NewModule(bus, nil)
	require.ErrorIs(t, err, pkgtools.ErrNilRegistry)
}

func TestRegistryRejectsToolWithEmptyName(t *testing.T) {
	_, err := pkgtools.NewRegistry(badTool{name: ""})
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty name")
}

func TestRegistryRejectsToolWithEmptyDescription(t *testing.T) {
	_, err := pkgtools.NewRegistry(badTool{name: "test", description: ""})
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty description")
}

func TestRegistryRejectsToolWithNilParameters(t *testing.T) {
	_, err := pkgtools.NewRegistry(badTool{name: "test", description: "desc", parameters: nil})
	require.Error(t, err)
	require.Contains(t, err.Error(), "nil parameters")
}

func TestRegistryRejectsDuplicateToolNames(t *testing.T) {
	tool1 := echoTool{}
	tool2 := echoTool{}
	_, err := pkgtools.NewRegistry(tool1, tool2)
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate")
}

type badTool struct {
	name        string
	description string
	parameters  json.RawMessage
}

func (b badTool) Definition() pkgtools.Definition {
	return pkgtools.Definition{
		Name:        b.name,
		Description: b.description,
		Parameters:  b.parameters,
	}
}

func (badTool) Run(context.Context, json.RawMessage) (string, error) {
	return "", nil
}
