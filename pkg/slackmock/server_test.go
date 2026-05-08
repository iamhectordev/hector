//go:build integration

package slackmock_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/iamhectordev/hector/pkg/slackmock"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/socketmode"
	"github.com/stretchr/testify/require"
)

func TestPush_DeliveredToConnectedClient(t *testing.T) {
	srv := slackmock.New(t)

	client := socketmode.New(
		slack.New("xoxb-fake-token", slack.OptionAPIURL(srv.BaseURL()+"/api/")),
		socketmode.OptionAppLevelToken("xapp-fake-token"),
	)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	received := make(chan socketmode.Event, 1)
	go func() {
		for evt := range client.Events {
			if evt.Type == socketmode.EventTypeEventsAPI {
				received <- evt
				return
			}
		}
	}()
	go client.RunContext(ctx) //nolint:errcheck

	payload := slackmock.DMMessage("U222", "D111", "hello")
	err := srv.Push(ctx, payload)
	require.NoError(t, err)

	select {
	case evt := <-received:
		inner, ok := evt.Data.(slack.EventsAPIEvent)
		require.True(t, ok)
		require.Equal(t, "event_callback", inner.Type)
	case <-time.After(3 * time.Second):
		t.Fatal("timeout: event not received by SDK client")
	}
}
