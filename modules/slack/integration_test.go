package slack_test

import (
	"context"
	"testing"
	"time"

	module "github.com/iamhectordev/hector/modules/slack"
	"github.com/iamhectordev/hector/pkg/slackmock"
	"github.com/iamhectordev/hector/pkg/waffle"
	"github.com/slack-go/slack/slackevents"
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

	srv.ExpectWithResponse("users.info", map[string]any{
		"ok": true,
		"user": map[string]any{
			"id": "U222",
			"profile": map[string]any{
				"display_name": "Test User",
			},
		},
	})
	srv.ExpectWithResponse("conversations.info", map[string]any{
		"ok": true,
		"channel": map[string]any{
			"id":          "D111",
			"name":        "dm-test",
			"num_members": 2,
		},
	})

	require.NoError(t, srv.Push(ctx, &slackevents.MessageEvent{
		Channel:     "D111",
		User:        "U222",
		Text:        "hello from dm",
		ChannelType: slackevents.ChannelTypeIM,
		TimeStamp:   "1610241741.000200",
	}))

	select {
	case data := <-got:
		require.Equal(t, "D111", data.Channel.ID)
		require.Equal(t, module.ChannelTypeDM, data.Channel.Type)
		require.Equal(t, "dm-test", data.Channel.Name)
		require.Equal(t, 2, data.Channel.MemberCount)
		require.Equal(t, "U222", data.Sender.ID)
		require.Equal(t, "Test User", data.Sender.Name)
		require.Equal(t, "hello from dm", data.Text)
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for slack message event")
	}

	cancel()
	require.NoError(t, <-done)
}

func TestModule_Start_ChannelMessageReceived(t *testing.T) {
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

	srv.ExpectWithResponse("users.info", map[string]any{"ok": true, "user": map[string]any{"id": "U222"}})
	srv.ExpectWithResponse("conversations.info", map[string]any{"ok": true, "channel": map[string]any{"id": "C111", "name": "public-channel"}})

	require.NoError(t, srv.Push(ctx, &slackevents.MessageEvent{
		Channel:     "C111",
		User:        "U222",
		Text:        "hello channel",
		ChannelType: slackevents.ChannelTypeChannel,
		TimeStamp:   "1610241741.000200",
	}))

	select {
	case data := <-got:
		require.Equal(t, module.ChannelTypeChannel, data.Channel.Type)
		require.Equal(t, "public-channel", data.Channel.Name)
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for slack message event")
	}

	cancel()
	require.NoError(t, <-done)
}

func TestModule_Start_EnrichmentFailureIgnores(t *testing.T) {
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

	srv.ExpectWithResponse("users.info", map[string]any{
		"ok":    false,
		"error": "user_not_found",
	})
	srv.ExpectWithResponse("conversations.info", map[string]any{
		"ok":    false,
		"error": "channel_not_found",
	})

	require.NoError(t, srv.Push(ctx, &slackevents.MessageEvent{
		Channel:     "D111",
		User:        "U222",
		Text:        "hello from dm",
		ChannelType: slackevents.ChannelTypeIM,
		TimeStamp:   "1610241741.000200",
	}))

	select {
	case data := <-got:
		require.Equal(t, "D111", data.Channel.ID)
		require.Equal(t, module.ChannelTypeDM, data.Channel.Type)
		require.Equal(t, "", data.Channel.Name)
		require.Equal(t, 0, data.Channel.MemberCount)
		require.Equal(t, "U222", data.Sender.ID)
		require.Equal(t, "", data.Sender.Name)
		require.Equal(t, "hello from dm", data.Text)
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for slack message event")
	}

	cancel()
	require.NoError(t, <-done)
}

func TestModule_Start_GroupDMPublishesMessageReceived(t *testing.T) {
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

	srv.ExpectWithResponse("users.info", map[string]any{"ok": true, "user": map[string]any{"id": "U222", "profile": map[string]any{"display_name": "Test User"}}})
	srv.ExpectWithResponse("conversations.info", map[string]any{"ok": true, "channel": map[string]any{"id": "G111", "name": "mpdm-test", "num_members": 3}})

	require.NoError(t, srv.Push(ctx, &slackevents.MessageEvent{
		Channel:     "G111",
		User:        "U222",
		Text:        "hello from group dm",
		ChannelType: slackevents.ChannelTypeMPIM,
		TimeStamp:   "1610241741.000200",
	}))

	select {
	case data := <-got:
		require.Equal(t, module.ChannelTypeGroupDM, data.Channel.Type)
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for slack message event")
	}

	cancel()
	require.NoError(t, <-done)
}

func TestModule_Start_UnknownChannelTypePassesThrough(t *testing.T) {
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

	srv.ExpectWithResponse("users.info", map[string]any{"ok": true, "user": map[string]any{"id": "U222"}})
	srv.ExpectWithResponse("conversations.info", map[string]any{"ok": true, "channel": map[string]any{"id": "X111"}})

	require.NoError(t, srv.Push(ctx, &slackevents.MessageEvent{
		Channel:     "X111",
		User:        "U222",
		Text:        "hello from unknown",
		ChannelType: "bizarre_type",
		TimeStamp:   "1610241741.000200",
	}))

	select {
	case data := <-got:
		require.Equal(t, module.ChannelType("bizarre_type"), data.Channel.Type)
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for slack message event")
	}

	cancel()
	require.NoError(t, <-done)
}
