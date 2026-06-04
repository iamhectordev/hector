package waffle_test

import (
	"context"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iamhectordev/hector/pkg/waffle"
)

func TestMemoryStoreAppendsEvents(t *testing.T) {
	ctx := t.Context()
	store := waffle.NewMemoryStore()
	event := testRecord("evt_1")

	require.NoError(t, store.Append(ctx, event))

	events := store.Events()
	require.Len(t, events, 1)
	require.Equal(t, event.ID, events[0].ID)
	require.Equal(t, event.Type, events[0].Type)
	require.Equal(t, event.SchemaVersion, events[0].SchemaVersion)
	require.Equal(t, event.OccurredAt, events[0].OccurredAt)
	require.Equal(t, event.Payload, events[0].Payload)
}

func TestMemoryStoreReturnsCopies(t *testing.T) {
	ctx := t.Context()
	store := waffle.NewMemoryStore()

	record := testRecord("evt_1")
	record.Headers = map[string]string{"traceparent": "original"}
	require.NoError(t, store.Append(ctx, record))

	events := store.Events()
	events[0].ID = "changed"
	events[0].Payload[0] = 'x'
	events[0].Headers["traceparent"] = "changed"

	events = store.Events()
	require.Equal(t, "evt_1", events[0].ID)
	require.JSONEq(t, `{"message":"hello"}`, string(events[0].Payload))
	require.Equal(t, "original", events[0].Headers["traceparent"])
}

func TestMemoryStorePreservesHeadersOnReadPaths(t *testing.T) {
	ctx := t.Context()
	store := waffle.NewMemoryStore()
	record := testRecord("evt_headers")
	record.Headers = map[string]string{
		"traceparent": "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
		"baggage":     "session.id=sess_123",
	}
	require.NoError(t, store.Append(ctx, record))

	got, err := store.Get(ctx, record.ID)
	require.NoError(t, err)
	require.Equal(t, record.Headers, got.Headers)

	list, err := store.List(ctx, waffle.EventQuery{Limit: 10})
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, record.Headers, list[0].Headers)
}

func TestMemoryStoreRespectsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	store := waffle.NewMemoryStore()

	require.ErrorIs(t, store.Append(ctx, testRecord("evt_1")), context.Canceled)
	require.Empty(t, store.Events())
}

func TestMemoryStoreConcurrentAppend(t *testing.T) {
	ctx := t.Context()
	store := waffle.NewMemoryStore()
	const total = 100

	var wg sync.WaitGroup
	for i := range total {
		wg.Add(1)
		go func() {
			defer wg.Done()
			require.NoError(t, store.Append(ctx, testRecord("evt_"+strconv.Itoa(i))))
		}()
	}
	wg.Wait()

	require.Len(t, store.Events(), total)
}

func testRecord(id string) waffle.EventRecord {
	return waffle.EventRecord{
		ID:            id,
		Type:          "test.message_received",
		SchemaVersion: 1,
		OccurredAt:    time.Now().UTC(),
		Payload:       []byte(`{"message":"hello"}`),
	}
}
