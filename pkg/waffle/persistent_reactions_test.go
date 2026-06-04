package waffle_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

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

func TestPersistentReactionContinuesRecordedTrace(t *testing.T) {
	ctx := t.Context()
	installTestTracing(t)

	db := openPersistentTestDB(t)
	store := wafflesqlite.NewStore(db)
	bus, err := waffle.NewEventBus(
		waffle.WithStore(store),
		waffle.WithPersistentReactions(),
	)
	require.NoError(t, err)
	def, err := waffle.Define[testMessage]("test.trace_context", 1)
	require.NoError(t, err)

	parentCtx, parent := otel.Tracer("test").Start(ctx, "record")
	parentTraceID := parent.SpanContext().TraceID()
	parent.End()

	got := make(chan string, 1)
	err = waffle.On(bus, def).Handle("test.trace", func(ctx context.Context, _ waffle.Event[testMessage]) error {
		_, span := otel.Tracer("test").Start(ctx, "handle")
		defer span.End()
		got <- span.SpanContext().TraceID().String()
		return nil
	})
	require.NoError(t, err)
	require.NoError(t, bus.Start(ctx))

	require.NoError(t, bus.Record(parentCtx, def.New(testMessage{Text: "hello"})))
	require.NoError(t, bus.Drain(ctx))

	require.Equal(t, parentTraceID.String(), <-got)
}

func installTestTracing(t *testing.T) {
	t.Helper()

	previousPropagator := otel.GetTextMapPropagator()
	previousProvider := otel.GetTracerProvider()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.AlwaysSample()))
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		require.NoError(t, provider.Shutdown(t.Context()))
		otel.SetTextMapPropagator(previousPropagator)
		otel.SetTracerProvider(previousProvider)
	})
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

func TestPersistentReactionsConcurrentRecordStress(t *testing.T) {
	ctx := t.Context()
	db := openPersistentTestDB(t)
	store := wafflesqlite.NewStore(db)
	bus, err := waffle.NewEventBus(
		waffle.WithWorkers(8),
		waffle.WithStore(store),
		waffle.WithPersistentReactions(),
	)
	require.NoError(t, err)
	def, err := waffle.Define[testMessage]("test.concurrent", 1)
	require.NoError(t, err)

	const events = 80
	const handlers = 4
	var calls atomic.Int32
	for i := range handlers {
		name := "test.concurrent." + string(rune('a'+i))
		err = waffle.On(bus, def).Handle(name, func(context.Context, waffle.Event[testMessage]) error {
			calls.Add(1)
			return nil
		})
		require.NoError(t, err)
	}
	require.NoError(t, bus.Start(ctx))

	var wg sync.WaitGroup
	for i := range events {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			require.NoError(t, bus.Record(ctx, def.New(testMessage{Text: string(rune('a' + i%26))})))
		}(i)
	}
	wg.Wait()
	require.NoError(t, bus.Drain(ctx))

	require.EqualValues(t, events*handlers, calls.Load())
	requireReactionCounts(t, db, reactionCounts{
		total:     events * handlers,
		succeeded: events * handlers,
	})
}

func TestPersistentReactionsRestartStress(t *testing.T) {
	ctx := t.Context()
	db := openPersistentTestDB(t)
	store := wafflesqlite.NewStore(db)
	def, err := waffle.Define[testMessage]("test.restart_stress", 1)
	require.NoError(t, err)

	const events = 50
	const handlers = 3
	now := time.Now().UTC()
	for i := range events {
		event := waffle.EventRecord{
			ID:            "evt_restart_stress_" + stringID(i),
			Type:          def.Type(),
			SchemaVersion: def.SchemaVersion(),
			OccurredAt:    now.Add(time.Duration(i) * time.Millisecond),
			Payload:       []byte(`{"Text":"resume"}`),
		}
		reactions := make([]waffle.ReactionRecord, 0, handlers)
		for h := range handlers {
			reactions = append(reactions, waffle.ReactionRecord{
				ID:          "rxn_restart_stress_" + stringID(i) + "_" + stringID(h),
				EventID:     event.ID,
				HandlerName: "test.restart." + stringID(h),
				Status:      waffle.ReactionPending,
				CreatedAt:   now,
				UpdatedAt:   now,
			})
		}
		require.NoError(t, store.RecordEventReactions(ctx, event, reactions))
	}

	bus, err := waffle.NewEventBus(
		waffle.WithWorkers(6),
		waffle.WithStore(store),
		waffle.WithPersistentReactions(),
	)
	require.NoError(t, err)
	var calls atomic.Int32
	for h := range handlers {
		err = waffle.On(bus, def).Handle("test.restart."+stringID(h), func(context.Context, waffle.Event[testMessage]) error {
			calls.Add(1)
			return nil
		})
		require.NoError(t, err)
	}
	require.NoError(t, bus.Start(ctx))
	require.NoError(t, bus.Drain(ctx))

	require.EqualValues(t, events*handlers, calls.Load())
	requireReactionCounts(t, db, reactionCounts{
		total:     events * handlers,
		succeeded: events * handlers,
	})
}

func TestPersistentReactionsFailureMixStress(t *testing.T) {
	ctx := t.Context()
	db := openPersistentTestDB(t)
	store := wafflesqlite.NewStore(db)
	hookCalls := make(chan error, 100)
	bus, err := waffle.NewEventBus(
		waffle.WithWorkers(4),
		waffle.WithStore(store),
		waffle.WithPersistentReactions(),
		waffle.WithErrorHook(func(context.Context, waffle.AnyEvent, string, error) {
			hookCalls <- errors.New("called")
		}),
	)
	require.NoError(t, err)
	def, err := waffle.Define[testMessage]("test.failure_mix", 1)
	require.NoError(t, err)

	err = waffle.On(bus, def).Handle("test.ok", func(context.Context, waffle.Event[testMessage]) error {
		return nil
	})
	require.NoError(t, err)
	err = waffle.On(bus, def).Handle("test.fail", func(context.Context, waffle.Event[testMessage]) error {
		return errors.New("failed")
	})
	require.NoError(t, err)
	err = waffle.On(bus, def).Handle("test.panic", func(context.Context, waffle.Event[testMessage]) error {
		panic("boom")
	})
	require.NoError(t, err)
	require.NoError(t, bus.Start(ctx))

	const events = 30
	for range events {
		require.NoError(t, bus.Record(ctx, def.New(testMessage{})))
	}
	require.NoError(t, bus.Drain(ctx))

	require.Len(t, hookCalls, events*2)
	requireReactionCounts(t, db, reactionCounts{
		total:     events * 3,
		succeeded: events,
		failed:    events * 2,
	})
}

func TestPersistentReactionsShutdownPressureKeepsPendingWork(t *testing.T) {
	ctx := t.Context()
	db := openPersistentTestDB(t)
	store := wafflesqlite.NewStore(db)
	bus, err := waffle.NewEventBus(
		waffle.WithWorkers(2),
		waffle.WithStore(store),
		waffle.WithPersistentReactions(),
	)
	require.NoError(t, err)
	def, err := waffle.Define[testMessage]("test.shutdown_pressure", 1)
	require.NoError(t, err)
	release := make(chan struct{})
	started := make(chan struct{}, 20)

	err = waffle.On(bus, def).Handle("test.slow", func(context.Context, waffle.Event[testMessage]) error {
		started <- struct{}{}
		<-release
		return nil
	})
	require.NoError(t, err)
	require.NoError(t, bus.Start(ctx))

	const events = 10
	for range events {
		require.NoError(t, bus.Record(ctx, def.New(testMessage{})))
	}
	for range 2 {
		<-started
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	require.ErrorIs(t, bus.Shutdown(shutdownCtx), context.DeadlineExceeded)

	requireNoTerminalReactions(t, db, events)
	close(release)
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

type reactionCounts struct {
	total     int
	pending   int
	running   int
	succeeded int
	failed    int
}

func requireReactionCounts(t *testing.T, db *sql.DB, want reactionCounts) {
	t.Helper()

	var total int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM waffle_reactions`).Scan(&total))
	require.Equal(t, want.total, total)

	for status, wantCount := range map[waffle.ReactionStatus]int{
		waffle.ReactionPending:   want.pending,
		waffle.ReactionRunning:   want.running,
		waffle.ReactionSucceeded: want.succeeded,
		waffle.ReactionFailed:    want.failed,
	} {
		var got int
		require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM waffle_reactions WHERE status = ?`, string(status)).Scan(&got))
		require.Equal(t, wantCount, got, "status %s", status)
	}
}

func requireNoTerminalReactions(t *testing.T, db *sql.DB, total int) {
	t.Helper()

	var gotTotal int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM waffle_reactions`).Scan(&gotTotal))
	require.Equal(t, total, gotTotal)

	for _, status := range []waffle.ReactionStatus{waffle.ReactionSucceeded, waffle.ReactionFailed} {
		var got int
		require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM waffle_reactions WHERE status = ?`, string(status)).Scan(&got))
		require.Equal(t, 0, got, "status %s", status)
	}
}

func stringID(n int) string {
	return strconv.Itoa(n)
}
