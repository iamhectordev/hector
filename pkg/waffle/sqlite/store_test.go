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

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()

	path := filepath.Join(t.TempDir(), "test.db")
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path))
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, db.Close()) })

	return db
}
