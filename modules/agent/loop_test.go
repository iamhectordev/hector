package agent_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/iamhectordev/hector/modules/agent"
	"github.com/iamhectordev/hector/pkg/tools"
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
	reply, err := loop.Run(t.Context(), &recordingAgentContext{}, "", []*schema.Message{schema.UserMessage("hello")})
	require.NoError(t, err)
	require.Equal(t, "hi", reply.Content)
	require.Equal(t, schema.RoleAssistant, reply.Role)
}

func TestLoop_Run_PropagatesCompleterError(t *testing.T) {
	boom := errors.New("llm down")
	loop := agent.NewLoop(llmtest.NewCompleter(t, llmtest.Error(boom)))
	reply, err := loop.Run(t.Context(), &recordingAgentContext{}, "", []*schema.Message{schema.UserMessage("hello")})
	require.ErrorIs(t, err, boom)
	require.Nil(t, reply)
}

func TestLoop_Run_NilReplyIsError(t *testing.T) {
	loop := agent.NewLoop(llmtest.NewCompleter(t, llmtest.Nil()))
	reply, err := loop.Run(t.Context(), &recordingAgentContext{}, "", []*schema.Message{schema.UserMessage("hello")})
	require.Error(t, err)
	require.Nil(t, reply)
}

func TestLoop_Run_UnknownFinishReasonIsError(t *testing.T) {
	m := schema.AssistantMessage("")
	m.FinishReason = "content_filter"
	loop := agent.NewLoop(&rawCompleter{msg: m})
	_, err := loop.Run(t.Context(), &recordingAgentContext{}, "", []*schema.Message{schema.UserMessage("hello")})
	require.ErrorContains(t, err, "content_filter")
}

func TestLoop_Run_RequiresAgentContext(t *testing.T) {
	loop := agent.NewLoop(llmtest.NewCompleter(t, llmtest.Stop("hi")))
	reply, err := loop.Run(t.Context(), nil, "", []*schema.Message{schema.UserMessage("hello")})
	require.ErrorContains(t, err, "context is required")
	require.Nil(t, reply)
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

	reply, err := loop.Run(t.Context(), &recordingAgentContext{}, "", []*schema.Message{schema.UserMessage("hello")})
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

	reply, err := loop.Run(t.Context(), &recordingAgentContext{}, "", []*schema.Message{schema.UserMessage("go")})
	require.NoError(t, err)
	require.Equal(t, "final", reply.Content)
	require.Equal(t, 2, calls)
}

func TestLoop_Run_RecordsUserMessageAndAssistantReply(t *testing.T) {
	agentCtx := &recordingAgentContext{}
	loop := agent.NewLoop(llmtest.NewCompleter(t, llmtest.Stop("hi")))

	reply, err := loop.Run(t.Context(), agentCtx, "", []*schema.Message{schema.UserMessage("hello")})
	require.NoError(t, err)
	require.Equal(t, "hi", reply.Content)

	require.Equal(t, []schema.Message{
		{Role: schema.RoleUser, Content: "hello"},
		{Role: schema.RoleAssistant, Content: "hi", FinishReason: schema.FinishReasonStop},
	}, agentCtx.messages)
}

func TestLoop_Run_LoadsSessionHistory(t *testing.T) {
	agentCtx := &recordingAgentContext{
		history: []*schema.Message{
			schema.UserMessage("previous user"),
			schema.AssistantMessage("previous assistant"),
		},
	}
	completer := &requestCompleter{
		reply: &schema.Message{
			Role:         schema.RoleAssistant,
			Content:      "current assistant",
			FinishReason: schema.FinishReasonStop,
		},
	}
	loop := agent.NewLoop(completer)

	reply, err := loop.Run(t.Context(), agentCtx, "", []*schema.Message{schema.UserMessage("current user")})
	require.NoError(t, err)
	require.Equal(t, "current assistant", reply.Content)

	require.Len(t, completer.requests, 1)
	require.Equal(t, []*schema.Message{
		schema.UserMessage("previous user"),
		schema.AssistantMessage("previous assistant"),
		schema.UserMessage("current user"),
	}, completer.requests[0].Messages)
	require.Equal(t, []schema.Message{
		{Role: schema.RoleUser, Content: "current user"},
		{Role: schema.RoleAssistant, Content: "current assistant", FinishReason: schema.FinishReasonStop},
	}, agentCtx.messages)
}

func TestLoop_Run_AddsAgentSessionToCompleterContext(t *testing.T) {
	agentCtx := &recordingAgentContext{
		session: session.Session{
			ID:        "sess_123",
			SourceURI: "slack://D123/1",
		},
	}
	completer := &sessionCaptureCompleter{
		reply: &schema.Message{
			Role:         schema.RoleAssistant,
			Content:      "assistant",
			FinishReason: schema.FinishReasonStop,
		},
	}
	loop := agent.NewLoop(completer)

	_, err := loop.Run(t.Context(), agentCtx, "", []*schema.Message{schema.UserMessage("hello")})
	require.NoError(t, err)

	require.Equal(t, agentCtx.session, completer.session)
}

func TestLoop_Run_ContinuesWithoutHistoryWhenSessionStoreFails(t *testing.T) {
	agentCtx := &recordingAgentContext{messagesErr: errors.New("db unavailable")}
	completer := &requestCompleter{
		reply: &schema.Message{
			Role:         schema.RoleAssistant,
			Content:      "assistant",
			FinishReason: schema.FinishReasonStop,
		},
	}
	loop := agent.NewLoop(completer)

	reply, err := loop.Run(t.Context(), agentCtx, "", []*schema.Message{schema.UserMessage("hello")})
	require.NoError(t, err)
	require.Equal(t, "assistant", reply.Content)

	require.Len(t, completer.requests, 1)
	require.Equal(t, []*schema.Message{schema.UserMessage("hello")}, completer.requests[0].Messages)
}

func TestLoop_Run_RecordsToolCallTranscriptInOrder(t *testing.T) {
	agentCtx := &recordingAgentContext{}
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
	)

	_, err = loop.Run(t.Context(), agentCtx, "", []*schema.Message{schema.UserMessage("hello")})
	require.NoError(t, err)

	require.Equal(t, []schema.Message{
		{Role: schema.RoleUser, Content: "hello"},
		{Role: schema.RoleAssistant, FinishReason: schema.FinishReasonToolCalls, ToolCalls: []schema.ToolCall{call}},
		{Role: schema.RoleTool, Content: "pong", ToolCallID: "call_1"},
		{Role: schema.RoleAssistant, Content: "done", FinishReason: schema.FinishReasonStop},
	}, agentCtx.messages)
}

func TestLoop_Run_DoesNotRecordWhenCompleteFails(t *testing.T) {
	agentCtx := &recordingAgentContext{}
	boom := errors.New("llm down")
	loop := agent.NewLoop(llmtest.NewCompleter(t, llmtest.Error(boom)))

	_, err := loop.Run(t.Context(), agentCtx, "", []*schema.Message{schema.UserMessage("hello")})
	require.ErrorIs(t, err, boom)
	require.Empty(t, agentCtx.messages)
}

func TestLoop_Run_ContinuesWhenRecordFails(t *testing.T) {
	agentCtx := &recordingAgentContext{err: errors.New("record failed")}
	loop := agent.NewLoop(llmtest.NewCompleter(t, llmtest.Stop("hi")))

	reply, err := loop.Run(t.Context(), agentCtx, "", []*schema.Message{schema.UserMessage("hello")})
	require.NoError(t, err)
	require.Equal(t, "hi", reply.Content)
}

// rawCompleter returns a fixed message, used to test unknown finish reasons.
type rawCompleter struct{ msg *schema.Message }

func (r *rawCompleter) Complete(_ context.Context, _ schema.CompletionRequest) (*schema.Message, error) {
	return r.msg, nil
}

type recordingAgentContext struct {
	err         error
	messagesErr error
	history     []*schema.Message
	messages    []schema.Message
	session     session.Session
}

func (c *recordingAgentContext) Session(context.Context) (session.Session, error) {
	return c.session, nil
}

func (c *recordingAgentContext) Messages(context.Context) ([]*schema.Message, error) {
	if c.messagesErr != nil {
		return nil, c.messagesErr
	}
	return c.history, nil
}

func (c *recordingAgentContext) Record(_ context.Context, messages []*schema.Message) error {
	if c.err != nil {
		return c.err
	}
	for _, msg := range messages {
		c.messages = append(c.messages, *msg)
	}
	return nil
}

type requestCompleter struct {
	reply    *schema.Message
	requests []schema.CompletionRequest
}

func (c *requestCompleter) Complete(_ context.Context, req schema.CompletionRequest) (*schema.Message, error) {
	c.requests = append(c.requests, req)
	return c.reply, nil
}

type sessionCaptureCompleter struct {
	reply   *schema.Message
	session session.Session
}

func (c *sessionCaptureCompleter) Complete(ctx context.Context, _ schema.CompletionRequest) (*schema.Message, error) {
	c.session, _ = session.From(ctx)
	return c.reply, nil
}
