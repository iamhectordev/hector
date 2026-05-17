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

func TestSessionContextRepairsOrphanedToolCall(t *testing.T) {
	store := &sessionStoreFake{
		history: []*schema.Message{
			schema.UserMessage("do something"),
			{
				Role:         schema.RoleAssistant,
				FinishReason: schema.FinishReasonToolCalls,
				ToolCalls:    []schema.ToolCall{{ID: "call_abc", Name: "search"}},
			},
		},
	}
	agentCtx, err := agent.NewSessionContext(store, "slack://C123/1")
	require.NoError(t, err)

	msgs, err := agentCtx.Messages(t.Context())
	require.NoError(t, err)
	require.Len(t, msgs, 3)
	require.Equal(t, schema.RoleTool, msgs[2].Role)
	require.Equal(t, "call_abc", msgs[2].ToolCallID)
	require.NotEmpty(t, msgs[2].Content)
}

func TestSessionContextRepairPersistsInjectedMessages(t *testing.T) {
	store := &sessionStoreFake{
		history: []*schema.Message{
			schema.UserMessage("do something"),
			{
				Role:         schema.RoleAssistant,
				FinishReason: schema.FinishReasonToolCalls,
				ToolCalls:    []schema.ToolCall{{ID: "call_abc", Name: "search"}},
			},
		},
	}
	agentCtx, err := agent.NewSessionContext(store, "slack://C123/1")
	require.NoError(t, err)

	_, err = agentCtx.Messages(t.Context())
	require.NoError(t, err)

	require.Len(t, store.recorded, 1)
	require.Equal(t, schema.RoleTool, store.recorded[0].Role)
	require.Equal(t, "call_abc", store.recorded[0].ToolCallID)
}

func TestSessionContextRepairsMultipleOrphanedToolCalls(t *testing.T) {
	store := &sessionStoreFake{
		history: []*schema.Message{
			schema.UserMessage("do something"),
			{
				Role:         schema.RoleAssistant,
				FinishReason: schema.FinishReasonToolCalls,
				ToolCalls: []schema.ToolCall{
					{ID: "call_1", Name: "search"},
					{ID: "call_2", Name: "read"},
				},
			},
		},
	}
	agentCtx, err := agent.NewSessionContext(store, "slack://C123/1")
	require.NoError(t, err)

	msgs, err := agentCtx.Messages(t.Context())
	require.NoError(t, err)
	require.Len(t, msgs, 4)
	require.Equal(t, "call_1", msgs[2].ToolCallID)
	require.Equal(t, "call_2", msgs[3].ToolCallID)
}

func TestSessionContextDoesNotRepairSatisfiedToolCalls(t *testing.T) {
	store := &sessionStoreFake{
		history: []*schema.Message{
			schema.UserMessage("do something"),
			{
				Role:         schema.RoleAssistant,
				FinishReason: schema.FinishReasonToolCalls,
				ToolCalls:    []schema.ToolCall{{ID: "call_abc", Name: "search"}},
			},
			schema.ToolResultMessage("call_abc", "result"),
			schema.AssistantMessage("done"),
		},
	}
	agentCtx, err := agent.NewSessionContext(store, "slack://C123/1")
	require.NoError(t, err)

	msgs, err := agentCtx.Messages(t.Context())
	require.NoError(t, err)
	require.Len(t, msgs, 4)
	require.Empty(t, store.recorded)
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
