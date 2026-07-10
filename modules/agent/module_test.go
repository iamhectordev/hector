package agent_test

import (
	"context"
	"sync"
	"testing"
	"time"

	islack "github.com/iamhectordev/hector/integrations/slack"
	"github.com/iamhectordev/hector/modules/agent"
	"github.com/iamhectordev/hector/modules/tui"
	"github.com/iamhectordev/hector/pkg/llm/schema"
	llmtest "github.com/iamhectordev/hector/pkg/llm/testing"
	"github.com/iamhectordev/hector/pkg/session"
	"github.com/iamhectordev/hector/pkg/supervisor"
	"github.com/iamhectordev/hector/pkg/waffle"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"strings"
)

func TestModule_IgnoresMessageWhenPerceptionSaysIgnore(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	bus, err := waffle.NewEventBus(waffle.WithWorkers(2))
	require.NoError(t, err)

	runner := &runnerSpy{}
	sv, err := supervisor.New([]supervisor.Module{
		agent.NewModule(bus, runner,
			agent.WithSessionStore(moduleTestSessionStore{}),
			agent.WithPerceiver(staticPerceiver{result: agent.PerceptionResult{
				Action: agent.PerceptionActionIgnore,
				Reason: "ambient chatter",
			}}),
			agent.WithConfig(agent.Config{
				Perception: agent.PerceptionConfig{Enabled: true},
			}),
		),
	}, supervisor.WithPostInitHook("bus.start", bus.Start))
	require.NoError(t, err)

	go sv.Run(ctx)

	require.Eventually(t, func() bool {
		err = bus.Record(context.Background(), islack.MessageReceived.New(islack.MessageReceivedData{
			Channel:  islack.Channel{ID: "D123", Type: islack.ChannelTypeDM},
			ThreadTS: "1710000000.000100",
			TS:       "1710000000.000100",
			Sender:   islack.Sender{ID: "U123", Name: "alice"},
			Text:     "hello there",
		}))
		return err == nil
	}, 2*time.Second, 20*time.Millisecond)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return runner.CallCount() == 0
	}, 500*time.Millisecond, 20*time.Millisecond)

	cancel()
	require.NoError(t, bus.Shutdown(context.Background()))
}

func TestModule_QueuesMessageWhenPerceptionSaysQueue(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	bus, err := waffle.NewEventBus(waffle.WithWorkers(2))
	require.NoError(t, err)

	runner := &runnerSpy{}
	turnEnded := make(chan struct{}, 1)
	err = waffle.On(bus, agent.TurnEnd).Handle("test.capture", func(_ context.Context, _ waffle.Event[agent.TurnEndData]) error {
		select {
		case turnEnded <- struct{}{}:
		default:
		}
		return nil
	})
	require.NoError(t, err)

	sv, err := supervisor.New([]supervisor.Module{
		agent.NewModule(bus, runner,
			agent.WithSessionStore(moduleTestSessionStore{}),
			agent.WithPerceiver(staticPerceiver{result: agent.PerceptionResult{
				Action: agent.PerceptionActionQueue,
				Reason: "direct question",
			}}),
			agent.WithConfig(agent.Config{
				Perception: agent.PerceptionConfig{Enabled: true},
			}),
		),
	}, supervisor.WithPostInitHook("bus.start", bus.Start))
	require.NoError(t, err)

	go sv.Run(ctx)

	require.Eventually(t, func() bool {
		err = bus.Record(context.Background(), islack.MessageReceived.New(islack.MessageReceivedData{
			Channel:  islack.Channel{ID: "D123", Type: islack.ChannelTypeDM},
			ThreadTS: "1710000000.000100",
			TS:       "1710000000.000100",
			Sender:   islack.Sender{ID: "U123", Name: "alice"},
			Text:     "can you help?",
		}))
		return err == nil
	}, 2*time.Second, 20*time.Millisecond)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return runner.CallCount() == 1
	}, 2*time.Second, 20*time.Millisecond)

	select {
	case <-turnEnded:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for TurnEnd event")
	}

	cancel()
	require.NoError(t, bus.Shutdown(context.Background()))
}

func TestModule_TracesPerceptionDecision(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	recorder := newSpanRecorder(t)
	bus, err := waffle.NewEventBus(waffle.WithWorkers(2))
	require.NoError(t, err)

	runner := &runnerSpy{}
	sv, err := supervisor.New([]supervisor.Module{
		agent.NewModule(bus, runner,
			agent.WithSessionStore(moduleTestSessionStore{}),
			agent.WithPerceiver(staticPerceiver{result: agent.PerceptionResult{
				Action: agent.PerceptionActionQueue,
				Reason: "direct question",
			}}),
			agent.WithConfig(agent.Config{
				Perception: agent.PerceptionConfig{Enabled: true},
			}),
		),
	}, supervisor.WithPostInitHook("bus.start", bus.Start))
	require.NoError(t, err)

	go sv.Run(ctx)

	require.Eventually(t, func() bool {
		err = bus.Record(context.Background(), islack.MessageReceived.New(islack.MessageReceivedData{
			Channel:  islack.Channel{ID: "D123", Type: islack.ChannelTypeDM},
			ThreadTS: "1710000000.000100",
			TS:       "1710000000.000100",
			Sender:   islack.Sender{ID: "U123", Name: "alice"},
			Text:     "can you help?",
		}))
		return err == nil
	}, 2*time.Second, 20*time.Millisecond)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return runner.CallCount() == 1
	}, 2*time.Second, 20*time.Millisecond)

	span := findSpan(t, recorder.Ended(), "agent.perception.assess")
	require.Equal(t, int64(0), requireSpanAttrInt(t, span, "agent.history_message_count"))
	require.Equal(t, int64(1), requireSpanAttrInt(t, span, "agent.incoming_message_count"))
	require.Equal(t, "queue", requireSpanAttr(t, span, "agent.perception.action"))

	cancel()
	require.NoError(t, bus.Shutdown(context.Background()))
}

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

type staticPerceiver struct {
	result agent.PerceptionResult
	err    error
}

func (p staticPerceiver) Assess(_ context.Context, _ []*schema.Message, _ []*schema.Message) (agent.PerceptionResult, error) {
	return p.result, p.err
}

type runnerSpy struct {
	mu    sync.Mutex
	calls int
}

func (r *runnerSpy) Run(_ context.Context, _ agent.Context, _ string, _ []*schema.Message) (*schema.Message, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	reply := schema.AssistantMessage("ok")
	reply.FinishReason = schema.FinishReasonStop
	return reply, nil
}

func (r *runnerSpy) CallCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls
}

func newSpanRecorder(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		require.NoError(t, provider.Shutdown(context.Background()))
	})
	return recorder
}

func findSpan(t *testing.T, spans []sdktrace.ReadOnlySpan, name string) sdktrace.ReadOnlySpan {
	t.Helper()
	for _, span := range spans {
		if span.Name() == name {
			return span
		}
	}
	t.Fatalf("missing span %q", name)
	return nil
}

func requireSpanAttr(t *testing.T, span sdktrace.ReadOnlySpan, key string) string {
	t.Helper()
	for _, attr := range span.Attributes() {
		if string(attr.Key) == key {
			return attr.Value.AsString()
		}
	}
	t.Fatalf("missing attr %q on span %q", key, span.Name())
	return ""
}

func requireSpanAttrInt(t *testing.T, span sdktrace.ReadOnlySpan, key string) int64 {
	t.Helper()
	for _, attr := range span.Attributes() {
		if string(attr.Key) == key {
			return attr.Value.AsInt64()
		}
	}
	t.Fatalf("missing attr %q on span %q", key, span.Name())
	return 0
}
