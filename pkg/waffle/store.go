package waffle

import (
	"context"
	"time"
)

// Store persists recorded events.
type Store interface {
	Append(ctx context.Context, event EventRecord) error
}

// EventRecord is the storage form of an event.
type EventRecord struct {
	ID            string
	Type          string
	SchemaVersion int
	OccurredAt    time.Time
	Payload       []byte
}
