package sqlite

import (
	"context"
	"database/sql"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"strings"
	"unicode"

	"github.com/iamhectordev/hector/pkg/memory"
)

// embedder is the subset of embed.Embedder used by the store.
// Defined locally to avoid an import cycle between pkg/memory/sqlite and internal/embed.
type embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

// Store persists memory objects in SQLite with FTS5 search. The caller owns db.
type Store struct {
	db       *sql.DB
	embedder embedder
}

// StoreOption configures a Store.
type StoreOption func(*Store)

// WithEmbedder sets an embedder; when set, Put stores a vector alongside each object.
func WithEmbedder(e embedder) StoreOption {
	return func(s *Store) { s.embedder = e }
}

// NewStore creates a Store. The caller owns db (open, configure, close).
func NewStore(db *sql.DB, opts ...StoreOption) *Store {
	s := &Store{db: db}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Put inserts or replaces an object and keeps the FTS index in sync.
// If an embedder is configured, it also stores the embedding vector.
func (s *Store) Put(ctx context.Context, obj memory.Object) error {
	vec, err := s.embed(ctx, obj.Content)
	if err != nil {
		return err
	}

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

	if vec != nil {
		if _, err := tx.ExecContext(ctx,
			`INSERT OR REPLACE INTO memory_objects_vec (id, vec) VALUES (?, ?)`,
			obj.ID, float32sToBlob(vec),
		); err != nil {
			return fmt.Errorf("memory/sqlite: put vec: %w", err)
		}
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

func (s *Store) embed(ctx context.Context, content string) ([]float32, error) {
	if s.embedder == nil {
		return nil, nil
	}
	vec, err := s.embedder.Embed(ctx, content)
	if err != nil {
		return nil, fmt.Errorf("memory/sqlite: embed: %w", err)
	}
	return vec, nil
}

func float32sToBlob(vec []float32) []byte {
	b := make([]byte, len(vec)*4)
	for i, f := range vec {
		binary.LittleEndian.PutUint32(b[i*4:], math.Float32bits(f))
	}
	return b
}

// sanitizeFTSQuery converts a natural language string into an FTS5 OR query.
// Each token is quoted and suffixed with * so that:
//   - FTS5 keywords (AND, OR, NOT) are treated as literals, not operators
//   - Column filter syntax (col:term) is neutralised — the colon becomes a space
//   - Prefix matching is enabled: "deploy" matches "deployment"
//   - Results are ranked by how many tokens match (more matches = higher rank)
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
	quoted := make([]string, len(tokens))
	for i, tok := range tokens {
		quoted[i] = `"` + tok + `"*`
	}
	return strings.Join(quoted, " OR ")
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
