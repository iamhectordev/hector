package waffle

import (
	"context"
	"errors"
	"time"
)

// ErrEventNotFound is returned when no stored event matches the id.
var ErrEventNotFound = errors.New("waffle: event not found")

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

// EventRecord is the storage form of an event.
type EventRecord struct {
	ID            string
	Type          string
	SchemaVersion int
	OccurredAt    time.Time
	Payload       []byte
}
