package waffle_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	"github.com/iamhectordev/hector/pkg/waffle"
)

type testMessage struct {
	Text string
}

func TestRecordCallsMatchingTypedHandler(t *testing.T) {
	ctx := t.Context()
	bus, err := waffle.NewEventBus()
	require.NoError(t, err)
	require.NoError(t, bus.Start(ctx))
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
	require.NoError(t, bus.Start(ctx))
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
	require.NoError(t, bus.Start(ctx))
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
	require.NoError(t, bus.Start(ctx))
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

func TestErrorHookCalledOnHandlerFailure(t *testing.T) {
	ctx := t.Context()
	handlerErr := errors.New("handler failed")
	got := make(chan error, 1)

	bus, err := waffle.NewEventBus(
		waffle.WithErrorHook(func(_ context.Context, _ waffle.AnyEvent, handlerName string, err error) {
			require.Equal(t, "test.fail", handlerName)
			got <- err
		}),
	)
	require.NoError(t, err)
	require.NoError(t, bus.Start(ctx))
	def, err := waffle.Define[testMessage]("test.message_received", 1)
	require.NoError(t, err)

	err = waffle.On(bus, def).Handle("test.fail", func(context.Context, waffle.Event[testMessage]) error {
		return handlerErr
	})
	require.NoError(t, err)

	require.NoError(t, bus.Record(ctx, def.New(testMessage{})))
	require.NoError(t, bus.Drain(ctx))

	require.ErrorIs(t, <-got, handlerErr)
}

func TestHandlerPanicReportedAsFailure(t *testing.T) {
	ctx := t.Context()
	got := make(chan error, 1)

	bus, err := waffle.NewEventBus(
		waffle.WithErrorHook(func(_ context.Context, _ waffle.AnyEvent, handlerName string, err error) {
			require.Equal(t, "test.panic", handlerName)
			got <- err
		}),
	)
	require.NoError(t, err)
	require.NoError(t, bus.Start(ctx))
	def, err := waffle.Define[testMessage]("test.message_received", 1)
	require.NoError(t, err)

	err = waffle.On(bus, def).Handle("test.panic", func(context.Context, waffle.Event[testMessage]) error {
		panic("handler panic")
	})
	require.NoError(t, err)

	require.NoError(t, bus.Record(ctx, def.New(testMessage{})))
	require.NoError(t, bus.Drain(ctx))

	require.ErrorContains(t, <-got, "handler panic")
}

func TestWithWorkersRunsHandlersConcurrently(t *testing.T) {
	ctx := t.Context()
	bus, err := waffle.NewEventBus(waffle.WithWorkers(2))
	require.NoError(t, err)
	require.NoError(t, bus.Start(ctx))
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

func TestRecordBeforeStartReturnsError(t *testing.T) {
	ctx := t.Context()
	bus, err := waffle.NewEventBus()
	require.NoError(t, err)
	def, err := waffle.Define[testMessage]("test.message_received", 1)
	require.NoError(t, err)

	require.ErrorIs(t, bus.Record(ctx, def.New(testMessage{})), waffle.ErrNotStarted)
}

func TestHandleRejectsSameEventTypeWithDifferentPayloadType(t *testing.T) {
	bus, err := waffle.NewEventBus()
	require.NoError(t, err)
	first, err := waffle.Define[testMessage]("test.same_type", 1)
	require.NoError(t, err)
	second, err := waffle.Define[struct {
		Other string
	}]("test.same_type", 1)
	require.NoError(t, err)

	err = waffle.On(bus, first).Handle("test.first", func(context.Context, waffle.Event[testMessage]) error {
		return nil
	})
	require.NoError(t, err)

	err = waffle.On(bus, second).Handle("test.second", func(context.Context, waffle.Event[struct{ Other string }]) error {
		return nil
	})
	require.ErrorContains(t, err, "different payload type")
}

func TestStartIsIdempotent(t *testing.T) {
	ctx := t.Context()
	bus, err := waffle.NewEventBus()
	require.NoError(t, err)

	require.NoError(t, bus.Start(ctx))
	require.NoError(t, bus.Start(ctx))
}

func TestRecordAppendsToStore(t *testing.T) {
	ctx := t.Context()
	store := waffle.NewMemoryStore()
	bus, err := waffle.NewEventBus(waffle.WithStore(store))
	require.NoError(t, err)
	require.NoError(t, bus.Start(ctx))
	def, err := waffle.Define[testMessage]("test.message_received", 1)
	require.NoError(t, err)

	require.NoError(t, bus.Record(ctx, def.New(testMessage{Text: "hello"})))

	events := store.Events()
	require.Len(t, events, 1)
	require.Equal(t, "test.message_received", events[0].Type)
	require.Equal(t, 1, events[0].SchemaVersion)
	require.JSONEq(t, `{"Text":"hello"}`, string(events[0].Payload))
}

func TestRecordStoresPropagationHeadersSeparatelyFromPayload(t *testing.T) {
	ctx := t.Context()
	installBusTestTracing(t)
	store := waffle.NewMemoryStore()
	bus, err := waffle.NewEventBus(waffle.WithStore(store))
	require.NoError(t, err)
	require.NoError(t, bus.Start(ctx))
	def, err := waffle.Define[testMessage]("test.propagation_headers", 1)
	require.NoError(t, err)

	recordCtx, span := otel.Tracer("test").Start(ctx, "record")
	defer span.End()
	bag, err := baggage.Parse("session.id=sess_123")
	require.NoError(t, err)
	recordCtx = baggage.ContextWithBaggage(recordCtx, bag)

	require.NoError(t, bus.Record(recordCtx, def.New(testMessage{Text: "hello"})))

	events := store.Events()
	require.Len(t, events, 1)
	require.JSONEq(t, `{"Text":"hello"}`, string(events[0].Payload))
	require.NotEmpty(t, events[0].Headers["traceparent"])
	require.Contains(t, events[0].Headers["baggage"], "session.id=sess_123")
}

func TestInMemoryHandlerExtractsRecordedPropagationHeaders(t *testing.T) {
	ctx := t.Context()
	installBusTestTracing(t)
	bus, err := waffle.NewEventBus()
	require.NoError(t, err)
	def, err := waffle.Define[testMessage]("test.memory_trace_context", 1)
	require.NoError(t, err)
	gotTraceID := make(chan string, 1)
	gotBaggage := make(chan string, 1)

	err = waffle.On(bus, def).Handle("test.trace", func(ctx context.Context, _ waffle.Event[testMessage]) error {
		spanContext := trace.SpanContextFromContext(ctx)
		gotTraceID <- spanContext.TraceID().String()
		gotBaggage <- baggage.FromContext(ctx).Member("session.id").Value()
		return nil
	})
	require.NoError(t, err)
	require.NoError(t, bus.Start(ctx))

	recordCtx, span := otel.Tracer("test").Start(ctx, "record")
	recordTraceID := span.SpanContext().TraceID().String()
	span.End()
	bag, err := baggage.Parse("session.id=sess_456")
	require.NoError(t, err)
	recordCtx = baggage.ContextWithBaggage(recordCtx, bag)

	require.NoError(t, bus.Record(recordCtx, def.New(testMessage{})))
	require.NoError(t, bus.Drain(ctx))

	require.Equal(t, recordTraceID, <-gotTraceID)
	require.Equal(t, "sess_456", <-gotBaggage)
}

func TestRecordReturnsStoreErrors(t *testing.T) {
	ctx := t.Context()
	storeErr := errors.New("store failed")
	bus, err := waffle.NewEventBus(waffle.WithStore(failingStore{err: storeErr}))
	require.NoError(t, err)
	require.NoError(t, bus.Start(ctx))
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

func installBusTestTracing(t *testing.T) {
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
