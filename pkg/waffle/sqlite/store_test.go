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
		Headers: map[string]string{
			"traceparent": "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
			"baggage":     "session.id=sess_123",
		},
	}
	require.NoError(t, store.Append(ctx, record))

	got, err := store.Get(ctx, record.ID)
	require.NoError(t, err)
	require.Equal(t, record.ID, got.ID)
	require.Equal(t, record.Type, got.Type)
	require.Equal(t, record.SchemaVersion, got.SchemaVersion)
	require.True(t, record.OccurredAt.Equal(got.OccurredAt))
	require.Equal(t, record.Payload, got.Payload)
	require.Equal(t, record.Headers, got.Headers)

	_, err = store.Get(ctx, "missing")
	require.ErrorIs(t, err, waffle.ErrEventNotFound)
}

func TestStorePersistsHeadersSeparatelyFromPayload(t *testing.T) {
	ctx := t.Context()
	db := openTestDB(t)

	runner := migrations.New(db)
	require.NoError(t, runner.Add(sqlite.Migrations()))
	require.NoError(t, runner.Run(ctx))

	store := sqlite.NewStore(db)
	record := waffle.EventRecord{
		ID:            "evt_headers",
		Type:          "test.event",
		SchemaVersion: 1,
		OccurredAt:    time.Now().UTC(),
		Payload:       []byte(`{"message":"hello"}`),
		Headers: map[string]string{
			"traceparent": "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
			"tracestate":  "vendor=value",
			"baggage":     "session.id=sess_123",
		},
	}

	require.NoError(t, store.Append(ctx, record))

	var payload, headers []byte
	require.NoError(t, db.QueryRowContext(ctx, `
SELECT payload, headers FROM waffle_events WHERE id = ?
`, record.ID).Scan(&payload, &headers))
	require.JSONEq(t, `{"message":"hello"}`, string(payload))
	require.JSONEq(t, `{
	"traceparent": "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
	"tracestate": "vendor=value",
	"baggage": "session.id=sess_123"
}`, string(headers))
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

func TestStoreReactions(t *testing.T) {
	ctx := t.Context()
	db := openTestDB(t)

	runner := migrations.New(db)
	require.NoError(t, runner.Add(sqlite.Migrations()))
	require.NoError(t, runner.Run(ctx))

	store := sqlite.NewStore(db)
	event := waffle.EventRecord{
		ID:            "evt_reactions",
		Type:          "test.event",
		SchemaVersion: 1,
		OccurredAt:    time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC),
		Payload:       []byte(`{}`),
	}
	reaction := waffle.ReactionRecord{
		ID:          "rxn_test",
		EventID:     event.ID,
		HandlerName: "test.handler",
		Status:      waffle.ReactionPending,
		CreatedAt:   time.Date(2026, 5, 15, 12, 1, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 5, 15, 12, 1, 0, 0, time.UTC),
	}

	require.NoError(t, store.RecordEventReactions(ctx, event, []waffle.ReactionRecord{reaction}))

	pending, err := store.ListPendingReactions(ctx, 10)
	require.NoError(t, err)
	require.Len(t, pending, 1)
	require.Equal(t, reaction.ID, pending[0].ID)
	require.Equal(t, reaction.EventID, pending[0].EventID)
	require.Equal(t, reaction.HandlerName, pending[0].HandlerName)
	require.Equal(t, waffle.ReactionPending, pending[0].Status)

	require.NoError(t, store.MarkReactionSucceeded(ctx, reaction.ID))

	pending, err = store.ListPendingReactions(ctx, 10)
	require.NoError(t, err)
	require.Empty(t, pending)

	var status string
	require.NoError(t, db.QueryRowContext(ctx, `SELECT status FROM waffle_reactions WHERE id = ?`, reaction.ID).Scan(&status))
	require.Equal(t, string(waffle.ReactionSucceeded), status)
}

func TestStoreClaimReaction(t *testing.T) {
	ctx := t.Context()
	db := openTestDB(t)

	runner := migrations.New(db)
	require.NoError(t, runner.Add(sqlite.Migrations()))
	require.NoError(t, runner.Run(ctx))

	store := sqlite.NewStore(db)
	event := waffle.EventRecord{
		ID:            "evt_claim",
		Type:          "test.event",
		SchemaVersion: 1,
		OccurredAt:    time.Now().UTC(),
		Payload:       []byte(`{}`),
	}
	reaction := waffle.ReactionRecord{
		ID:          "rxn_claim",
		EventID:     event.ID,
		HandlerName: "test.handler",
		Status:      waffle.ReactionPending,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	require.NoError(t, store.RecordEventReactions(ctx, event, []waffle.ReactionRecord{reaction}))

	claimed, err := store.ClaimReaction(ctx, reaction.ID)
	require.NoError(t, err)
	require.True(t, claimed)

	claimed, err = store.ClaimReaction(ctx, reaction.ID)
	require.NoError(t, err)
	require.False(t, claimed)

	var status string
	require.NoError(t, db.QueryRowContext(ctx, `SELECT status FROM waffle_reactions WHERE id = ?`, reaction.ID).Scan(&status))
	require.Equal(t, string(waffle.ReactionRunning), status)
}

func TestStoreResetRunningReactions(t *testing.T) {
	ctx := t.Context()
	db := openTestDB(t)

	runner := migrations.New(db)
	require.NoError(t, runner.Add(sqlite.Migrations()))
	require.NoError(t, runner.Run(ctx))

	store := sqlite.NewStore(db)
	event := waffle.EventRecord{
		ID:            "evt_reset_running",
		Type:          "test.event",
		SchemaVersion: 1,
		OccurredAt:    time.Now().UTC(),
		Payload:       []byte(`{}`),
	}
	reaction := waffle.ReactionRecord{
		ID:          "rxn_reset_running",
		EventID:     event.ID,
		HandlerName: "test.handler",
		Status:      waffle.ReactionRunning,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	require.NoError(t, store.RecordEventReactions(ctx, event, []waffle.ReactionRecord{reaction}))

	require.NoError(t, store.ResetRunningReactions(ctx))

	var status string
	require.NoError(t, db.QueryRowContext(ctx, `SELECT status FROM waffle_reactions WHERE id = ?`, reaction.ID).Scan(&status))
	require.Equal(t, string(waffle.ReactionPending), status)
}

func TestStoreMarkReactionMissingID(t *testing.T) {
	ctx := t.Context()
	db := openTestDB(t)

	runner := migrations.New(db)
	require.NoError(t, runner.Add(sqlite.Migrations()))
	require.NoError(t, runner.Run(ctx))

	store := sqlite.NewStore(db)
	require.ErrorIs(t, store.MarkReactionSucceeded(ctx, "missing"), waffle.ErrReactionNotFound)
	require.ErrorIs(t, store.MarkReactionFailed(ctx, "missing"), waffle.ErrReactionNotFound)
}

func TestStoreAppendReactionsIsIdempotent(t *testing.T) {
	ctx := t.Context()
	db := openTestDB(t)

	runner := migrations.New(db)
	require.NoError(t, runner.Add(sqlite.Migrations()))
	require.NoError(t, runner.Run(ctx))

	store := sqlite.NewStore(db)
	event := waffle.EventRecord{
		ID:            "evt_reaction_idempotent",
		Type:          "test.event",
		SchemaVersion: 1,
		OccurredAt:    time.Now().UTC(),
		Payload:       []byte(`{}`),
	}
	require.NoError(t, store.Append(ctx, event))

	reaction := waffle.ReactionRecord{
		ID:          "rxn_once",
		EventID:     event.ID,
		HandlerName: "test.handler",
		Status:      waffle.ReactionPending,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	require.NoError(t, store.AppendReactions(ctx, []waffle.ReactionRecord{reaction}))
	reaction.ID = "rxn_duplicate"
	require.NoError(t, store.AppendReactions(ctx, []waffle.ReactionRecord{reaction}))

	var count int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM waffle_reactions WHERE event_id = ? AND handler_name = ?`, event.ID, reaction.HandlerName).Scan(&count))
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
