package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/iamhectordev/hector/pkg/waffle"
	moderncsqlite "modernc.org/sqlite"
	sqlitelib "modernc.org/sqlite/lib"
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
	var sqliteErr *moderncsqlite.Error
	if !errors.As(err, &sqliteErr) {
		return false
	}

	switch sqliteErr.Code() {
	case sqlitelib.SQLITE_CONSTRAINT_PRIMARYKEY,
		sqlitelib.SQLITE_CONSTRAINT_UNIQUE:
		return true
	default:
		return false
	}
}

// Get returns one event by id.
func (s *Store) Get(ctx context.Context, id string) (waffle.EventRecord, error) {
	var rec waffle.EventRecord
	var occurred string
	var payload []byte

	err := s.db.QueryRowContext(ctx, `
SELECT id, type, schema_version, occurred_at, payload
FROM waffle_events WHERE id = ?
`, id).Scan(&rec.ID, &rec.Type, &rec.SchemaVersion, &occurred, &payload)
	if errors.Is(err, sql.ErrNoRows) {
		return waffle.EventRecord{}, waffle.ErrEventNotFound
	}
	if err != nil {
		return waffle.EventRecord{}, err
	}

	rec.OccurredAt, err = time.Parse(time.RFC3339Nano, occurred)
	if err != nil {
		return waffle.EventRecord{}, fmt.Errorf("waffle/sqlite: parse occurred_at: %w", err)
	}

	rec.Payload = append([]byte(nil), payload...)
	return rec, nil
}

// List returns stored events, newest first.
func (s *Store) List(ctx context.Context, query waffle.EventQuery) ([]waffle.EventRecord, error) {
	query = waffle.NormalizeEventQuery(query)

	var rows *sql.Rows
	var err error

	if query.Before.IsZero() {
		rows, err = s.db.QueryContext(ctx, `
SELECT id, type, schema_version, occurred_at, payload
FROM waffle_events
ORDER BY occurred_at DESC
LIMIT ?
`, query.Limit)
	} else {
		before := query.Before.UTC().Format(time.RFC3339Nano)
		rows, err = s.db.QueryContext(ctx, `
SELECT id, type, schema_version, occurred_at, payload
FROM waffle_events
WHERE occurred_at < ?
ORDER BY occurred_at DESC
LIMIT ?
`, before, query.Limit)
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []waffle.EventRecord

	for rows.Next() {
		var rec waffle.EventRecord
		var occurred string
		var payload []byte

		if err := rows.Scan(&rec.ID, &rec.Type, &rec.SchemaVersion, &occurred, &payload); err != nil {
			return nil, err
		}

		rec.OccurredAt, err = time.Parse(time.RFC3339Nano, occurred)
		if err != nil {
			return nil, fmt.Errorf("waffle/sqlite: parse occurred_at: %w", err)
		}

		rec.Payload = append([]byte(nil), payload...)
		out = append(out, rec)
	}

	return out, rows.Err()
}
