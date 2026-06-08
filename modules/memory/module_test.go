package memory_test

import (
	"context"
	"sync"
	"testing"

	"github.com/iamhectordev/hector/modules/agent"
	"github.com/iamhectordev/hector/modules/memory"
	"github.com/iamhectordev/hector/pkg/llm/schema"
	llmtest "github.com/iamhectordev/hector/pkg/llm/testing"
	pkgmem "github.com/iamhectordev/hector/pkg/memory"
	"github.com/iamhectordev/hector/pkg/waffle"
	"github.com/stretchr/testify/require"
)

func TestMemoryModule_StoresExtractedObjectsOnTurnEnd(t *testing.T) {
	ctx := t.Context()
	bus, err := waffle.NewEventBus()
	require.NoError(t, err)

	sessions := &stubSessionStore{
		messages: []*schema.Message{
			schema.UserMessage("the auth service uses go"),
			schema.AssistantMessage("noted"),
		},
	}
	extracted := `{"objects":[{"content":"auth service uses go"}]}`
	extractor := llmtest.NewCompleter(t,
		llmtest.ToolCalls(llmtest.Call("c1", "produce_result", extracted)),
	)
	store := &stubMemoryStore{}

	mod, err := memory.NewModule(bus, store, sessions, extractor)
	require.NoError(t, err)
	require.NoError(t, mod.Init(ctx))
	require.NoError(t, bus.Start(ctx))

	require.NoError(t, bus.Record(ctx, agent.TurnEnd.New(agent.TurnEndData{
		SourceURI:  "tui://local",
		TurnOffset: 0,
	})))
	require.NoError(t, bus.Drain(ctx))

	puts := store.Puts()
	require.Len(t, puts, 1)
	require.Equal(t, "auth service uses go", puts[0].Content)
}

func TestMemoryModule_SlicesTurnOffsetFromHistory(t *testing.T) {
	ctx := t.Context()
	bus, err := waffle.NewEventBus()
	require.NoError(t, err)

	sessions := &stubSessionStore{
		messages: []*schema.Message{
			schema.UserMessage("old message"),         // index 0
			schema.AssistantMessage("old reply"),      // index 1
			schema.UserMessage("new fact about auth"), // index 2 — turn start
			schema.AssistantMessage("got it"),         // index 3
		},
	}

	var capturedMessages []*schema.Message
	var once sync.Once
	extractor := &capturingCompleter{
		reply: toolCallMessage("produce_result", `{"objects":[]}`),
		onComplete: func(req schema.CompletionRequest) {
			once.Do(func() { capturedMessages = req.Messages })
		},
	}

	store := &stubMemoryStore{}
	mod, err := memory.NewModule(bus, store, sessions, extractor)
	require.NoError(t, err)
	require.NoError(t, mod.Init(ctx))
	require.NoError(t, bus.Start(ctx))

	require.NoError(t, bus.Record(ctx, agent.TurnEnd.New(agent.TurnEndData{
		SourceURI:  "tui://local",
		TurnOffset: 2,
	})))
	require.NoError(t, bus.Drain(ctx))

	require.Len(t, capturedMessages, 2)
	require.Equal(t, "new fact about auth", capturedMessages[0].Content)
}

func TestMemoryModule_ContinuesWhenExtractionFails(t *testing.T) {
	ctx := t.Context()
	bus, err := waffle.NewEventBus()
	require.NoError(t, err)

	sessions := &stubSessionStore{messages: []*schema.Message{schema.UserMessage("hello")}}
	extractor := llmtest.NewCompleter(t, llmtest.Error(llmtest.ErrLLMDown))
	store := &stubMemoryStore{}

	mod, err := memory.NewModule(bus, store, sessions, extractor)
	require.NoError(t, err)
	require.NoError(t, mod.Init(ctx))
	require.NoError(t, bus.Start(ctx))

	require.NoError(t, bus.Record(ctx, agent.TurnEnd.New(agent.TurnEndData{SourceURI: "tui://local"})))
	require.NoError(t, bus.Drain(ctx)) // handler ran and returned nil (log-and-continue)

	require.Empty(t, store.Puts())
}

func TestMemoryModule_SkipsTurnWithNoMessages(t *testing.T) {
	ctx := t.Context()
	bus, err := waffle.NewEventBus()
	require.NoError(t, err)

	sessions := &stubSessionStore{messages: nil}
	extractor := llmtest.NewCompleter(t) // no scripted turns — would fail if called
	store := &stubMemoryStore{}

	mod, err := memory.NewModule(bus, store, sessions, extractor)
	require.NoError(t, err)
	require.NoError(t, mod.Init(ctx))
	require.NoError(t, bus.Start(ctx))

	require.NoError(t, bus.Record(ctx, agent.TurnEnd.New(agent.TurnEndData{SourceURI: "tui://local"})))
	require.NoError(t, bus.Drain(ctx))

	require.Empty(t, store.Puts())
}

// --- helpers ---

func toolCallMessage(toolName, argsJSON string) *schema.Message {
	return &schema.Message{
		Role:         schema.RoleAssistant,
		FinishReason: schema.FinishReasonToolCalls,
		ToolCalls:    []schema.ToolCall{{ID: "c1", Name: toolName, Arguments: []byte(argsJSON)}},
	}
}

// --- stubs ---

type stubSessionStore struct {
	messages []*schema.Message
}

func (s *stubSessionStore) Messages(_ context.Context, _ string) ([]*schema.Message, error) {
	return s.messages, nil
}

type stubMemoryStore struct {
	mu   sync.Mutex
	puts []pkgmem.Object
}

func (s *stubMemoryStore) Put(_ context.Context, obj pkgmem.Object) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.puts = append(s.puts, obj)
	return nil
}

func (s *stubMemoryStore) Puts() []pkgmem.Object {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]pkgmem.Object{}, s.puts...)
}

type capturingCompleter struct {
	reply      *schema.Message
	onComplete func(schema.CompletionRequest)
}

func (c *capturingCompleter) Complete(_ context.Context, req schema.CompletionRequest) (*schema.Message, error) {
	if c.onComplete != nil {
		c.onComplete(req)
	}
	return c.reply, nil
}
