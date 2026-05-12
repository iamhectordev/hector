package slack_test

import (
	"context"
	"testing"
	"time"

	module "github.com/iamhectordev/hector/modules/slack"
	"github.com/iamhectordev/hector/pkg/slackmock"
	"github.com/iamhectordev/hector/pkg/waffle"
	"github.com/stretchr/testify/require"
)

func TestModule_Start_DMPublishesMessageReceived(t *testing.T) {
	t.Parallel()

	srv := slackmock.New(t)
	bus, err := waffle.NewEventBus(waffle.WithWorkers(2))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, bus.Shutdown(context.Background()))
	})

	got := make(chan module.MessageReceivedData, 1)
	err = waffle.On(bus, module.MessageReceived).Handle("test.capture", func(_ context.Context, e waffle.Event[module.MessageReceivedData]) error {
		got <- e.Data()
		return nil
	})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		m, err := module.NewModule(bus, module.Config{
			AppToken: "xapp-fake-token",
			BotToken: "xoxb-fake-token",
			APIURL:   srv.BaseURL() + "/api/",
		})
		if err != nil {
			done <- err
			return
		}
		if err := m.Init(ctx); err != nil {
			done <- err
			return
		}
		if err := bus.Start(ctx); err != nil {
			done <- err
			return
		}
		done <- m.Start(ctx)
	}()

	require.NoError(t, srv.Push(ctx, slackmock.DMMessage("U222", "D111", "hello from dm")))

	select {
	case data := <-got:
		require.Equal(t, "D111", data.ChannelID)
		require.Equal(t, "U222", data.SenderID)
		require.Equal(t, "hello from dm", data.Text)
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for slack message event")
	}

	cancel()
	require.NoError(t, <-done)
}

func TestModule_Start_ChannelMessageIgnored(t *testing.T) {
	t.Parallel()

	srv := slackmock.New(t)
	bus, err := waffle.NewEventBus(waffle.WithWorkers(2))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, bus.Shutdown(context.Background()))
	})

	got := make(chan module.MessageReceivedData, 1)
	err = waffle.On(bus, module.MessageReceived).Handle("test.capture", func(_ context.Context, e waffle.Event[module.MessageReceivedData]) error {
		got <- e.Data()
		return nil
	})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		m, err := module.NewModule(bus, module.Config{
			AppToken: "xapp-fake-token",
			BotToken: "xoxb-fake-token",
			APIURL:   srv.BaseURL() + "/api/",
		})
		if err != nil {
			done <- err
			return
		}
		if err := m.Init(ctx); err != nil {
			done <- err
			return
		}
		if err := bus.Start(ctx); err != nil {
			done <- err
			return
		}
		done <- m.Start(ctx)
	}()

	require.NoError(t, srv.Push(ctx, slackmock.ChannelMessage("U222", "C111", "ignore me")))

	select {
	case data := <-got:
		t.Fatalf("unexpected slack message event: %+v", data)
	case <-time.After(300 * time.Millisecond):
	}

	cancel()
	require.NoError(t, <-done)
}
