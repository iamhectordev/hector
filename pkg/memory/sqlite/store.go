package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/iamhectordev/hector/pkg/memory"
)

// Store persists memory objects in SQLite with FTS5 search. The caller owns db.
type Store struct {
	db *sql.DB
}

// NewStore creates a Store. The caller owns db (open, configure, close).
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// Put inserts or replaces an object and keeps the FTS index in sync.
func (s *Store) Put(ctx context.Context, obj memory.Object) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("memory/sqlite: begin put: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx,
		`INSERT OR REPLACE INTO memory_objects (id, content) VALUES (?, ?)`,
		obj.ID, obj.Content,
	); err != nil {
		return fmt.Errorf("memory/sqlite: put object: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM memory_objects_fts WHERE id = ?`, obj.ID,
	); err != nil {
		return fmt.Errorf("memory/sqlite: delete fts: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO memory_objects_fts (id, content) VALUES (?, ?)`,
		obj.ID, obj.Content,
	); err != nil {
		return fmt.Errorf("memory/sqlite: put fts: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("memory/sqlite: commit put: %w", err)
	}
	return nil
}

// Search returns objects whose content matches query, ordered by FTS rank.
func (s *Store) Search(ctx context.Context, query string, limit int) ([]memory.Object, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, content FROM memory_objects_fts WHERE content MATCH ? ORDER BY rank LIMIT ?`,
		sanitizeFTSQuery(query), limit,
	)
	if err != nil {
		if isNoMatchError(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("memory/sqlite: search: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []memory.Object
	for rows.Next() {
		var obj memory.Object
		if err := rows.Scan(&obj.ID, &obj.Content); err != nil {
			return nil, fmt.Errorf("memory/sqlite: scan object: %w", err)
		}
		out = append(out, obj)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("memory/sqlite: search rows: %w", err)
	}
	return out, nil
}

// sanitizeFTSQuery converts a natural language string into an FTS5 OR query.
// Punctuation is stripped, tokens are joined with OR so that documents
// matching any query word are returned (ranked by how many they match).
func sanitizeFTSQuery(query string) string {
	var b strings.Builder
	for _, r := range query {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else {
			b.WriteRune(' ')
		}
	}
	tokens := strings.Fields(b.String())
	return strings.Join(tokens, " OR ")
}

// isNoMatchError reports whether err is the FTS5 "no match" error, which is
// returned when the query contains only stop words and yields no candidates.
func isNoMatchError(err error) bool {
	var sqlErr interface{ Error() string }
	if !errors.As(err, &sqlErr) || sqlErr == nil {
		return false
	}
	return sqlErr.Error() == "no match"
}
