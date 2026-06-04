package mock_test

import (
	"context"
	"testing"
	"time"

	"github.com/iamhectordev/hector/internal/slack/mock"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
	"github.com/stretchr/testify/require"
)

func TestPush_ErrorWhenNoClientConnected(t *testing.T) {
	srv := mock.New(t)

	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()

	err := srv.Push(ctx, &slackevents.MessageEvent{
		Channel:     "D111",
		User:        "U222",
		Text:        "hello",
		ChannelType: slackevents.ChannelTypeIM,
		TimeStamp:   "1610241741.000200",
	})
	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestPush_DeliveredToConnectedClient(t *testing.T) {
	srv := mock.New(t)

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

	err := srv.Push(ctx, &slackevents.MessageEvent{
		Channel:     "D111",
		User:        "U222",
		Text:        "hello",
		ChannelType: slackevents.ChannelTypeIM,
		TimeStamp:   "1610241741.000200",
	})
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

func TestExpect_CapturesAPICall(t *testing.T) {
	srv := mock.New(t)

	expect := srv.Expect("chat.postMessage")

	go func() {
		_, _, err := slack.New(
			"xoxb-fake-token",
			slack.OptionAPIURL(srv.BaseURL()+"/api/"),
		).PostMessageContext(t.Context(), "D123", slack.MsgOptionText("hello", false))
		_ = err
	}()

	call := expect.Require(t, t.Context())
	require.Equal(t, "D123", call.Get("channel"))
	require.Equal(t, "hello", call.Get("text"))
}

func TestExpect_AssertNotCalled(t *testing.T) {
	srv := mock.New(t)
	expect := srv.Expect("chat.postMessage")
	expect.AssertNotCalled(t, 50*time.Millisecond)
}
