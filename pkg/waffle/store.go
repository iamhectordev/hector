package waffle

import (
	"context"
	"errors"
	"time"
)

// ErrEventNotFound is returned when no stored event matches the id.
var ErrEventNotFound = errors.New("waffle: event not found")

// ErrReactionNotFound is returned when no stored reaction matches the id.
var ErrReactionNotFound = errors.New("waffle: reaction not found")

// EventWriter appends recorded events.
type EventWriter interface {
	Append(ctx context.Context, event EventRecord) error
}

// EventReader reads stored events.
type EventReader interface {
	Get(ctx context.Context, id string) (EventRecord, error)
	List(ctx context.Context, query EventQuery) ([]EventRecord, error)
}

// Store persists and reads event records.
type Store interface {
	EventWriter
	EventReader
}

// ReactionStatus is the durable state of a handler reaction.
type ReactionStatus string

const (
	ReactionPending   ReactionStatus = "pending"
	ReactionRunning   ReactionStatus = "running"
	ReactionSucceeded ReactionStatus = "succeeded"
	ReactionFailed    ReactionStatus = "failed"
)

// ReactionRecord is the storage form of a handler reaction to an event.
type ReactionRecord struct {
	ID          string
	EventID     string
	HandlerName string
	Status      ReactionStatus
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ReactionStore persists and reads durable handler reactions.
type ReactionStore interface {
	EventReader

	AppendReactions(ctx context.Context, reactions []ReactionRecord) error
	RecordEventReactions(ctx context.Context, event EventRecord, reactions []ReactionRecord) error
	ListPendingReactions(ctx context.Context, limit int) ([]ReactionRecord, error)
	ResetRunningReactions(ctx context.Context) error
	ClaimReaction(ctx context.Context, id string) (bool, error)
	MarkReactionSucceeded(ctx context.Context, id string) error
	MarkReactionFailed(ctx context.Context, id string) error
}

// EventRecord is the storage form of an event.
type EventRecord struct {
	ID            string
	Type          string
	SchemaVersion int
	OccurredAt    time.Time
	Payload       []byte
	Headers       map[string]string
}
