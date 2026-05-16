package agent_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/iamhectordev/hector/modules/agent"
	"github.com/iamhectordev/hector/modules/tools"
	"github.com/iamhectordev/hector/pkg/llm/schema"
	llmtest "github.com/iamhectordev/hector/pkg/llm/testing"
	"github.com/iamhectordev/hector/pkg/session"
	"github.com/stretchr/testify/require"
)

// funcTool adapts a plain function into the tools.Tool interface.
type funcTool struct {
	name    string
	desc    string
	params  json.RawMessage
	execute func(context.Context, json.RawMessage) (string, error)
}

func (f funcTool) Definition() tools.Definition {
	return tools.Definition{Name: f.name, Description: f.desc, Parameters: f.params}
}
func (f funcTool) Run(ctx context.Context, args json.RawMessage) (string, error) {
	return f.execute(ctx, args)
}

func TestLoop_Run_StopsOnFinishReasonStop(t *testing.T) {
	loop := agent.NewLoop(llmtest.NewCompleter(t, llmtest.Stop("hi")))
	reply, err := loop.Run(t.Context(), []*schema.Message{schema.UserMessage("hello")})
	require.NoError(t, err)
	require.Equal(t, "hi", reply.Content)
	require.Equal(t, schema.RoleAssistant, reply.Role)
}

func TestLoop_Run_PropagatesCompleterError(t *testing.T) {
	boom := errors.New("llm down")
	loop := agent.NewLoop(llmtest.NewCompleter(t, llmtest.Error(boom)))
	reply, err := loop.Run(t.Context(), []*schema.Message{schema.UserMessage("hello")})
	require.ErrorIs(t, err, boom)
	require.Nil(t, reply)
}

func TestLoop_Run_NilReplyIsError(t *testing.T) {
	loop := agent.NewLoop(llmtest.NewCompleter(t, llmtest.Nil()))
	reply, err := loop.Run(t.Context(), []*schema.Message{schema.UserMessage("hello")})
	require.Error(t, err)
	require.Nil(t, reply)
}

func TestLoop_Run_UnknownFinishReasonIsError(t *testing.T) {
	m := schema.AssistantMessage("")
	m.FinishReason = "content_filter"
	loop := agent.NewLoop(&rawCompleter{msg: m})
	_, err := loop.Run(t.Context(), []*schema.Message{schema.UserMessage("hello")})
	require.ErrorContains(t, err, "content_filter")
}

func TestLoop_Run_ExecutesToolAndContinues(t *testing.T) {
	call := llmtest.Call("c1", "echo", `{"text":"ping"}`)

	var got string
	echo := funcTool{
		name:   "echo",
		desc:   "echoes text",
		params: json.RawMessage(`{"type":"object","properties":{"text":{"type":"string"}}}`),
		execute: func(_ context.Context, args json.RawMessage) (string, error) {
			var p struct{ Text string }
			_ = json.Unmarshal(args, &p)
			got = p.Text
			return p.Text, nil
		},
	}

	registry, err := tools.NewRegistry(echo)
	require.NoError(t, err)
	loop := agent.NewLoop(
		llmtest.NewCompleter(t, llmtest.ToolCalls(call), llmtest.Stop("done")),
		agent.WithTools(registry),
	)

	reply, err := loop.Run(t.Context(), []*schema.Message{schema.UserMessage("hello")})
	require.NoError(t, err)
	require.Equal(t, "done", reply.Content)
	require.Equal(t, "ping", got)
}

func TestLoop_Run_MultipleToolRoundsThenStop(t *testing.T) {
	var calls int
	noop := funcTool{
		name:   "noop",
		desc:   "does nothing",
		params: json.RawMessage(`{"type":"object","properties":{}}`),
		execute: func(_ context.Context, _ json.RawMessage) (string, error) {
			calls++
			return "ok", nil
		},
	}

	registry, err := tools.NewRegistry(noop)
	require.NoError(t, err)
	loop := agent.NewLoop(
		llmtest.NewCompleter(t,
			llmtest.ToolCalls(llmtest.Call("c1", "noop", `{}`)),
			llmtest.ToolCalls(llmtest.Call("c2", "noop", `{}`)),
			llmtest.Stop("final"),
		),
		agent.WithTools(registry),
	)

	reply, err := loop.Run(t.Context(), []*schema.Message{schema.UserMessage("go")})
	require.NoError(t, err)
	require.Equal(t, "final", reply.Content)
	require.Equal(t, 2, calls)
}

func TestLoop_Run_RecordsUserMessageAndAssistantReply(t *testing.T) {
	store := &recordingSessionStore{}
	loop := agent.NewLoop(
		llmtest.NewCompleter(t, llmtest.Stop("hi")),
		agent.WithSessionStore(store),
	)
	ctx := session.With(t.Context(), session.Session{SourceURI: "tui://stdout"})

	reply, err := loop.Run(ctx, []*schema.Message{schema.UserMessage("hello")})
	require.NoError(t, err)
	require.Equal(t, "hi", reply.Content)

	require.Equal(t, []schema.Message{
		{Role: schema.RoleUser, Content: "hello"},
		{Role: schema.RoleAssistant, Content: "hi", FinishReason: schema.FinishReasonStop},
	}, store.messages)
	require.Equal(t, []string{"tui://stdout", "tui://stdout"}, store.sourceURIs)
}

func TestLoop_Run_RecordsToolCallTranscriptInOrder(t *testing.T) {
	store := &recordingSessionStore{}
	call := llmtest.Call("call_1", "echo", `{"text":"ping"}`)
	echo := funcTool{
		name:   "echo",
		desc:   "echoes text",
		params: json.RawMessage(`{"type":"object","properties":{"text":{"type":"string"}}}`),
		execute: func(_ context.Context, _ json.RawMessage) (string, error) {
			return "pong", nil
		},
	}
	registry, err := tools.NewRegistry(echo)
	require.NoError(t, err)

	loop := agent.NewLoop(
		llmtest.NewCompleter(t, llmtest.ToolCalls(call), llmtest.Stop("done")),
		agent.WithTools(registry),
		agent.WithSessionStore(store),
	)
	ctx := session.With(t.Context(), session.Session{SourceURI: "slack://C123/1"})

	_, err = loop.Run(ctx, []*schema.Message{schema.UserMessage("hello")})
	require.NoError(t, err)

	require.Equal(t, []schema.Message{
		{Role: schema.RoleUser, Content: "hello"},
		{Role: schema.RoleAssistant, FinishReason: schema.FinishReasonToolCalls, ToolCalls: []schema.ToolCall{call}},
		{Role: schema.RoleTool, Content: "pong", ToolCallID: "call_1"},
		{Role: schema.RoleAssistant, Content: "done", FinishReason: schema.FinishReasonStop},
	}, store.messages)
}

func TestLoop_Run_DoesNotRecordWhenCompleteFails(t *testing.T) {
	store := &recordingSessionStore{}
	boom := errors.New("llm down")
	loop := agent.NewLoop(
		llmtest.NewCompleter(t, llmtest.Error(boom)),
		agent.WithSessionStore(store),
	)
	ctx := session.With(t.Context(), session.Session{SourceURI: "tui://stdout"})

	_, err := loop.Run(ctx, []*schema.Message{schema.UserMessage("hello")})
	require.ErrorIs(t, err, boom)
	require.Empty(t, store.messages)
}

func TestLoop_Run_ReturnsRecordError(t *testing.T) {
	boom := errors.New("record failed")
	store := &recordingSessionStore{err: boom}
	loop := agent.NewLoop(
		llmtest.NewCompleter(t, llmtest.Stop("hi")),
		agent.WithSessionStore(store),
	)
	ctx := session.With(t.Context(), session.Session{SourceURI: "tui://stdout"})

	_, err := loop.Run(ctx, []*schema.Message{schema.UserMessage("hello")})
	require.ErrorIs(t, err, boom)
}

// rawCompleter returns a fixed message, used to test unknown finish reasons.
type rawCompleter struct{ msg *schema.Message }

func (r *rawCompleter) Complete(_ context.Context, _ schema.CompletionRequest) (*schema.Message, error) {
	return r.msg, nil
}

type recordingSessionStore struct {
	err        error
	sourceURIs []string
	messages   []schema.Message
}

func (s *recordingSessionStore) Record(_ context.Context, sourceURI string, messages []*schema.Message) error {
	if s.err != nil {
		return s.err
	}
	for _, msg := range messages {
		s.sourceURIs = append(s.sourceURIs, sourceURI)
		s.messages = append(s.messages, *msg)
	}
	return nil
}
