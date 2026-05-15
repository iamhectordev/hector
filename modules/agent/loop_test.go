package agent_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/iamhectordev/hector/modules/agent"
	"github.com/iamhectordev/hector/modules/tools"
	llmtest "github.com/iamhectordev/hector/pkg/llm/testing"
	"github.com/iamhectordev/hector/pkg/llm/schema"
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

// rawCompleter returns a fixed message, used to test unknown finish reasons.
type rawCompleter struct{ msg *schema.Message }

func (r *rawCompleter) Complete(_ context.Context, _ schema.CompletionRequest) (*schema.Message, error) {
	return r.msg, nil
}
