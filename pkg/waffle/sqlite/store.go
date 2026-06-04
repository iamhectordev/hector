package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
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
	headers, err := encodeHeaders(event.Headers)
	if err != nil {
		return err
	}

	err = insertEvent(ctx, s.db, event, payload, headers)
	if err != nil {
		return err
	}

	return nil
}

type sqlExecer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func insertEvent(ctx context.Context, exec sqlExecer, event waffle.EventRecord, payload, headers []byte) error {
	_, err := exec.ExecContext(ctx, `
INSERT INTO waffle_events (id, type, schema_version, occurred_at, payload, headers)
VALUES (?, ?, ?, ?, ?, ?)
`, event.ID, event.Type, event.SchemaVersion, event.OccurredAt.UTC().Format(time.RFC3339Nano), payload, headers)
	if err != nil {
		if isUniqueConstraint(err) {
			return fmt.Errorf("%w: %s", ErrDuplicateID, event.ID)
		}
		return fmt.Errorf("waffle/sqlite: append event: %w", err)
	}

	return nil
}

// AppendReactions inserts durable handler reactions. Existing event/handler pairs are ignored.
func (s *Store) AppendReactions(ctx context.Context, reactions []waffle.ReactionRecord) error {
	for _, reaction := range reactions {
		if err := insertReaction(ctx, s.db, reaction); err != nil {
			return err
		}
	}
	return nil
}

// RecordEventReactions stores an event and its reactions atomically.
func (s *Store) RecordEventReactions(ctx context.Context, event waffle.EventRecord, reactions []waffle.ReactionRecord) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("waffle/sqlite: begin event reactions: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	payload := append([]byte(nil), event.Payload...)
	headers, err := encodeHeaders(event.Headers)
	if err != nil {
		return err
	}
	if err := insertEvent(ctx, tx, event, payload, headers); err != nil {
		return err
	}

	for _, reaction := range reactions {
		if err := insertReaction(ctx, tx, reaction); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("waffle/sqlite: commit event reactions: %w", err)
	}
	return nil
}

func insertReaction(ctx context.Context, exec sqlExecer, reaction waffle.ReactionRecord) error {
	_, err := exec.ExecContext(ctx, `
INSERT OR IGNORE INTO waffle_reactions (id, event_id, handler_name, status, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?)
`, reaction.ID,
		reaction.EventID,
		reaction.HandlerName,
		string(reaction.Status),
		reaction.CreatedAt.UTC().Format(time.RFC3339Nano),
		reaction.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("waffle/sqlite: append reaction: %w", err)
	}
	return nil
}

// ListPendingReactions returns pending reactions in creation order.
func (s *Store) ListPendingReactions(ctx context.Context, limit int) ([]waffle.ReactionRecord, error) {
	if limit <= 0 {
		limit = 100
	}

	rows, err := s.db.QueryContext(ctx, `
SELECT id, event_id, handler_name, status, created_at, updated_at
FROM waffle_reactions
WHERE status = ?
ORDER BY created_at ASC
LIMIT ?
`, string(waffle.ReactionPending), limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []waffle.ReactionRecord
	for rows.Next() {
		reaction, err := scanReaction(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, reaction)
	}
	return out, rows.Err()
}

type reactionScanner interface {
	Scan(...any) error
}

func scanReaction(row reactionScanner) (waffle.ReactionRecord, error) {
	var reaction waffle.ReactionRecord
	var status, createdAt, updatedAt string

	if err := row.Scan(&reaction.ID, &reaction.EventID, &reaction.HandlerName, &status, &createdAt, &updatedAt); err != nil {
		return waffle.ReactionRecord{}, err
	}

	reaction.Status = waffle.ReactionStatus(status)
	var err error
	reaction.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return waffle.ReactionRecord{}, fmt.Errorf("waffle/sqlite: parse reaction created_at: %w", err)
	}
	reaction.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return waffle.ReactionRecord{}, fmt.Errorf("waffle/sqlite: parse reaction updated_at: %w", err)
	}
	return reaction, nil
}

// ResetRunningReactions returns in-flight reactions to pending for restart safety.
func (s *Store) ResetRunningReactions(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE waffle_reactions
SET status = ?, updated_at = ?
WHERE status = ?
`, string(waffle.ReactionPending), time.Now().UTC().Format(time.RFC3339Nano), string(waffle.ReactionRunning))
	if err != nil {
		return fmt.Errorf("waffle/sqlite: reset running reactions: %w", err)
	}
	return nil
}

// ClaimReaction moves a pending reaction to running.
func (s *Store) ClaimReaction(ctx context.Context, id string) (bool, error) {
	res, err := s.db.ExecContext(ctx, `
UPDATE waffle_reactions
SET status = ?, updated_at = ?
WHERE id = ? AND status = ?
`, string(waffle.ReactionRunning), time.Now().UTC().Format(time.RFC3339Nano), id, string(waffle.ReactionPending))
	if err != nil {
		return false, fmt.Errorf("waffle/sqlite: claim reaction: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("waffle/sqlite: claim reaction rows affected: %w", err)
	}
	return affected == 1, nil
}

// MarkReactionSucceeded marks a reaction as successfully handled.
func (s *Store) MarkReactionSucceeded(ctx context.Context, id string) error {
	return s.markReaction(ctx, id, waffle.ReactionSucceeded)
}

// MarkReactionFailed marks a reaction as failed.
func (s *Store) MarkReactionFailed(ctx context.Context, id string) error {
	return s.markReaction(ctx, id, waffle.ReactionFailed)
}

func (s *Store) markReaction(ctx context.Context, id string, status waffle.ReactionStatus) error {
	res, err := s.db.ExecContext(ctx, `
UPDATE waffle_reactions
SET status = ?, updated_at = ?
WHERE id = ?
`, string(status), time.Now().UTC().Format(time.RFC3339Nano), id)
	if err != nil {
		return fmt.Errorf("waffle/sqlite: mark reaction %s: %w", status, err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("waffle/sqlite: mark reaction %s rows affected: %w", status, err)
	}
	if affected == 0 {
		return fmt.Errorf("%w: %s", waffle.ErrReactionNotFound, id)
	}
	if affected != 1 {
		return fmt.Errorf("waffle/sqlite: mark reaction %s affected %d rows", status, affected)
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
	var payload, headers []byte

	err := s.db.QueryRowContext(ctx, `
SELECT id, type, schema_version, occurred_at, payload, headers
FROM waffle_events WHERE id = ?
`, id).Scan(&rec.ID, &rec.Type, &rec.SchemaVersion, &occurred, &payload, &headers)
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
	rec.Headers, err = decodeHeaders(headers)
	if err != nil {
		return waffle.EventRecord{}, err
	}
	return rec, nil
}

// List returns stored events, newest first.
func (s *Store) List(ctx context.Context, query waffle.EventQuery) ([]waffle.EventRecord, error) {
	query = waffle.NormalizeEventQuery(query)

	var rows *sql.Rows
	var err error

	if query.Before.IsZero() {
		rows, err = s.db.QueryContext(ctx, `
SELECT id, type, schema_version, occurred_at, payload, headers
FROM waffle_events
ORDER BY occurred_at DESC
LIMIT ?
`, query.Limit)
	} else {
		before := query.Before.UTC().Format(time.RFC3339Nano)
		rows, err = s.db.QueryContext(ctx, `
SELECT id, type, schema_version, occurred_at, payload, headers
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
		var payload, headers []byte

		if err := rows.Scan(&rec.ID, &rec.Type, &rec.SchemaVersion, &occurred, &payload, &headers); err != nil {
			return nil, err
		}

		rec.OccurredAt, err = time.Parse(time.RFC3339Nano, occurred)
		if err != nil {
			return nil, fmt.Errorf("waffle/sqlite: parse occurred_at: %w", err)
		}

		rec.Payload = append([]byte(nil), payload...)
		rec.Headers, err = decodeHeaders(headers)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}

	return out, rows.Err()
}

func encodeHeaders(headers map[string]string) ([]byte, error) {
	if len(headers) == 0 {
		return []byte(`{}`), nil
	}
	out, err := json.Marshal(headers)
	if err != nil {
		return nil, fmt.Errorf("waffle/sqlite: encode headers: %w", err)
	}
	return out, nil
}

func decodeHeaders(headers []byte) (map[string]string, error) {
	if len(headers) == 0 {
		return nil, nil
	}
	var out map[string]string
	if err := json.Unmarshal(headers, &out); err != nil {
		return nil, fmt.Errorf("waffle/sqlite: decode headers: %w", err)
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}
