package waffle_test

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
	wafflesqlite "github.com/iamhectordev/hector/pkg/waffle/sqlite"
)

func BenchmarkMemoryDispatcherRecord(b *testing.B) {
	ctx := context.Background()
	bus, err := waffle.NewEventBus(waffle.WithWorkers(8))
	require.NoError(b, err)
	require.NoError(b, bus.Start(ctx))
	def, err := waffle.Define[testMessage]("bench.memory", 1)
	require.NoError(b, err)
	require.NoError(b, waffle.On(bus, def).Handle("bench.handler", func(context.Context, waffle.Event[testMessage]) error {
		return nil
	}))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		require.NoError(b, bus.Record(ctx, def.New(testMessage{})))
	}
	require.NoError(b, bus.Drain(ctx))
}

func BenchmarkPersistentReactionRecord(b *testing.B) {
	ctx := context.Background()
	db := openBenchDB(b)
	bus, err := waffle.NewEventBus(
		waffle.WithWorkers(8),
		waffle.WithStore(wafflesqlite.NewStore(db)),
		waffle.WithPersistentReactions(),
	)
	require.NoError(b, err)
	def, err := waffle.Define[testMessage]("bench.persistent", 1)
	require.NoError(b, err)
	require.NoError(b, waffle.On(bus, def).Handle("bench.handler", func(context.Context, waffle.Event[testMessage]) error {
		return nil
	}))
	require.NoError(b, bus.Start(ctx))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		require.NoError(b, bus.Record(ctx, def.New(testMessage{})))
	}
	require.NoError(b, bus.Drain(ctx))
}

func BenchmarkPersistentReactionDrainPending(b *testing.B) {
	ctx := context.Background()
	db := openBenchDB(b)
	store := wafflesqlite.NewStore(db)
	def, err := waffle.Define[testMessage]("bench.pending", 1)
	require.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		event := waffle.EventRecord{
			ID:            "evt_bench_" + stringID(i),
			Type:          def.Type(),
			SchemaVersion: def.SchemaVersion(),
			OccurredAt:    time.Now().UTC(),
			Payload:       []byte(`{"Text":"bench"}`),
		}
		reaction := waffle.ReactionRecord{
			ID:          "rxn_bench_" + stringID(i),
			EventID:     event.ID,
			HandlerName: "bench.handler",
			Status:      waffle.ReactionPending,
			CreatedAt:   time.Now().UTC(),
			UpdatedAt:   time.Now().UTC(),
		}
		require.NoError(b, store.RecordEventReactions(ctx, event, []waffle.ReactionRecord{reaction}))
	}
	b.StopTimer()

	bus, err := waffle.NewEventBus(
		waffle.WithWorkers(8),
		waffle.WithStore(store),
		waffle.WithPersistentReactions(),
	)
	require.NoError(b, err)
	require.NoError(b, waffle.On(bus, def).Handle("bench.handler", func(context.Context, waffle.Event[testMessage]) error {
		return nil
	}))

	b.StartTimer()
	require.NoError(b, bus.Start(ctx))
	require.NoError(b, bus.Drain(ctx))
}

func openBenchDB(b *testing.B) *sql.DB {
	b.Helper()

	path := filepath.Join(b.TempDir(), "bench.db")
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path))
	require.NoError(b, err)
	db.SetMaxOpenConns(1)
	b.Cleanup(func() { require.NoError(b, db.Close()) })

	runner := migrations.New(db)
	require.NoError(b, runner.Add(wafflesqlite.Migrations()))
	require.NoError(b, runner.Run(context.Background()))
	return db
}
