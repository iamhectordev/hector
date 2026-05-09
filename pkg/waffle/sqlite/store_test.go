package sqlite_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/stretchr/testify/require"

	"github.com/iamhectordev/hector/pkg/migrations"
	"github.com/iamhectordev/hector/pkg/waffle"
	"github.com/iamhectordev/hector/pkg/waffle/sqlite"
)

func TestStoreAppendAndRoundTrip(t *testing.T) {
	ctx := t.Context()
	db := openTestDB(t)

	runner := migrations.New(db)
	require.NoError(t, runner.Add(sqlite.Migrations()))
	require.NoError(t, runner.Run(ctx))

	store := sqlite.NewStore(db)
	record := waffle.EventRecord{
		ID:            "evt_test",
		Type:          "test.event",
		SchemaVersion: 1,
		OccurredAt:    time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC),
		Payload:       []byte(`{"x":1}`),
	}

	require.NoError(t, store.Append(ctx, record))

	var gotID, gotType, gotOccurred string
	var gotSchema int
	var gotPayload []byte

	err := db.QueryRowContext(ctx, `
SELECT id, type, schema_version, occurred_at, payload
FROM waffle_events WHERE id = ?
`, record.ID).Scan(&gotID, &gotType, &gotSchema, &gotOccurred, &gotPayload)
	require.NoError(t, err)

	require.Equal(t, record.ID, gotID)
	require.Equal(t, record.Type, gotType)
	require.Equal(t, record.SchemaVersion, gotSchema)
	require.Equal(t, record.OccurredAt.UTC().Format(time.RFC3339Nano), gotOccurred)
	require.Equal(t, record.Payload, gotPayload)
}

func TestStoreDuplicateID(t *testing.T) {
	ctx := t.Context()
	db := openTestDB(t)

	runner := migrations.New(db)
	require.NoError(t, runner.Add(sqlite.Migrations()))
	require.NoError(t, runner.Run(ctx))

	store := sqlite.NewStore(db)
	record := waffle.EventRecord{
		ID:            "evt_dup",
		Type:          "test.event",
		SchemaVersion: 1,
		OccurredAt:    time.Now().UTC(),
		Payload:       []byte(`{}`),
	}

	require.NoError(t, store.Append(ctx, record))
	err := store.Append(ctx, record)
	require.ErrorIs(t, err, sqlite.ErrDuplicateID)
}

func TestStoreAppendCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	db := openTestDB(t)
	runner := migrations.New(db)
	require.NoError(t, runner.Add(sqlite.Migrations()))
	require.NoError(t, runner.Run(t.Context()))

	store := sqlite.NewStore(db)
	err := store.Append(ctx, waffle.EventRecord{
		ID:            "evt_cancel",
		Type:          "test.event",
		SchemaVersion: 1,
		OccurredAt:    time.Now().UTC(),
		Payload:       []byte(`{}`),
	})
	require.ErrorIs(t, err, context.Canceled)
}

func TestStoreSurvivesReopen(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "events.db")
	dsn := "file:" + filepath.ToSlash(dbPath)

	db1, err := sql.Open("sqlite", dsn)
	require.NoError(t, err)
	db1.SetMaxOpenConns(1)

	runner := migrations.New(db1)
	require.NoError(t, runner.Add(sqlite.Migrations()))
	require.NoError(t, runner.Run(t.Context()))

	store := sqlite.NewStore(db1)
	record := waffle.EventRecord{
		ID:            "evt_reopen",
		Type:          "test.event",
		SchemaVersion: 1,
		OccurredAt:    time.Now().UTC(),
		Payload:       []byte(`{"ok":true}`),
	}
	require.NoError(t, store.Append(t.Context(), record))
	require.NoError(t, db1.Close())

	db2, err := sql.Open("sqlite", dsn)
	require.NoError(t, err)
	db2.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, db2.Close()) })

	var count int
	require.NoError(t, db2.QueryRow(`SELECT COUNT(*) FROM waffle_events WHERE id = ?`, record.ID).Scan(&count))
	require.Equal(t, 1, count)
}

func TestStoreGet(t *testing.T) {
	ctx := t.Context()
	db := openTestDB(t)

	runner := migrations.New(db)
	require.NoError(t, runner.Add(sqlite.Migrations()))
	require.NoError(t, runner.Run(ctx))

	store := sqlite.NewStore(db)
	record := waffle.EventRecord{
		ID:            "evt_get",
		Type:          "test.event",
		SchemaVersion: 2,
		OccurredAt:    time.Date(2026, 6, 1, 15, 30, 0, 0, time.UTC),
		Payload:       []byte(`{"k":"v"}`),
	}
	require.NoError(t, store.Append(ctx, record))

	got, err := store.Get(ctx, record.ID)
	require.NoError(t, err)
	require.Equal(t, record.ID, got.ID)
	require.Equal(t, record.Type, got.Type)
	require.Equal(t, record.SchemaVersion, got.SchemaVersion)
	require.True(t, record.OccurredAt.Equal(got.OccurredAt))
	require.Equal(t, record.Payload, got.Payload)

	_, err = store.Get(ctx, "missing")
	require.ErrorIs(t, err, waffle.ErrEventNotFound)
}

func TestStoreList(t *testing.T) {
	ctx := t.Context()
	db := openTestDB(t)

	runner := migrations.New(db)
	require.NoError(t, runner.Add(sqlite.Migrations()))
	require.NoError(t, runner.Run(ctx))

	store := sqlite.NewStore(db)

	t1 := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)

	require.NoError(t, store.Append(ctx, waffle.EventRecord{ID: "a", Type: "t", SchemaVersion: 1, OccurredAt: t1, Payload: []byte(`1`)}))
	require.NoError(t, store.Append(ctx, waffle.EventRecord{ID: "b", Type: "t", SchemaVersion: 1, OccurredAt: t2, Payload: []byte(`2`)}))

	out, err := store.List(ctx, waffle.EventQuery{Limit: 10})
	require.NoError(t, err)
	require.Len(t, out, 2)
	require.Equal(t, "b", out[0].ID)
	require.Equal(t, "a", out[1].ID)

	out, err = store.List(ctx, waffle.EventQuery{Limit: 10, Before: t2})
	require.NoError(t, err)
	require.Len(t, out, 1)
	require.Equal(t, "a", out[0].ID)

	reader := waffle.NewReader(store)
	list, err := reader.List(ctx, waffle.EventQuery{Limit: 1})
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, "b", list[0].ID)
}

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()

	path := filepath.Join(t.TempDir(), "test.db")
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path))
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, db.Close()) })

	return db
}
