package waffle_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iamhectordev/hector/pkg/waffle"
)

type testMessage struct {
	Text string
}

func TestRecordCallsMatchingTypedHandler(t *testing.T) {
	ctx := t.Context()
	bus, err := waffle.NewEventBus()
	require.NoError(t, err)
	def, err := waffle.Define[testMessage]("test.message_received", 1)
	require.NoError(t, err)
	got := make(chan string, 1)

	err = waffle.On(bus, def).Handle("test.capture", func(_ context.Context, event waffle.Event[testMessage]) error {
		got <- event.Data().Text
		return nil
	})
	require.NoError(t, err)

	require.NoError(t, bus.Record(ctx, def.New(testMessage{Text: "hello"})))
	require.NoError(t, bus.Drain(ctx))

	require.Equal(t, "hello", <-got)
}

func TestRecordFansOutToHandlers(t *testing.T) {
	ctx := t.Context()
	bus, err := waffle.NewEventBus()
	require.NoError(t, err)
	def, err := waffle.Define[testMessage]("test.message_received", 1)
	require.NoError(t, err)
	var calls atomic.Int32

	err = waffle.On(bus, def).Handle("test.first", func(context.Context, waffle.Event[testMessage]) error {
		calls.Add(1)
		return nil
	})
	require.NoError(t, err)
	err = waffle.On(bus, def).Handle("test.second", func(context.Context, waffle.Event[testMessage]) error {
		calls.Add(1)
		return nil
	})
	require.NoError(t, err)

	require.NoError(t, bus.Record(ctx, def.New(testMessage{})))
	require.NoError(t, bus.Drain(ctx))

	require.EqualValues(t, 2, calls.Load())
}

func TestRecordSkipsOtherEventTypes(t *testing.T) {
	ctx := t.Context()
	bus, err := waffle.NewEventBus()
	require.NoError(t, err)
	messageDef, err := waffle.Define[testMessage]("test.message_received", 1)
	require.NoError(t, err)
	otherDef, err := waffle.Define[testMessage]("test.other_event", 1)
	require.NoError(t, err)
	var calls atomic.Int32

	err = waffle.On(bus, otherDef).Handle("test.other", func(context.Context, waffle.Event[testMessage]) error {
		calls.Add(1)
		return nil
	})
	require.NoError(t, err)

	require.NoError(t, bus.Record(ctx, messageDef.New(testMessage{})))
	require.NoError(t, bus.Drain(ctx))

	require.EqualValues(t, 0, calls.Load())
}

func TestDrainWaitsForHandlers(t *testing.T) {
	ctx := t.Context()
	bus, err := waffle.NewEventBus()
	require.NoError(t, err)
	def, err := waffle.Define[testMessage]("test.message_received", 1)
	require.NoError(t, err)
	release := make(chan struct{})
	entered := make(chan struct{}, 1)
	drained := make(chan error, 1)

	err = waffle.On(bus, def).Handle("test.block", func(context.Context, waffle.Event[testMessage]) error {
		entered <- struct{}{}
		<-release
		return nil
	})
	require.NoError(t, err)

	require.NoError(t, bus.Record(ctx, def.New(testMessage{})))

	<-entered

	go func() {
		drained <- bus.Drain(ctx)
	}()

	select {
	case err := <-drained:
		t.Fatalf("drain returned before handler completed: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	close(release)

	require.NoError(t, <-drained)
}

func TestDrainReturnsHandlerErrors(t *testing.T) {
	ctx := t.Context()
	bus, err := waffle.NewEventBus()
	require.NoError(t, err)
	def, err := waffle.Define[testMessage]("test.message_received", 1)
	require.NoError(t, err)
	handlerErr := errors.New("handler failed")

	err = waffle.On(bus, def).Handle("test.fail", func(context.Context, waffle.Event[testMessage]) error {
		return handlerErr
	})
	require.NoError(t, err)

	require.NoError(t, bus.Record(ctx, def.New(testMessage{})))

	require.ErrorIs(t, bus.Drain(ctx), handlerErr)
	require.NoError(t, bus.Drain(ctx))
}

func TestWithWorkersRunsHandlersConcurrently(t *testing.T) {
	ctx := t.Context()
	bus, err := waffle.NewEventBus(waffle.WithWorkers(2))
	require.NoError(t, err)
	def, err := waffle.Define[testMessage]("test.message_received", 1)
	require.NoError(t, err)
	started := make(chan struct{}, 2)
	release := make(chan struct{})

	for _, name := range []string{"test.first", "test.second"} {
		err = waffle.On(bus, def).Handle(name, func(context.Context, waffle.Event[testMessage]) error {
			started <- struct{}{}
			<-release
			return nil
		})
		require.NoError(t, err)
	}

	require.NoError(t, bus.Record(ctx, def.New(testMessage{})))

	for range 2 {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("expected handlers to start concurrently")
		}
	}

	close(release)

	require.NoError(t, bus.Drain(ctx))
}

func TestShutdownRejectsRecord(t *testing.T) {
	ctx := t.Context()
	bus, err := waffle.NewEventBus()
	require.NoError(t, err)
	def, err := waffle.Define[testMessage]("test.message_received", 1)
	require.NoError(t, err)

	require.NoError(t, bus.Shutdown(ctx))

	require.ErrorIs(t, bus.Record(ctx, def.New(testMessage{})), waffle.ErrClosed)
}

func TestRecordAppendsToStore(t *testing.T) {
	ctx := t.Context()
	store := waffle.NewMemoryStore()
	bus, err := waffle.NewEventBus(waffle.WithStore(store))
	require.NoError(t, err)
	def, err := waffle.Define[testMessage]("test.message_received", 1)
	require.NoError(t, err)

	require.NoError(t, bus.Record(ctx, def.New(testMessage{Text: "hello"})))

	events := store.Events()
	require.Len(t, events, 1)
	require.Equal(t, "test.message_received", events[0].Type)
	require.Equal(t, 1, events[0].SchemaVersion)
	require.JSONEq(t, `{"Text":"hello"}`, string(events[0].Payload))
}

func TestRecordReturnsStoreErrors(t *testing.T) {
	ctx := t.Context()
	storeErr := errors.New("store failed")
	bus, err := waffle.NewEventBus(waffle.WithStore(failingStore{err: storeErr}))
	require.NoError(t, err)
	def, err := waffle.Define[testMessage]("test.message_received", 1)
	require.NoError(t, err)

	require.ErrorIs(t, bus.Record(ctx, def.New(testMessage{})), storeErr)
}

type failingStore struct {
	err error
}

func (s failingStore) Append(context.Context, waffle.EventRecord) error {
	return s.err
}

func (s failingStore) Get(context.Context, string) (waffle.EventRecord, error) {
	return waffle.EventRecord{}, waffle.ErrEventNotFound
}

func (s failingStore) List(context.Context, waffle.EventQuery) ([]waffle.EventRecord, error) {
	return nil, nil
}
