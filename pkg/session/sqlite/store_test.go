package sqlite_test

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/iamhectordev/hector/pkg/llm/schema"
	"github.com/iamhectordev/hector/pkg/migrations"
	"github.com/iamhectordev/hector/pkg/session/sqlite"
	"github.com/stretchr/testify/require"
)

func TestStoreRecordCreatesSessionAndRecordsMessages(t *testing.T) {
	ctx := t.Context()
	db := openTestDB(t)
	migrate(t, db)

	store := sqlite.NewStore(db)
	sourceURI := "slack://C123/1700000000.000100"

	require.NoError(t, store.Record(ctx, sourceURI, []*schema.Message{
		schema.UserMessage("hello"),
		schema.AssistantMessage("hi"),
	}))

	var sessionID, gotSourceURI string
	require.NoError(t, db.QueryRowContext(ctx, `
SELECT id, source_uri
FROM session_sessions
`).Scan(&sessionID, &gotSourceURI))
	require.True(t, strings.HasPrefix(sessionID, "sess_"))
	require.Equal(t, sourceURI, gotSourceURI)

	rows, err := db.QueryContext(ctx, `
SELECT role, message_json
FROM session_records
WHERE session_id = ?
ORDER BY seq ASC
`, sessionID)
	require.NoError(t, err)
	defer func() { require.NoError(t, rows.Close()) }()

	got := scanMessages(t, rows)
	require.Len(t, got, 2)
	require.Equal(t, []schema.Role{schema.RoleUser, schema.RoleAssistant}, []schema.Role{got[0].Role, got[1].Role})
	require.Equal(t, "hello", got[0].Content)
	require.Equal(t, "hi", got[1].Content)
}

func TestStoreRecordReusesSessionForSameSourceURI(t *testing.T) {
	ctx := t.Context()
	db := openTestDB(t)
	migrate(t, db)

	store := sqlite.NewStore(db)
	sourceURI := "slack://C123/1700000000.000100"

	require.NoError(t, store.Record(ctx, sourceURI, []*schema.Message{schema.UserMessage("one")}))
	require.NoError(t, store.Record(ctx, sourceURI, []*schema.Message{schema.UserMessage("two")}))

	var sessions, records int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM session_sessions`).Scan(&sessions))
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM session_records`).Scan(&records))
	require.Equal(t, 1, sessions)
	require.Equal(t, 2, records)
}

func TestStoreRecordKeepsSourceURIsIndependent(t *testing.T) {
	ctx := t.Context()
	db := openTestDB(t)
	migrate(t, db)

	store := sqlite.NewStore(db)
	require.NoError(t, store.Record(ctx, "slack://C123/1", []*schema.Message{schema.UserMessage("one")}))
	require.NoError(t, store.Record(ctx, "slack://C999/1", []*schema.Message{schema.UserMessage("two")}))

	rows, err := db.QueryContext(ctx, `
SELECT s.source_uri, COUNT(r.id)
FROM session_sessions s
JOIN session_records r ON r.session_id = s.id
GROUP BY s.source_uri
ORDER BY s.source_uri
`)
	require.NoError(t, err)
	defer func() { require.NoError(t, rows.Close()) }()

	got := map[string]int{}
	for rows.Next() {
		var sourceURI string
		var count int
		require.NoError(t, rows.Scan(&sourceURI, &count))
		got[sourceURI] = count
	}
	require.NoError(t, rows.Err())
	require.Equal(t, map[string]int{
		"slack://C123/1": 1,
		"slack://C999/1": 1,
	}, got)
}

func TestStoreMessagesReturnsRecordedMessagesInOrder(t *testing.T) {
	ctx := t.Context()
	db := openTestDB(t)
	migrate(t, db)

	store := sqlite.NewStore(db)
	require.NoError(t, store.Record(ctx, "slack://C123/1", []*schema.Message{
		schema.UserMessage("one"),
		schema.AssistantMessage("two"),
	}))
	require.NoError(t, store.Record(ctx, "slack://C999/1", []*schema.Message{
		schema.UserMessage("other"),
	}))

	got, err := store.Messages(ctx, "slack://C123/1")
	require.NoError(t, err)
	require.Equal(t, []*schema.Message{
		schema.UserMessage("one"),
		schema.AssistantMessage("two"),
	}, got)
}

func TestStoreMessagesSurvivesReopen(t *testing.T) {
	ctx := t.Context()
	path := filepath.Join(t.TempDir(), "session.db")
	dsn := "file:" + filepath.ToSlash(path)

	db1, err := sql.Open("sqlite", dsn)
	require.NoError(t, err)
	db1.SetMaxOpenConns(1)
	migrate(t, db1)

	store1 := sqlite.NewStore(db1)
	require.NoError(t, store1.Record(ctx, "slack://C123/1", []*schema.Message{
		schema.UserMessage("before restart"),
		schema.AssistantMessage("persisted"),
	}))
	require.NoError(t, db1.Close())

	db2, err := sql.Open("sqlite", dsn)
	require.NoError(t, err)
	db2.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, db2.Close()) })

	store2 := sqlite.NewStore(db2)
	got, err := store2.Messages(ctx, "slack://C123/1")
	require.NoError(t, err)
	require.Equal(t, []*schema.Message{
		schema.UserMessage("before restart"),
		schema.AssistantMessage("persisted"),
	}, got)
}

func TestStoreRecordPersistsFullMessageJSON(t *testing.T) {
	ctx := t.Context()
	db := openTestDB(t)
	migrate(t, db)

	store := sqlite.NewStore(db)
	msg := &schema.Message{
		Role:         schema.RoleAssistant,
		Content:      "calling tool",
		FinishReason: schema.FinishReasonToolCalls,
		ToolCalls: []schema.ToolCall{{
			ID:        "call_1",
			Name:      "time_now",
			Arguments: json.RawMessage(`{"timezone":"UTC"}`),
		}},
	}

	require.NoError(t, store.Record(ctx, "slack://C123/1", []*schema.Message{msg}))

	var raw string
	require.NoError(t, db.QueryRowContext(ctx, `
SELECT message_json
FROM session_records
`).Scan(&raw))

	var got schema.Message
	require.NoError(t, json.Unmarshal([]byte(raw), &got))
	require.Equal(t, *msg, got)
}

func TestStoreRecordOmitsEmptyMessageJSONFields(t *testing.T) {
	ctx := t.Context()
	db := openTestDB(t)
	migrate(t, db)

	store := sqlite.NewStore(db)
	require.NoError(t, store.Record(ctx, "slack://C123/1", []*schema.Message{
		schema.UserMessage("hello"),
	}))

	var raw string
	require.NoError(t, db.QueryRowContext(ctx, `
SELECT message_json
FROM session_records
`).Scan(&raw))
	require.JSONEq(t, `{"Role":"user","Content":"hello"}`, raw)
}

func TestStoreGetOrCreateReturnsExistingSession(t *testing.T) {
	ctx := t.Context()
	db := openTestDB(t)
	migrate(t, db)

	store := sqlite.NewStore(db)
	first, err := store.GetOrCreate(ctx, "slack://C123/1")
	require.NoError(t, err)
	second, err := store.GetOrCreate(ctx, "slack://C123/1")
	require.NoError(t, err)

	require.Equal(t, first.ID, second.ID)
	require.Equal(t, first.SourceURI, second.SourceURI)
	require.True(t, first.CreatedAt.Equal(second.CreatedAt))
}

func TestStoreRecordRejectsBlankSourceURI(t *testing.T) {
	db := openTestDB(t)
	migrate(t, db)

	store := sqlite.NewStore(db)
	err := store.Record(t.Context(), "", []*schema.Message{schema.UserMessage("hello")})
	require.Error(t, err)
}

func TestStoreRecordRejectsNilMessage(t *testing.T) {
	db := openTestDB(t)
	migrate(t, db)

	store := sqlite.NewStore(db)
	err := store.Record(t.Context(), "slack://C123/1", []*schema.Message{nil})
	require.Error(t, err)
}

func scanMessages(t *testing.T, rows *sql.Rows) []schema.Message {
	t.Helper()

	var out []schema.Message
	for rows.Next() {
		var role string
		var raw string
		require.NoError(t, rows.Scan(&role, &raw))

		var msg schema.Message
		require.NoError(t, json.Unmarshal([]byte(raw), &msg))
		require.Equal(t, string(msg.Role), role)
		out = append(out, msg)
	}
	require.NoError(t, rows.Err())
	return out
}

func migrate(t *testing.T, db *sql.DB) {
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
