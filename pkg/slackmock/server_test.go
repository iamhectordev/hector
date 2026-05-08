package slackmock_test

import (
	"context"
	"testing"
	"time"

	"github.com/iamhectordev/hector/pkg/slackmock"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
	"github.com/stretchr/testify/require"
)

func TestPush_ErrorWhenNoClientConnected(t *testing.T) {
	srv := slackmock.New(t)

	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()

	err := srv.Push(ctx, slackmock.DMMessage("U222", "D111", "hello"))
	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestPush_DeliveredToConnectedClient(t *testing.T) {
	srv := slackmock.New(t)

	client := socketmode.New(
		slack.New(
			"xoxb-fake-token",
			slack.OptionAPIURL(srv.BaseURL()+"/api/"),
			slack.OptionAppLevelToken("xapp-fake-token"),
		),
	)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	type recv struct {
		evt    socketmode.Event
		ackErr error
	}
	received := make(chan recv, 1)
	go func() {
		for evt := range client.Events {
			if evt.Type == socketmode.EventTypeEventsAPI {
				received <- recv{evt: evt, ackErr: client.Ack(*evt.Request)}
				return
			}
		}
	}()
	go client.RunContext(ctx) //nolint:errcheck

	err := srv.Push(ctx, slackmock.DMMessage("U222", "D111", "hello"))
	require.NoError(t, err)

	select {
	case r := <-received:
		require.NoError(t, r.ackErr)
		inner, ok := r.evt.Data.(slackevents.EventsAPIEvent)
		require.True(t, ok)
		require.Equal(t, "event_callback", inner.Type)
	case <-time.After(3 * time.Second):
		t.Fatal("timeout: event not received by SDK client")
	}
}
