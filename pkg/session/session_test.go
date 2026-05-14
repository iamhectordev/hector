package session_test

import (
	"context"
	"testing"

	"github.com/iamhectordev/hector/pkg/session"
	"github.com/iamhectordev/hector/pkg/waffle"
	"github.com/stretchr/testify/require"
)

type testPayload struct{}

func TestSession_PropagatesThroughWaffle(t *testing.T) {
	ctx := t.Context()
	bus, err := waffle.NewEventBus()
	require.NoError(t, err)
	require.NoError(t, bus.Start(ctx))

	def, err := waffle.Define[testPayload]("session.test", 1)
	require.NoError(t, err)

	received := make(chan session.Session, 1)
	err = waffle.On(bus, def).Handle("session.test", func(ctx context.Context, _ waffle.Event[testPayload]) error {
		s, ok := session.From(ctx)
		if ok {
			received <- s
		}
		return nil
	})
	require.NoError(t, err)

	want := session.Session{SourceURI: "slack://channels/D123"}
	ctx = session.With(ctx, want)
	require.NoError(t, bus.Record(ctx, def.New(testPayload{})))
	require.NoError(t, bus.Drain(ctx))

	require.Equal(t, want, <-received)
}
