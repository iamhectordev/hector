package sqlite

import (
	"context"
	"database/sql"
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/iamhectordev/hector/pkg/memory"
)

const (
	rrfK          = 60
	embedTimeout  = 1 * time.Second
	overfetchMult = 2
)

// Embedder converts text to a vector representation.
// Defined locally to avoid an import cycle between pkg/memory/sqlite and internal/embed.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

// Store persists memory objects in SQLite with FTS5 and vector search. The caller owns db.
type Store struct {
	db       *sql.DB
	embedder Embedder
}

// StoreOption configures a Store.
type StoreOption func(*Store)

// WithEmbedder sets an embedder; when set, Put stores a vector and Search runs hybrid retrieval.
func WithEmbedder(e Embedder) StoreOption {
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

// Put inserts or replaces an object and keeps the FTS and vector indexes in sync.
func (s *Store) Put(ctx context.Context, obj memory.Object) error {
	vec, err := s.embedWithTimeout(ctx, obj.Content)
	if err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("memory/sqlite: begin put: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	createdAt := obj.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT OR REPLACE INTO memory_objects (id, content, session_id, created_at) VALUES (?, ?, ?, ?)`,
		obj.ID, obj.Content, obj.SessionID, createdAt.UTC().Format(time.RFC3339),
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
			`INSERT OR REPLACE INTO memory_objects_vec (id, embedding) VALUES (?, ?)`,
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

// Search returns objects matching query. When an embedder is configured it runs FTS and
// vector search sequentially then merges results with reciprocal rank fusion.
func (s *Store) Search(ctx context.Context, query string, limit int) ([]memory.Object, error) {
	fetch := limit * overfetchMult

	ftsIDs, err := s.ftsSearch(ctx, query, fetch)
	if err != nil {
		return nil, err
	}

	var vecIDs []rankedID
	if s.embedder != nil {
		vec, embedErr := s.embedWithTimeout(ctx, query)
		if embedErr != nil {
			slog.WarnContext(ctx, "memory/sqlite: embed query failed, falling back to FTS", "err", embedErr)
		} else if vec != nil {
			vecIDs, err = s.vecSearch(ctx, vec, fetch)
			if err != nil {
				slog.WarnContext(ctx, "memory/sqlite: vec search failed, falling back to FTS", "err", err)
				vecIDs = nil
			}
		}
	}

	ids := rrf(ftsIDs, vecIDs, limit)
	if len(ids) == 0 {
		return nil, nil
	}

	objMap, err := s.fetchObjects(ctx, ids)
	if err != nil {
		return nil, err
	}

	out := make([]memory.Object, 0, len(ids))
	for _, id := range ids {
		if obj, ok := objMap[id]; ok {
			out = append(out, obj)
		}
	}
	return out, nil
}

type rankedID struct {
	id   string
	rank int // 0-based position in result list
}

func (s *Store) ftsSearch(ctx context.Context, query string, limit int) ([]rankedID, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id FROM memory_objects_fts WHERE content MATCH ? ORDER BY rank LIMIT ?`,
		sanitizeFTSQuery(query), limit,
	)
	if err != nil {
		if isNoMatchError(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("memory/sqlite: fts search: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []rankedID
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("memory/sqlite: fts scan: %w", err)
		}
		out = append(out, rankedID{id: id, rank: len(out)})
	}
	return out, rows.Err()
}

func (s *Store) vecSearch(ctx context.Context, queryVec []float32, limit int) ([]rankedID, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, distance FROM memory_objects_vec WHERE embedding MATCH ? AND k = ?`,
		float32sToBlob(queryVec), limit,
	)
	if err != nil {
		return nil, fmt.Errorf("memory/sqlite: vec search: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []rankedID
	for rows.Next() {
		var id string
		var distance float64
		if err := rows.Scan(&id, &distance); err != nil {
			return nil, fmt.Errorf("memory/sqlite: vec scan: %w", err)
		}
		out = append(out, rankedID{id: id, rank: len(out)})
	}
	return out, rows.Err()
}

func (s *Store) fetchObjects(ctx context.Context, ids []string) (map[string]memory.Object, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	placeholders := strings.Repeat("?,", len(ids))
	placeholders = placeholders[:len(placeholders)-1]

	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, content, session_id, created_at FROM memory_objects WHERE id IN (`+placeholders+`)`,
		args...,
	)
	if err != nil {
		return nil, fmt.Errorf("memory/sqlite: fetch objects: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make(map[string]memory.Object, len(ids))
	for rows.Next() {
		var obj memory.Object
		var createdAt string
		if err := rows.Scan(&obj.ID, &obj.Content, &obj.SessionID, &createdAt); err != nil {
			return nil, fmt.Errorf("memory/sqlite: fetch scan: %w", err)
		}
		if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
			obj.CreatedAt = t
		}
		out[obj.ID] = obj
	}
	return out, rows.Err()
}

func (s *Store) embedWithTimeout(ctx context.Context, content string) ([]float32, error) {
	if s.embedder == nil {
		return nil, nil
	}
	embedCtx, cancel := context.WithTimeout(ctx, embedTimeout)
	defer cancel()

	vec, err := s.embedder.Embed(embedCtx, content)
	if err != nil {
		return nil, fmt.Errorf("memory/sqlite: embed: %w", err)
	}
	return vec, nil
}

// rrf merges two ranked lists using reciprocal rank fusion and returns the top limit IDs.
func rrf(fts, vec []rankedID, limit int) []string {
	scores := make(map[string]float64)
	for _, r := range fts {
		scores[r.id] += 1.0 / float64(rrfK+r.rank+1)
	}
	for _, r := range vec {
		scores[r.id] += 1.0 / float64(rrfK+r.rank+1)
	}

	ids := make([]string, 0, len(scores))
	for id := range scores {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		if scores[ids[i]] != scores[ids[j]] {
			return scores[ids[i]] > scores[ids[j]]
		}
		return ids[i] < ids[j]
	})

	if len(ids) > limit {
		ids = ids[:limit]
	}
	return ids
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

func float32sToBlob(vec []float32) []byte {
	b := make([]byte, len(vec)*4)
	for i, f := range vec {
		binary.LittleEndian.PutUint32(b[i*4:], math.Float32bits(f))
	}
	return b
}
