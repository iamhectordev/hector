package agent

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/iamhectordev/hector/pkg/llm/schema"
	"github.com/iamhectordev/hector/pkg/session"
)

// Context provides transcript state for one agent turn.
type Context interface {
	Messages(ctx context.Context) ([]*schema.Message, error)
	Record(ctx context.Context, messages []*schema.Message) error
}

// SessionContext backs an agent turn with a session store and source URI.
type SessionContext struct {
	store     session.Store
	sourceURI string
}

func NewSessionContext(store session.Store, sourceURI string) (*SessionContext, error) {
	if store == nil {
		return nil, fmt.Errorf("agent: session store is required")
	}
	if sourceURI == "" {
		return nil, fmt.Errorf("agent: source URI is required")
	}
	return &SessionContext{store: store, sourceURI: sourceURI}, nil
}

func (c *SessionContext) Messages(ctx context.Context) ([]*schema.Message, error) {
	messages, err := c.store.Messages(ctx, c.sourceURI)
	if err != nil {
		return nil, err
	}
	repaired, injected := repairHistory(messages)
	if len(injected) > 0 {
		if err := c.store.Record(ctx, c.sourceURI, injected); err != nil {
			slog.WarnContext(ctx, "agent: failed to persist repaired session history", "err", err)
		}
	}
	return repaired, nil
}

func (c *SessionContext) Session(ctx context.Context) (session.Session, error) {
	stored, err := c.store.GetOrCreate(ctx, c.sourceURI)
	if err != nil {
		return session.Session{}, err
	}
	return session.Session{
		ID:        stored.ID,
		SourceURI: stored.SourceURI,
	}, nil
}

const interruptedToolResult = "interrupted: the process restarted before this tool call completed — please retry if needed"

func repairHistory(messages []*schema.Message) (repaired []*schema.Message, injected []*schema.Message) {
	repaired = make([]*schema.Message, 0, len(messages))
	for i, msg := range messages {
		repaired = append(repaired, msg)
		if msg.Role != schema.RoleAssistant || len(msg.ToolCalls) == 0 {
			continue
		}
		satisfied := make(map[string]bool)
		for _, next := range messages[i+1:] {
			if next.Role == schema.RoleTool {
				satisfied[next.ToolCallID] = true
			}
		}
		for _, call := range msg.ToolCalls {
			if satisfied[call.ID] {
				continue
			}
			synthetic := schema.ToolResultMessage(call.ID, interruptedToolResult)
			repaired = append(repaired, synthetic)
			injected = append(injected, synthetic)
		}
	}
	return repaired, injected
}

func (c *SessionContext) Record(ctx context.Context, messages []*schema.Message) error {
	return c.store.Record(ctx, c.sourceURI, messages)
}
