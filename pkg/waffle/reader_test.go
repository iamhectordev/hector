package waffle_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iamhectordev/hector/pkg/waffle"
)

func TestNormalizeEventQuery(t *testing.T) {
	q := waffle.NormalizeEventQuery(waffle.EventQuery{})
	require.Equal(t, 100, q.Limit)

	q = waffle.NormalizeEventQuery(waffle.EventQuery{Limit: 5})
	require.Equal(t, 5, q.Limit)

	q = waffle.NormalizeEventQuery(waffle.EventQuery{Limit: 99999})
	require.Equal(t, 1000, q.Limit)
}

func TestBusReader(t *testing.T) {
	ctx := t.Context()
	store := waffle.NewMemoryStore()
	bus, err := waffle.NewEventBus(waffle.WithStore(store))
	require.NoError(t, err)
	require.NoError(t, bus.Start(ctx))

	def, err := waffle.Define[testMessage]("test.message_received", 1)
	require.NoError(t, err)

	ev := def.New(testMessage{Text: "hi"})
	require.NoError(t, bus.Record(ctx, ev))

	reader := bus.Reader()
	got, err := reader.Get(ctx, ev.ID())
	require.NoError(t, err)
	require.Equal(t, ev.Type(), got.Type)
	require.JSONEq(t, `{"Text":"hi"}`, string(got.Payload))

	list, err := reader.List(ctx, waffle.EventQuery{Limit: 10})
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, ev.ID(), list[0].ID)
}

func TestReaderGetNotFound(t *testing.T) {
	ctx := t.Context()
	store := waffle.NewMemoryStore()
	reader := waffle.NewReader(store)

	_, err := reader.Get(ctx, "missing")
	require.ErrorIs(t, err, waffle.ErrEventNotFound)
}

func TestMemoryStoreListBefore(t *testing.T) {
	ctx := t.Context()
	store := waffle.NewMemoryStore()

	t1 := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)

	require.NoError(t, store.Append(ctx, waffle.EventRecord{ID: "a", Type: "x", SchemaVersion: 1, OccurredAt: t1, Payload: []byte(`{}`)}))
	require.NoError(t, store.Append(ctx, waffle.EventRecord{ID: "b", Type: "x", SchemaVersion: 1, OccurredAt: t2, Payload: []byte(`{}`)}))

	out, err := store.List(ctx, waffle.EventQuery{Limit: 10, Before: t2})
	require.NoError(t, err)
	require.Len(t, out, 1)
	require.Equal(t, "a", out[0].ID)
}
