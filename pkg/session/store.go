package session

import (
	"context"
	"time"

	"github.com/iamhectordev/hector/pkg/llm/schema"
)

// Store persists the model-facing transcript for a source session.
type Store interface {
	GetOrCreate(ctx context.Context, sourceURI string) (StoredSession, error)
	Messages(ctx context.Context, sourceURI string) ([]*schema.Message, error)
	Record(ctx context.Context, sourceURI string, messages []*schema.Message) error
}

// StoredSession is a persisted session row.
type StoredSession struct {
	ID        string
	SourceURI string
	CreatedAt time.Time
}
