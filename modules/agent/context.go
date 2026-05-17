package agent

import (
	"context"
	"fmt"

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
	return c.store.Messages(ctx, c.sourceURI)
}

func (c *SessionContext) Record(ctx context.Context, messages []*schema.Message) error {
	return c.store.Record(ctx, c.sourceURI, messages)
}
