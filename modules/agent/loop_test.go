package agent_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/iamhectordev/hector/modules/agent"
	"github.com/iamhectordev/hector/modules/tools"
	"github.com/iamhectordev/hector/pkg/llm/schema"
	"github.com/stretchr/testify/require"
)

// queueCompleter returns replies in order, one per Complete call.
type queueCompleter struct {
	replies []*schema.Message
	errs    []error
	i       int
}

func (q *queueCompleter) Complete(_ context.Context, _ schema.CompletionRequest) (*schema.Message, error) {
	if q.i >= len(q.replies) {
		return nil, errors.New("queueCompleter: no more replies")
	}
	reply := q.replies[q.i]
	var err error
	if q.i < len(q.errs) {
		err = q.errs[q.i]
	}
	q.i++
	return reply, err
}

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

func stopMsg(content string) *schema.Message {
	m := schema.AssistantMessage(content)
	m.FinishReason = schema.FinishReasonStop
	return m
}

func toolCallMsg(calls ...schema.ToolCall) *schema.Message {
	return &schema.Message{Role: schema.RoleAssistant, FinishReason: schema.FinishReasonToolCalls, ToolCalls: calls}
}

func TestLoop_Run_StopsOnFinishReasonStop(t *testing.T) {
	loop := agent.NewLoop(&queueCompleter{replies: []*schema.Message{stopMsg("hi")}})
	reply, err := loop.Run(t.Context(), []*schema.Message{schema.UserMessage("hello")})
	require.NoError(t, err)
	require.Equal(t, "hi", reply.Content)
	require.Equal(t, schema.RoleAssistant, reply.Role)
}

func TestLoop_Run_PropagatesCompleterError(t *testing.T) {
	boom := errors.New("llm down")
	loop := agent.NewLoop(&queueCompleter{
		replies: []*schema.Message{nil},
		errs:    []error{boom},
	})
	reply, err := loop.Run(t.Context(), []*schema.Message{schema.UserMessage("hello")})
	require.ErrorIs(t, err, boom)
	require.Nil(t, reply)
}

func TestLoop_Run_NilReplyIsError(t *testing.T) {
	loop := agent.NewLoop(&queueCompleter{replies: []*schema.Message{nil}})
	reply, err := loop.Run(t.Context(), []*schema.Message{schema.UserMessage("hello")})
	require.Error(t, err)
	require.Nil(t, reply)
}

func TestLoop_Run_UnknownFinishReasonIsError(t *testing.T) {
	m := schema.AssistantMessage("")
	m.FinishReason = "content_filter"
	loop := agent.NewLoop(&queueCompleter{replies: []*schema.Message{m}})
	_, err := loop.Run(t.Context(), []*schema.Message{schema.UserMessage("hello")})
	require.ErrorContains(t, err, "content_filter")
}

func TestLoop_Run_ExecutesToolAndContinues(t *testing.T) {
	call := schema.ToolCall{ID: "c1", Name: "echo", Arguments: json.RawMessage(`{"text":"ping"}`)}

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

	catalog := agent.NewCatalog(echo)
	loop := agent.NewLoop(
		&queueCompleter{replies: []*schema.Message{toolCallMsg(call), stopMsg("done")}},
		agent.WithCatalog(catalog),
	)

	reply, err := loop.Run(t.Context(), []*schema.Message{schema.UserMessage("hello")})
	require.NoError(t, err)
	require.Equal(t, "done", reply.Content)
	require.Equal(t, "ping", got)
}

func TestLoop_Run_MultipleToolRoundsThenStop(t *testing.T) {
	call := func(id string) schema.ToolCall {
		return schema.ToolCall{ID: id, Name: "noop", Arguments: json.RawMessage(`{}`)}
	}

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

	catalog := agent.NewCatalog(noop)
	loop := agent.NewLoop(
		&queueCompleter{replies: []*schema.Message{toolCallMsg(call("c1")), toolCallMsg(call("c2")), stopMsg("final")}},
		agent.WithCatalog(catalog),
	)

	reply, err := loop.Run(t.Context(), []*schema.Message{schema.UserMessage("go")})
	require.NoError(t, err)
	require.Equal(t, "final", reply.Content)
	require.Equal(t, 2, calls)
}
