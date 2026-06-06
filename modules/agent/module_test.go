package agent_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/iamhectordev/hector/modules/agent"
	"github.com/iamhectordev/hector/modules/tui"
	"github.com/iamhectordev/hector/pkg/llm/schema"
	llmtest "github.com/iamhectordev/hector/pkg/llm/testing"
	"github.com/iamhectordev/hector/pkg/session"
	"github.com/iamhectordev/hector/pkg/supervisor"
	"github.com/iamhectordev/hector/pkg/waffle"
	"github.com/stretchr/testify/require"
	"strings"
)

func TestModule_EmitsTurnEndAfterSuccessfulTurn(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	bus, err := waffle.NewEventBus(waffle.WithWorkers(2))
	require.NoError(t, err)

	var captured agent.TurnEndData
	var once sync.Once
	received := make(chan struct{})
	err = waffle.On(bus, agent.TurnEnd).Handle("test.capture", func(_ context.Context, e waffle.Event[agent.TurnEndData]) error {
		once.Do(func() {
			captured = e.Data()
			close(received)
		})
		return nil
	})
	require.NoError(t, err)

	sv, err := supervisor.New([]supervisor.Module{
		agent.NewModule(bus,
			agent.NewLoop(llmtest.NewCompleter(t, llmtest.Stop("reply text"))),
			agent.WithSessionStore(moduleTestSessionStore{}),
		),
		tui.NewModule(bus, tui.WithReader(strings.NewReader("hello\n"))),
	}, supervisor.WithPostInitHook("bus.start", bus.Start))
	require.NoError(t, err)

	go sv.Run(ctx)

	select {
	case <-received:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for TurnEnd event")
	}

	cancel()
	require.NoError(t, bus.Shutdown(context.Background()))

	require.NotEmpty(t, captured.SourceURI)
	require.Equal(t, 0, captured.TurnOffset) // fresh session — no prior history
}

func TestModule_DoesNotEmitTurnEndOnRunnerError(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	bus, err := waffle.NewEventBus(waffle.WithWorkers(2))
	require.NoError(t, err)

	emitted := make(chan struct{}, 1)
	err = waffle.On(bus, agent.TurnEnd).Handle("test.capture", func(_ context.Context, _ waffle.Event[agent.TurnEndData]) error {
		emitted <- struct{}{}
		return nil
	})
	require.NoError(t, err)

	completerDone := make(chan struct{})
	sv, err := supervisor.New([]supervisor.Module{
		agent.NewModule(bus,
			agent.NewLoop(&signallingErrorCompleter{signal: completerDone}),
			agent.WithSessionStore(moduleTestSessionStore{}),
		),
		tui.NewModule(bus, tui.WithReader(strings.NewReader("hello\n"))),
	}, supervisor.WithPostInitHook("bus.start", bus.Start))
	require.NoError(t, err)

	go sv.Run(ctx)

	select {
	case <-completerDone:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for completer to be called")
	}

	select {
	case <-emitted:
		t.Fatal("TurnEnd should not have been emitted on error")
	case <-time.After(100 * time.Millisecond):
	}

	cancel()
	require.NoError(t, bus.Shutdown(context.Background()))
}

// moduleTestSessionStore is a no-op session store for agent module tests.
type moduleTestSessionStore struct{}

func (moduleTestSessionStore) GetOrCreate(_ context.Context, sourceURI string) (session.StoredSession, error) {
	return session.StoredSession{SourceURI: sourceURI}, nil
}
func (moduleTestSessionStore) Messages(context.Context, string) ([]*schema.Message, error) {
	return nil, nil
}
func (moduleTestSessionStore) Record(context.Context, string, []*schema.Message) error {
	return nil
}

// signallingErrorCompleter signals a channel then returns an error.
type signallingErrorCompleter struct {
	signal chan struct{}
}

func (c *signallingErrorCompleter) Complete(_ context.Context, _ schema.CompletionRequest) (*schema.Message, error) {
	select {
	case c.signal <- struct{}{}:
	default:
	}
	return nil, llmtest.ErrLLMDown
}
