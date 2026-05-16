package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/iamhectordev/hector/pkg/llm/schema"
	"github.com/iamhectordev/hector/pkg/session"
	"github.com/oklog/ulid/v2"
)

// Store persists session records in SQLite. The caller owns db.
type Store struct {
	db *sql.DB
}

// NewStore creates a Store. The caller owns db (open, configure, close).
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// GetOrCreate returns the session for sourceURI, creating it when absent.
func (s *Store) GetOrCreate(ctx context.Context, sourceURI string) (session.StoredSession, error) {
	if strings.TrimSpace(sourceURI) == "" {
		return session.StoredSession{}, fmt.Errorf("session/sqlite: source URI is required")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return session.StoredSession{}, fmt.Errorf("session/sqlite: begin get or create session: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stored, err := getOrCreateSession(ctx, tx, sourceURI)
	if err != nil {
		return session.StoredSession{}, err
	}

	if err := tx.Commit(); err != nil {
		return session.StoredSession{}, fmt.Errorf("session/sqlite: commit get or create session: %w", err)
	}
	return stored, nil
}

// Record appends messages to the transcript for sourceURI.
func (s *Store) Record(ctx context.Context, sourceURI string, messages []*schema.Message) error {
	if len(messages) == 0 {
		return nil
	}
	if strings.TrimSpace(sourceURI) == "" {
		return fmt.Errorf("session/sqlite: source URI is required")
	}
	for _, msg := range messages {
		if msg == nil {
			return fmt.Errorf("session/sqlite: record nil message")
		}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("session/sqlite: begin record session: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stored, err := getOrCreateSession(ctx, tx, sourceURI)
	if err != nil {
		return err
	}

	for _, msg := range messages {
		if err := insertRecord(ctx, tx, stored.ID, msg); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("session/sqlite: commit record session: %w", err)
	}
	return nil
}

type sqlSessionStore interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func getOrCreateSession(ctx context.Context, db sqlSessionStore, sourceURI string) (session.StoredSession, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := db.ExecContext(ctx, `
INSERT OR IGNORE INTO session_sessions (id, source_uri, created_at)
VALUES (?, ?, ?)
`, newID("sess"), sourceURI, now)
	if err != nil {
		return session.StoredSession{}, fmt.Errorf("session/sqlite: create session: %w", err)
	}

	var stored session.StoredSession
	var createdAt string
	err = db.QueryRowContext(ctx, `
SELECT id, source_uri, created_at
FROM session_sessions
WHERE source_uri = ?
`, sourceURI).Scan(&stored.ID, &stored.SourceURI, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return session.StoredSession{}, fmt.Errorf("session/sqlite: session not found after create")
	}
	if err != nil {
		return session.StoredSession{}, fmt.Errorf("session/sqlite: get session: %w", err)
	}

	stored.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return session.StoredSession{}, fmt.Errorf("session/sqlite: parse session created_at: %w", err)
	}
	return stored, nil
}

func insertRecord(ctx context.Context, db sqlSessionStore, sessionID string, msg *schema.Message) error {
	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("session/sqlite: marshal message: %w", err)
	}

	_, err = db.ExecContext(ctx, `
INSERT INTO session_records (id, session_id, role, message_json, created_at)
VALUES (?, ?, ?, ?, ?)
`, newID("rec"), sessionID, string(msg.Role), string(payload), time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("session/sqlite: insert record: %w", err)
	}
	return nil
}

func newID(prefix string) string {
	return prefix + "_" + ulid.Make().String()
}
