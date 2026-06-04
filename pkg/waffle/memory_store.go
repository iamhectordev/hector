package waffle

import (
	"context"
	"sort"
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
	event.Headers = copyHeaders(event.Headers)

	s.mu.Lock()
	defer s.mu.Unlock()

	s.events = append(s.events, event)
	return nil
}

// Get returns one event by id.
func (s *MemoryStore) Get(ctx context.Context, id string) (EventRecord, error) {
	select {
	case <-ctx.Done():
		return EventRecord{}, ctx.Err()
	default:
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, event := range s.events {
		if event.ID == id {
			out := event
			out.Payload = append([]byte(nil), event.Payload...)
			out.Headers = copyHeaders(event.Headers)
			return out, nil
		}
	}

	return EventRecord{}, ErrEventNotFound
}

// List returns stored events, newest first.
func (s *MemoryStore) List(ctx context.Context, query EventQuery) ([]EventRecord, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	query = NormalizeEventQuery(query)

	s.mu.RLock()
	tmp := make([]EventRecord, len(s.events))
	for i, event := range s.events {
		tmp[i] = EventRecord{
			ID:            event.ID,
			Type:          event.Type,
			SchemaVersion: event.SchemaVersion,
			OccurredAt:    event.OccurredAt,
			Payload:       append([]byte(nil), event.Payload...),
			Headers:       copyHeaders(event.Headers),
		}
	}
	s.mu.RUnlock()

	var filtered []EventRecord
	for _, event := range tmp {
		if !query.Before.IsZero() && !event.OccurredAt.Before(query.Before) {
			continue
		}
		filtered = append(filtered, event)
	}

	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].OccurredAt.After(filtered[j].OccurredAt)
	})

	if len(filtered) > query.Limit {
		filtered = filtered[:query.Limit]
	}

	out := make([]EventRecord, len(filtered))
	copy(out, filtered)
	return out, nil
}

// Events returns a copy of the stored event records.
func (s *MemoryStore) Events() []EventRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	events := make([]EventRecord, len(s.events))
	for i, event := range s.events {
		event.Payload = append([]byte(nil), event.Payload...)
		event.Headers = copyHeaders(event.Headers)
		events[i] = event
	}

	return events
}

func copyHeaders(headers map[string]string) map[string]string {
	if len(headers) == 0 {
		return nil
	}
	out := make(map[string]string, len(headers))
	for key, value := range headers {
		out[key] = value
	}
	return out
}
