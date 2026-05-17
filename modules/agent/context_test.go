package agent_test

import (
	"context"
	"testing"

	"github.com/iamhectordev/hector/modules/agent"
	"github.com/iamhectordev/hector/pkg/llm/schema"
	"github.com/iamhectordev/hector/pkg/session"
	"github.com/stretchr/testify/require"
)

func TestSessionContextDelegatesToStore(t *testing.T) {
	store := &sessionStoreFake{
		history: []*schema.Message{schema.UserMessage("previous")},
	}
	agentCtx, err := agent.NewSessionContext(store, "slack://C123/1")
	require.NoError(t, err)

	history, err := agentCtx.Messages(t.Context())
	require.NoError(t, err)
	require.Equal(t, store.history, history)

	msg := schema.UserMessage("current")
	require.NoError(t, agentCtx.Record(t.Context(), []*schema.Message{msg}))
	require.Equal(t, "slack://C123/1", store.sourceURI)
	require.Equal(t, []schema.Message{{Role: schema.RoleUser, Content: "current"}}, store.recorded)
}

func TestNewSessionContextRejectsMissingInputs(t *testing.T) {
	_, err := agent.NewSessionContext(nil, "slack://C123/1")
	require.Error(t, err)

	_, err = agent.NewSessionContext(&sessionStoreFake{}, "")
	require.Error(t, err)
}

type sessionStoreFake struct {
	history   []*schema.Message
	sourceURI string
	recorded  []schema.Message
}

func (s *sessionStoreFake) GetOrCreate(_ context.Context, sourceURI string) (session.StoredSession, error) {
	return session.StoredSession{SourceURI: sourceURI}, nil
}

func (s *sessionStoreFake) Messages(_ context.Context, sourceURI string) ([]*schema.Message, error) {
	s.sourceURI = sourceURI
	return s.history, nil
}

func (s *sessionStoreFake) Record(_ context.Context, sourceURI string, messages []*schema.Message) error {
	s.sourceURI = sourceURI
	for _, msg := range messages {
		s.recorded = append(s.recorded, *msg)
	}
	return nil
}
