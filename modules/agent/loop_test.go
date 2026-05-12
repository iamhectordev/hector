package agent_test

import (
	"context"
	"errors"
	"testing"

	"github.com/iamhectordev/hector/modules/agent"
	"github.com/iamhectordev/hector/pkg/llm/message"
	"github.com/stretchr/testify/require"
)

type stubCompleter struct {
	reply *message.Message
	err   error
}

func (s *stubCompleter) Complete(_ context.Context, _ []*message.Message) (*message.Message, error) {
	return s.reply, s.err
}

func TestLoop_Run_ReturnsAssistantReply(t *testing.T) {
	loop := agent.NewLoop(&stubCompleter{reply: message.AssistantMessage("hi")})
	reply, err := loop.Run(t.Context(), []*message.Message{message.UserMessage("hello")})
	require.NoError(t, err)
	require.Equal(t, "hi", reply.Content)
	require.Equal(t, message.Assistant, reply.Role)
}

func TestLoop_Run_PropagatesCompleterError(t *testing.T) {
	boom := errors.New("llm down")
	loop := agent.NewLoop(&stubCompleter{err: boom})
	reply, err := loop.Run(t.Context(), []*message.Message{message.UserMessage("hello")})
	require.ErrorIs(t, err, boom)
	require.Nil(t, reply)
}

func TestLoop_Run_NilReplyIsError(t *testing.T) {
	loop := agent.NewLoop(&stubCompleter{reply: nil})
	reply, err := loop.Run(t.Context(), []*message.Message{message.UserMessage("hello")})
	require.Error(t, err)
	require.Nil(t, reply)
}
