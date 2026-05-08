package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/iamhectordev/hector/pkg/waffle"
)

// ErrDuplicateID is returned when an event with the same id already exists.
var ErrDuplicateID = errors.New("waffle/sqlite: duplicate event id")

// Store persists EventRecord rows in waffle_events.
type Store struct {
	db *sql.DB
}

// NewStore creates a Store. The caller owns db (open, configure, close).
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// Append inserts one event. Duplicate id returns ErrDuplicateID.
func (s *Store) Append(ctx context.Context, event waffle.EventRecord) error {
	payload := append([]byte(nil), event.Payload...)

	_, err := s.db.ExecContext(ctx, `
INSERT INTO waffle_events (id, type, schema_version, occurred_at, payload)
VALUES (?, ?, ?, ?, ?)
`, event.ID, event.Type, event.SchemaVersion, event.OccurredAt.UTC().Format(time.RFC3339Nano), payload)
	if err != nil {
		if isUniqueConstraint(err) {
			return fmt.Errorf("%w: %s", ErrDuplicateID, event.ID)
		}
		return fmt.Errorf("waffle/sqlite: append event: %w", err)
	}

	return nil
}

func isUniqueConstraint(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE constraint failed") || strings.Contains(msg, "constraint failed")
}
