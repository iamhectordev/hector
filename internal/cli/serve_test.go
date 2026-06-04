package cli_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/doron-cohen/klee"
	appconfig "github.com/iamhectordev/hector/internal/app"
	"github.com/iamhectordev/hector/internal/cli"
	slackmock "github.com/iamhectordev/hector/internal/slack/mock"
	"github.com/slack-go/slack/slackevents"
	"github.com/stretchr/testify/require"
)

func TestServe_IntegrationWithSlackMock(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	// 1. Setup Slack Mock
	srv := slackmock.New(t)

	// 2. Configure Environment variables mimicking the executable
	dbPath := filepath.Join(t.TempDir(), "integration.db")
	t.Setenv("HECTOR_DB_PATH", dbPath)
	t.Setenv("HECTOR_LLM_DEFAULT_PROVIDER", "echo")
	t.Setenv("HECTOR_LOG_LEVEL", "debug")
	t.Setenv("HECTOR_SLACK_API_URL", srv.BaseURL()+"/api/")
	t.Setenv("HECTOR_SLACK_APP_TOKEN", "xapp-fake")
	t.Setenv("HECTOR_SLACK_BOT_TOKEN", "xoxb-fake")
	t.Setenv("SLACK_API_URL", srv.BaseURL()+"/api/")
	t.Setenv("SLACK_APP_TOKEN", "xapp-fake")
	t.Setenv("SLACK_BOT_TOKEN", "xoxb-fake")
	t.Setenv("SLACK_APP_TOKEN", "xapp-fake")
	t.Setenv("SLACK_BOT_TOKEN", "xoxb-fake")
	t.Setenv("WEB_SEARCH_PROVIDER", "tavily")
	t.Setenv("TAVILY_API_KEY", "test-tavily-key")
	t.Setenv("TAVILY_API_URL", "https://example.com")

	// 3. Init klee app
	app := klee.New[appconfig.Config]("hector", "test", cli.Commands())
	require.NoError(t, app.LoadConfig(klee.ConfigOptions[appconfig.Config]{
		FlagArgs: []string{"hector", "serve"},
	}))

	// 4. Run the app in a goroutine
	errCh := make(chan error, 1)
	go func() {
		// Run returns an exit code.
		code := app.Run(ctx, []string{"hector", "--log-level", "debug", "serve"})
		if code != 0 && ctx.Err() == nil {
			t.Errorf("app run failed with code %d", code)
		}
		errCh <- nil
	}()

	// 5. Setup expectations
	srv.ExpectWithResponse("auth.test", map[string]any{
		"ok":      true,
		"team_id": "T123",
		"user_id": "U123",
		"bot_id":  "B123",
	})
	srv.ExpectWithResponse("users.info", map[string]any{
		"ok": true,
		"user": map[string]any{
			"id": "U222",
			"profile": map[string]any{
				"display_name": "Integration User",
			},
		},
	})
	srv.ExpectWithResponse("conversations.info", map[string]any{
		"ok": true,
		"channel": map[string]any{
			"id":          "D123",
			"name":        "integration-channel",
			"num_members": 2,
		},
	})
	postMessage := srv.Expect("chat.postMessage")

	// Wait a moment for supervisor/app to fully boot and websocket to connect
	time.Sleep(2 * time.Second)

	// 6. Push a message into the mocked slack socket
	err := srv.Push(ctx, &slackevents.MessageEvent{
		Channel:     "D123",
		User:        "U222",
		Text:        "hello from integration test",
		ChannelType: "im",
		TimeStamp:   "1610241741.000200",
	})
	require.NoError(t, err)

	// 7. Verify reply expectation was hit by the agent calling the tool
	call := postMessage.Require(t, ctx)
	require.Equal(t, "D123", call.Get("channel"))
	require.Equal(t, `<msg sender_id="U222" sender_name="Integration User">
  <text>hello from integration test</text>
</msg>`, call.Get("text"))
	require.Equal(t, "1610241741.000200", call.Get("thread_ts"))

	// 8. Graceful shutdown
	cancel()

	// Wait for Run to exit
	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for app to exit")
	}
}
