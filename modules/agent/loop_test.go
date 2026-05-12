package agent_test

import (
	"context"
	"errors"
	"testing"

	"github.com/iamhectordev/hector/modules/agent"
	"github.com/iamhectordev/hector/pkg/llm/schema"
	"github.com/stretchr/testify/require"
)

type stubCompleter struct {
	reply *schema.Message
	err   error
}

func (s *stubCompleter) Complete(_ context.Context, _ schema.CompletionRequest) (*schema.Message, error) {
	return s.reply, s.err
}

func TestLoop_Run_ReturnsAssistantReply(t *testing.T) {
	loop := agent.NewLoop(&stubCompleter{reply: schema.AssistantMessage("hi")})
	reply, err := loop.Run(t.Context(), []*schema.Message{schema.UserMessage("hello")})
	require.NoError(t, err)
	require.Equal(t, "hi", reply.Content)
	require.Equal(t, schema.RoleAssistant, reply.Role)
}

func TestLoop_Run_PropagatesCompleterError(t *testing.T) {
	boom := errors.New("llm down")
	loop := agent.NewLoop(&stubCompleter{err: boom})
	reply, err := loop.Run(t.Context(), []*schema.Message{schema.UserMessage("hello")})
	require.ErrorIs(t, err, boom)
	require.Nil(t, reply)
}

func TestLoop_Run_NilReplyIsError(t *testing.T) {
	loop := agent.NewLoop(&stubCompleter{reply: nil})
	reply, err := loop.Run(t.Context(), []*schema.Message{schema.UserMessage("hello")})
	require.Error(t, err)
	require.Nil(t, reply)
}
