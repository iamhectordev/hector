package waffle_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/stretchr/testify/require"

	"github.com/iamhectordev/hector/pkg/migrations"
	"github.com/iamhectordev/hector/pkg/waffle"
	wafflesqlite "github.com/iamhectordev/hector/pkg/waffle/sqlite"
)

func TestWithPersistentReactionsRequiresReactionStore(t *testing.T) {
	_, err := waffle.NewEventBus(
		waffle.WithStore(waffle.NewMemoryStore()),
		waffle.WithPersistentReactions(),
	)
	require.ErrorContains(t, err, "persistent reactions require a reaction store")
}

func TestPersistentReactionRunsAndMarksSucceeded(t *testing.T) {
	ctx := t.Context()
	db := openPersistentTestDB(t)
	store := wafflesqlite.NewStore(db)
	bus, err := waffle.NewEventBus(
		waffle.WithStore(store),
		waffle.WithPersistentReactions(),
	)
	require.NoError(t, err)
	def, err := waffle.Define[testMessage]("test.message_received", 1)
	require.NoError(t, err)
	got := make(chan string, 1)

	err = waffle.On(bus, def).Handle("test.capture", func(_ context.Context, event waffle.Event[testMessage]) error {
		got <- event.Data().Text
		return nil
	})
	require.NoError(t, err)
	require.NoError(t, bus.Start(ctx))

	require.NoError(t, bus.Record(ctx, def.New(testMessage{Text: "hello"})))
	require.NoError(t, bus.Drain(ctx))

	require.Equal(t, "hello", <-got)
	require.Equal(t, string(waffle.ReactionSucceeded), onlyReactionStatus(t, db))
}

func TestPersistentReactionFailureMarksFailed(t *testing.T) {
	ctx := t.Context()
	db := openPersistentTestDB(t)
	handlerErr := errors.New("handler failed")
	hookErr := make(chan error, 1)
	store := wafflesqlite.NewStore(db)
	bus, err := waffle.NewEventBus(
		waffle.WithStore(store),
		waffle.WithPersistentReactions(),
		waffle.WithErrorHook(func(_ context.Context, _ waffle.AnyEvent, handlerName string, err error) {
			require.Equal(t, "test.fail", handlerName)
			hookErr <- err
		}),
	)
	require.NoError(t, err)
	def, err := waffle.Define[testMessage]("test.message_received", 1)
	require.NoError(t, err)

	err = waffle.On(bus, def).Handle("test.fail", func(context.Context, waffle.Event[testMessage]) error {
		return handlerErr
	})
	require.NoError(t, err)
	require.NoError(t, bus.Start(ctx))

	require.NoError(t, bus.Record(ctx, def.New(testMessage{})))
	require.NoError(t, bus.Drain(ctx))

	require.ErrorIs(t, <-hookErr, handlerErr)
	require.Equal(t, string(waffle.ReactionFailed), onlyReactionStatus(t, db))
}

func TestPersistentReactionPanicMarksFailed(t *testing.T) {
	ctx := t.Context()
	db := openPersistentTestDB(t)
	hookErr := make(chan error, 1)
	store := wafflesqlite.NewStore(db)
	bus, err := waffle.NewEventBus(
		waffle.WithStore(store),
		waffle.WithPersistentReactions(),
		waffle.WithErrorHook(func(_ context.Context, _ waffle.AnyEvent, handlerName string, err error) {
			require.Equal(t, "test.panic", handlerName)
			hookErr <- err
		}),
	)
	require.NoError(t, err)
	def, err := waffle.Define[testMessage]("test.message_received", 1)
	require.NoError(t, err)

	err = waffle.On(bus, def).Handle("test.panic", func(context.Context, waffle.Event[testMessage]) error {
		panic("handler panic")
	})
	require.NoError(t, err)
	require.NoError(t, bus.Start(ctx))

	require.NoError(t, bus.Record(ctx, def.New(testMessage{})))
	require.NoError(t, bus.Drain(ctx))

	require.ErrorContains(t, <-hookErr, "handler panic")
	require.Equal(t, string(waffle.ReactionFailed), onlyReactionStatus(t, db))
}

func TestPersistentReactionResumesPendingAfterRestart(t *testing.T) {
	ctx := t.Context()
	db := openPersistentTestDB(t)
	store := wafflesqlite.NewStore(db)
	def, err := waffle.Define[testMessage]("test.message_received", 1)
	require.NoError(t, err)

	event := waffle.EventRecord{
		ID:            "evt_restart",
		Type:          def.Type(),
		SchemaVersion: def.SchemaVersion(),
		OccurredAt:    time.Now().UTC(),
		Payload:       []byte(`{"Text":"after restart"}`),
	}
	reaction := waffle.ReactionRecord{
		ID:          "rxn_restart",
		EventID:     event.ID,
		HandlerName: "test.resume",
		Status:      waffle.ReactionPending,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	require.NoError(t, store.RecordEventReactions(ctx, event, []waffle.ReactionRecord{reaction}))

	bus, err := waffle.NewEventBus(
		waffle.WithStore(store),
		waffle.WithPersistentReactions(),
	)
	require.NoError(t, err)
	got := make(chan string, 1)
	err = waffle.On(bus, def).Handle("test.resume", func(_ context.Context, event waffle.Event[testMessage]) error {
		got <- event.Data().Text
		return nil
	})
	require.NoError(t, err)

	require.NoError(t, bus.Start(ctx))
	require.NoError(t, bus.Drain(ctx))

	require.Equal(t, "after restart", <-got)
	require.Equal(t, string(waffle.ReactionSucceeded), onlyReactionStatus(t, db))
}

func TestPersistentReactionsDoNotRunMissingHandlers(t *testing.T) {
	ctx := t.Context()
	db := openPersistentTestDB(t)
	store := wafflesqlite.NewStore(db)
	def, err := waffle.Define[testMessage]("test.message_received", 1)
	require.NoError(t, err)

	event := waffle.EventRecord{
		ID:            "evt_missing_handler",
		Type:          def.Type(),
		SchemaVersion: def.SchemaVersion(),
		OccurredAt:    time.Now().UTC(),
		Payload:       []byte(`{"Text":"skip"}`),
	}
	reaction := waffle.ReactionRecord{
		ID:          "rxn_missing_handler",
		EventID:     event.ID,
		HandlerName: "test.missing",
		Status:      waffle.ReactionPending,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	require.NoError(t, store.RecordEventReactions(ctx, event, []waffle.ReactionRecord{reaction}))

	bus, err := waffle.NewEventBus(
		waffle.WithStore(store),
		waffle.WithPersistentReactions(),
	)
	require.NoError(t, err)
	require.NoError(t, bus.Start(ctx))
	require.NoError(t, bus.Drain(ctx))

	require.Equal(t, string(waffle.ReactionPending), onlyReactionStatus(t, db))
}

func TestPersistentReactionOnlyRunsOnceInProcess(t *testing.T) {
	ctx := t.Context()
	db := openPersistentTestDB(t)
	store := wafflesqlite.NewStore(db)
	bus, err := waffle.NewEventBus(
		waffle.WithStore(store),
		waffle.WithPersistentReactions(),
	)
	require.NoError(t, err)
	def, err := waffle.Define[testMessage]("test.message_received", 1)
	require.NoError(t, err)
	var calls atomic.Int32

	err = waffle.On(bus, def).Handle("test.once", func(context.Context, waffle.Event[testMessage]) error {
		calls.Add(1)
		return nil
	})
	require.NoError(t, err)
	require.NoError(t, bus.Start(ctx))

	require.NoError(t, bus.Record(ctx, def.New(testMessage{})))
	require.NoError(t, bus.Drain(ctx))
	require.NoError(t, bus.Drain(ctx))

	require.EqualValues(t, 1, calls.Load())
}

func openPersistentTestDB(t *testing.T) *sql.DB {
	t.Helper()

	path := filepath.Join(t.TempDir(), "persistent-reactions.db")
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path))
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, db.Close()) })

	runner := migrations.New(db)
	require.NoError(t, runner.Add(wafflesqlite.Migrations()))
	require.NoError(t, runner.Run(t.Context()))
	return db
}

func onlyReactionStatus(t *testing.T, db *sql.DB) string {
	t.Helper()

	var status string
	require.NoError(t, db.QueryRow(`SELECT status FROM waffle_reactions`).Scan(&status))
	return status
}
