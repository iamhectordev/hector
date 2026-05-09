package waffle

import (
	"context"
	"time"
)

const (
	defaultEventListLimit = 100
	maxEventListLimit     = 1000
)

// EventQuery filters List results.
type EventQuery struct {
	// Limit caps how many events are returned. Zero uses defaultEventListLimit.
	Limit int
	// Before excludes events at or after this time. Zero means no time upper bound.
	Before time.Time
}

// NormalizeEventQuery applies default and max limits.
func NormalizeEventQuery(q EventQuery) EventQuery {
	limit := q.Limit
	if limit <= 0 {
		limit = defaultEventListLimit
	}
	if limit > maxEventListLimit {
		limit = maxEventListLimit
	}
	q.Limit = limit
	return q
}

// Reader reads events through an EventReader (typically the same Store as the bus).
type Reader struct {
	r EventReader
}

// NewReader wraps an EventReader.
func NewReader(r EventReader) *Reader {
	return &Reader{r: r}
}

// Get returns one event by id.
func (r *Reader) Get(ctx context.Context, id string) (EventRecord, error) {
	return r.r.Get(ctx, id)
}

// List returns recent events, newest first.
func (r *Reader) List(ctx context.Context, query EventQuery) ([]EventRecord, error) {
	query = NormalizeEventQuery(query)
	return r.r.List(ctx, query)
}
