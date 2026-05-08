package waffle

import (
	"context"
	"sync"
)

// MemoryStore keeps event records in process memory.
type MemoryStore struct {
	mu     sync.RWMutex
	events []EventRecord
}

// NewMemoryStore creates an empty in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{}
}

// Append stores an event record.
func (s *MemoryStore) Append(ctx context.Context, event EventRecord) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	event.Payload = append([]byte(nil), event.Payload...)

	s.mu.Lock()
	defer s.mu.Unlock()

	s.events = append(s.events, event)
	return nil
}

// Events returns a copy of the stored event records.
func (s *MemoryStore) Events() []EventRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	events := make([]EventRecord, len(s.events))
	for i, event := range s.events {
		event.Payload = append([]byte(nil), event.Payload...)
		events[i] = event
	}

	return events
}
