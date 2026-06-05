package sqlite_test

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/iamhectordev/hector/pkg/memory"
	"github.com/iamhectordev/hector/pkg/memory/sqlite"
	"github.com/iamhectordev/hector/pkg/migrations"
	"github.com/stretchr/testify/require"
)

func TestStorePutAndSearchFindsStoredFact(t *testing.T) {
	ctx := t.Context()
	store := newStore(t)

	require.NoError(t, store.Put(ctx, memory.Object{ID: "1", Content: "the auth service is written in go"}))

	results, err := store.Search(ctx, "what language is the auth service written in?", 3)
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Contains(t, results[0].Content, "go")
}

func TestStorePutAndSearchRanksRelevantFactFirst(t *testing.T) {
	ctx := t.Context()
	store := newStore(t)

	require.NoError(t, store.Put(ctx, memory.Object{ID: "1", Content: "the auth service is written in go"}))
	require.NoError(t, store.Put(ctx, memory.Object{ID: "2", Content: "the payments team uses kafka for async processing"}))
	require.NoError(t, store.Put(ctx, memory.Object{ID: "3", Content: "the data warehouse runs on bigquery"}))

	results, err := store.Search(ctx, "auth service language", 3)
	require.NoError(t, err)
	require.NotEmpty(t, results)
	require.Equal(t, "1", results[0].ID)
	require.Contains(t, results[0].Content, "go")
}

func TestStorePutUpdatesExistingObject(t *testing.T) {
	ctx := t.Context()
	store := newStore(t)

	require.NoError(t, store.Put(ctx, memory.Object{ID: "1", Content: "original content"}))
	require.NoError(t, store.Put(ctx, memory.Object{ID: "1", Content: "updated content"}))

	results, err := store.Search(ctx, "updated content", 3)
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, "updated content", results[0].Content)
}

func TestStoreSearch_PrefixMatchesLongerWords(t *testing.T) {
	ctx := t.Context()
	store := newStore(t)

	require.NoError(t, store.Put(ctx, memory.Object{ID: "1", Content: "the deployment pipeline uses github actions"}))
	require.NoError(t, store.Put(ctx, memory.Object{ID: "2", Content: "the payments team uses kafka"}))

	results, err := store.Search(ctx, "deploy", 3)
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, "1", results[0].ID)
}

func TestStoreSearch_FTSKeywordTreatedAsLiteral(t *testing.T) {
	ctx := t.Context()
	store := newStore(t)

	require.NoError(t, store.Put(ctx, memory.Object{ID: "1", Content: "NOT sure about this decision"}))

	results, err := store.Search(ctx, "NOT", 3)
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, "1", results[0].ID)
}

func TestStoreSearch_ColumnFilterSyntaxDoesNotCrash(t *testing.T) {
	ctx := t.Context()
	store := newStore(t)

	require.NoError(t, store.Put(ctx, memory.Object{ID: "1", Content: "the auth service is written in go"}))

	results, err := store.Search(ctx, "content:auth", 3)
	require.NoError(t, err)
	require.NotEmpty(t, results)
}

func TestStore_Put_StoresVecWhenEmbedderConfigured(t *testing.T) {
	ctx := t.Context()
	db := openTestDB(t)
	migrateDB(t, db)
	store := sqlite.NewStore(db, sqlite.WithEmbedder(&echoEmbedder{}))

	require.NoError(t, store.Put(ctx, memory.Object{ID: "1", Content: "the auth service uses postgres"}))

	var blob []byte
	err := db.QueryRowContext(ctx, `SELECT vec FROM memory_objects_vec WHERE id = ?`, "1").Scan(&blob)
	require.NoError(t, err)
	require.NotEmpty(t, blob)
}

func TestStore_Put_SkipsVecWhenNoEmbedder(t *testing.T) {
	ctx := t.Context()
	db := openTestDB(t)
	migrateDB(t, db)
	store := sqlite.NewStore(db)

	require.NoError(t, store.Put(ctx, memory.Object{ID: "1", Content: "the auth service uses postgres"}))

	var count int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM memory_objects_vec`).Scan(&count))
	require.Equal(t, 0, count)
}

// echoEmbedder is an in-test embedder that returns a fixed vector.
type echoEmbedder struct{}

func (e *echoEmbedder) Embed(_ context.Context, _ string) ([]float32, error) {
	return []float32{0.1, 0.2, 0.3}, nil
}

func newStore(t *testing.T) *sqlite.Store {
	t.Helper()
	db := openTestDB(t)
	migrateDB(t, db)
	return sqlite.NewStore(db)
}

func migrateDB(t *testing.T, db *sql.DB) {
	t.Helper()
	runner := migrations.New(db)
	require.NoError(t, runner.Add(sqlite.Migrations()))
	require.NoError(t, runner.Run(t.Context()))
}

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	return db
}
