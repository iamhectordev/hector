package slack_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	slackgo "github.com/slack-go/slack"

	slack "github.com/iamhectordev/hector/integrations/slack"
	"github.com/iamhectordev/hector/integrations/slack/mock"
	"github.com/iamhectordev/hector/pkg/waffle"
	"github.com/slack-go/slack/slackevents"
	"github.com/stretchr/testify/require"
)

func TestModule_Start_DMPublishesMessageReceived(t *testing.T) {
	t.Parallel()

	srv := mock.New(t)
	bus, err := waffle.NewEventBus(waffle.WithWorkers(2))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, bus.Shutdown(context.Background()))
	})

	got := make(chan slack.MessageReceivedData, 1)
	err = waffle.On(bus, slack.MessageReceived).Handle("test.capture", func(_ context.Context, e waffle.Event[slack.MessageReceivedData]) error {
		got <- e.Data()
		return nil
	})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		m, err := slack.New(bus, testConfig(t, srv.BaseURL()+"/api/", nil))
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
		done <- m.Run(ctx)
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
		require.Equal(t, slack.ChannelTypeDM, data.Channel.Type)
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

	srv := mock.New(t)
	bus, err := waffle.NewEventBus(waffle.WithWorkers(2))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, bus.Shutdown(context.Background()))
	})

	got := make(chan slack.MessageReceivedData, 1)
	err = waffle.On(bus, slack.MessageReceived).Handle("test.capture", func(_ context.Context, e waffle.Event[slack.MessageReceivedData]) error {
		got <- e.Data()
		return nil
	})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		m, err := slack.New(bus, testConfig(t, srv.BaseURL()+"/api/", nil))
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
		done <- m.Run(ctx)
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
		require.Equal(t, slack.ChannelTypeChannel, data.Channel.Type)
		require.Equal(t, "public-channel", data.Channel.Name)
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for slack message event")
	}

	cancel()
	require.NoError(t, <-done)
}

func TestModule_Start_EnrichmentFailureIgnores(t *testing.T) {
	t.Parallel()

	srv := mock.New(t)
	bus, err := waffle.NewEventBus(waffle.WithWorkers(2))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, bus.Shutdown(context.Background()))
	})

	got := make(chan slack.MessageReceivedData, 1)
	err = waffle.On(bus, slack.MessageReceived).Handle("test.capture", func(_ context.Context, e waffle.Event[slack.MessageReceivedData]) error {
		got <- e.Data()
		return nil
	})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		m, err := slack.New(bus, testConfig(t, srv.BaseURL()+"/api/", nil))
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
		done <- m.Run(ctx)
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
		require.Equal(t, slack.ChannelTypeDM, data.Channel.Type)
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

	srv := mock.New(t)
	bus, err := waffle.NewEventBus(waffle.WithWorkers(2))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, bus.Shutdown(context.Background()))
	})

	got := make(chan slack.MessageReceivedData, 1)
	err = waffle.On(bus, slack.MessageReceived).Handle("test.capture", func(_ context.Context, e waffle.Event[slack.MessageReceivedData]) error {
		got <- e.Data()
		return nil
	})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		m, err := slack.New(bus, testConfig(t, srv.BaseURL()+"/api/", nil))
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
		done <- m.Run(ctx)
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
		require.Equal(t, slack.ChannelTypeGroupDM, data.Channel.Type)
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for slack message event")
	}

	cancel()
	require.NoError(t, <-done)
}

func TestModule_Start_UnknownChannelTypePassesThrough(t *testing.T) {
	t.Parallel()

	srv := mock.New(t)
	bus, err := waffle.NewEventBus(waffle.WithWorkers(2))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, bus.Shutdown(context.Background()))
	})

	got := make(chan slack.MessageReceivedData, 1)
	err = waffle.On(bus, slack.MessageReceived).Handle("test.capture", func(_ context.Context, e waffle.Event[slack.MessageReceivedData]) error {
		got <- e.Data()
		return nil
	})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		m, err := slack.New(bus, testConfig(t, srv.BaseURL()+"/api/", nil))
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
		done <- m.Run(ctx)
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
		require.Equal(t, slack.ChannelType("bizarre_type"), data.Channel.Type)
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for slack message event")
	}

	cancel()
	require.NoError(t, <-done)
}

func TestModule_Start_AllowListPassesListedUser(t *testing.T) {
	t.Parallel()

	srv := mock.New(t)
	bus, err := waffle.NewEventBus(waffle.WithWorkers(2))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, bus.Shutdown(context.Background()))
	})

	got := make(chan slack.MessageReceivedData, 1)
	err = waffle.On(bus, slack.MessageReceived).Handle("test.capture", func(_ context.Context, e waffle.Event[slack.MessageReceivedData]) error {
		got <- e.Data()
		return nil
	})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		m, err := slack.New(bus, testConfig(t, srv.BaseURL()+"/api/", []string{"U222"}))
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
		done <- m.Run(ctx)
	}()

	srv.ExpectWithResponse("users.info", map[string]any{
		"ok": true,
		"user": map[string]any{
			"id":      "U222",
			"profile": map[string]any{"display_name": "Alice"},
		},
	})
	srv.ExpectWithResponse("conversations.info", map[string]any{
		"ok":      true,
		"channel": map[string]any{"id": "D111", "name": "dm-alice"},
	})

	require.NoError(t, srv.Push(ctx, &slackevents.MessageEvent{
		Channel:     "D111",
		User:        "U222",
		Text:        "hello from allowed user",
		ChannelType: slackevents.ChannelTypeIM,
		TimeStamp:   "1610241741.000200",
	}))

	select {
	case data := <-got:
		require.Equal(t, "U222", data.Sender.ID)
		require.Equal(t, "hello from allowed user", data.Text)
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for slack message event")
	}

	cancel()
	require.NoError(t, <-done)
}

func TestModule_Start_AllowListBlocksUnlistedUser(t *testing.T) {
	t.Parallel()

	srv := mock.New(t)
	bus, err := waffle.NewEventBus(waffle.WithWorkers(2))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, bus.Shutdown(context.Background()))
	})

	got := make(chan slack.MessageReceivedData, 1)
	err = waffle.On(bus, slack.MessageReceived).Handle("test.capture", func(_ context.Context, e waffle.Event[slack.MessageReceivedData]) error {
		got <- e.Data()
		return nil
	})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		m, err := slack.New(bus, testConfig(t, srv.BaseURL()+"/api/", []string{"U222"}))
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
		done <- m.Run(ctx)
	}()

	require.NoError(t, srv.Push(ctx, &slackevents.MessageEvent{
		Channel:     "D111",
		User:        "U999",
		Text:        "hello from blocked user",
		ChannelType: slackevents.ChannelTypeIM,
		TimeStamp:   "1610241741.000100",
	}))

	srv.ExpectWithResponse("users.info", map[string]any{
		"ok": true,
		"user": map[string]any{
			"id":      "U222",
			"profile": map[string]any{"display_name": "Alice"},
		},
	})
	srv.ExpectWithResponse("conversations.info", map[string]any{
		"ok":      true,
		"channel": map[string]any{"id": "D111", "name": "dm-alice"},
	})

	require.NoError(t, srv.Push(ctx, &slackevents.MessageEvent{
		Channel:     "D111",
		User:        "U222",
		Text:        "hello from allowed user",
		ChannelType: slackevents.ChannelTypeIM,
		TimeStamp:   "1610241741.000200",
	}))

	select {
	case data := <-got:
		require.Equal(t, "U222", data.Sender.ID, "only allowed user's message should arrive")
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for slack message event")
	}

	cancel()
	require.NoError(t, <-done)
}

func TestModule_Start_MessageChangedPublishesMessageUpdated(t *testing.T) {
	t.Parallel()

	srv := mock.New(t)
	bus, err := waffle.NewEventBus(waffle.WithWorkers(2))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, bus.Shutdown(context.Background()))
	})

	got := make(chan slack.MessageUpdatedData, 1)
	err = waffle.On(bus, slack.MessageUpdated).Handle("test.capture", func(_ context.Context, e waffle.Event[slack.MessageUpdatedData]) error {
		got <- e.Data()
		return nil
	})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		m, err := slack.New(bus, testConfig(t, srv.BaseURL()+"/api/", nil))
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
		done <- m.Run(ctx)
	}()

	srv.ExpectWithResponse("users.info", map[string]any{
		"ok": true,
		"user": map[string]any{
			"id":      "U222",
			"profile": map[string]any{"display_name": "Alice", "real_name": "Alice"},
		},
	})
	srv.ExpectWithResponse("users.info", map[string]any{
		"ok": true,
		"user": map[string]any{
			"id":      "U999",
			"profile": map[string]any{"display_name": "Bob", "real_name": "Bob"},
		},
	})
	srv.ExpectWithResponse("conversations.info", map[string]any{
		"ok": true,
		"channel": map[string]any{
			"id":   "C999",
			"name": "general",
		},
	})
	srv.ExpectWithResponse("conversations.info", map[string]any{
		"ok": true,
		"channel": map[string]any{
			"id":   "D024BE91L",
			"name": "dm-channel",
		},
	})

	require.NoError(t, srv.Push(ctx, &slackevents.MessageEvent{
		SubType:     "message_changed",
		Channel:     "D024BE91L",
		TimeStamp:   "1355517523.000005",
		ChannelType: slackevents.ChannelTypeIM,
		Message: &slackgo.Msg{
			User:      "U222",
			Text:      "hello from dm",
			Timestamp: "1355517523.000005",
			Attachments: []slackgo.Attachment{
				{
					AuthorID:   "U999",
					AuthorName: "Bob",
					Ts:         json.Number("1355517522.000001"),
					Text:       "Original message",
					FromURL:    "https://hector.slack.com/archives/C999/p1355517522000001",
				},
			},
		},
	}))

	select {
	case data := <-got:
		require.Equal(t, "D024BE91L", data.Channel.ID)
		require.Equal(t, "U222", data.Sender.ID)
		require.Equal(t, "Alice", data.Sender.Name)
		require.Equal(t, "hello from dm", data.Text)
		require.Len(t, data.Forwards, 1)
		require.Equal(t, "U999", data.Forwards[0].Sender.ID)
		require.Equal(t, "Bob", data.Forwards[0].Sender.Name)
		require.Equal(t, "Original message", data.Forwards[0].Text)
		require.Equal(t, "C999", data.Forwards[0].Channel.ID)
		require.Equal(t, slack.ChannelTypeDM, data.Channel.Type)
		require.NotZero(t, data.UpdatedAt)
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for slack message_updated event")
	}

	cancel()
	require.NoError(t, <-done)
}

func testConfig(t *testing.T, apiURL string, allowUsers []string) *slack.Config {
	t.Helper()
	cfg := &slack.Config{
		Enabled:    true,
		APIURL:     apiURL,
		AllowUsers: allowUsers,
	}
	require.NoError(t, cfg.AppToken.UnmarshalText([]byte("xapp-fake-token")))
	require.NoError(t, cfg.BotToken.UnmarshalText([]byte("xoxb-fake-token")))
	return cfg
}
