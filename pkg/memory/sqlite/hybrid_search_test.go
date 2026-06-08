package sqlite_test

import (
	"strings"
	"testing"

	"github.com/iamhectordev/hector/pkg/memory"
	"github.com/stretchr/testify/require"
)

// TC-02: semantic query must surface the right result even when distractors share the same keywords.
// "our" appears in all three objects, so FTS ranks them equally. Only vector similarity can put github first.
func TestStore_Search_SemanticRecall_GitHubTopResult(t *testing.T) {
	ctx := t.Context()
	db := openTestDB(t)
	migrateDB(t, db)
	store := newStoreWithEmbedder(t, db, &fixtureEmbedder{})

	require.NoError(t, store.Put(ctx, memory.Object{ID: "1", Content: "we use github to host our code"}))
	require.NoError(t, store.Put(ctx, memory.Object{ID: "2", Content: "our team does standup every morning"}))
	require.NoError(t, store.Put(ctx, memory.Object{ID: "3", Content: "our cto presented the roadmap yesterday"}))

	results, err := store.Search(ctx, "what is our VCS?", 3)
	require.NoError(t, err)
	require.NotEmpty(t, results)
	require.Contains(t, strings.ToLower(results[0].Content), "github")
}

func TestStore_Search_HybridBoostsObjectInBothSets(t *testing.T) {
	ctx := t.Context()
	db := openTestDB(t)
	migrateDB(t, db)

	// echoEmbedder gives deterministic but not semantically meaningful vectors.
	// We control ranking by putting "auth" in content so FTS fires too.
	store := newStoreWithEmbedder(t, db, &echoEmbedder{})

	require.NoError(t, store.Put(ctx, memory.Object{ID: "a", Content: "auth service written in go"}))
	require.NoError(t, store.Put(ctx, memory.Object{ID: "b", Content: "payments team uses kafka"}))

	// "auth" matches FTS; both objects appear in vec results.
	// Object "a" should score higher because it appears in both lists.
	results, err := store.Search(ctx, "auth", 3)
	require.NoError(t, err)
	require.NotEmpty(t, results)
	require.Equal(t, "a", results[0].ID)
}

func TestStore_Search_VecErrorFallsBackToFTS(t *testing.T) {
	ctx := t.Context()
	store := newStore(t)

	require.NoError(t, store.Put(ctx, memory.Object{ID: "1", Content: "the auth service is written in go"}))

	// No embedder — FTS only path, no panic.
	results, err := store.Search(ctx, "auth service language", 3)
	require.NoError(t, err)
	require.NotEmpty(t, results)
}
