package cli_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/doron-cohen/klee"
	"github.com/doron-cohen/klee/kleetest"
	appconfig "github.com/iamhectordev/hector/internal/app"
	"github.com/iamhectordev/hector/internal/cli"
	dbsqlite "github.com/iamhectordev/hector/internal/db/sqlite"
	"github.com/iamhectordev/hector/pkg/waffle"
	wafflesqlite "github.com/iamhectordev/hector/pkg/waffle/sqlite"
	"github.com/stretchr/testify/require"
)

func TestEventsList_HumanLimitAndOrdering(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "events.db")
	t.Setenv("HECTOR_DB_PATH", dbPath)
	seedEvents(t, dbPath,
		waffle.EventRecord{
			ID:            "evt_old",
			Type:          "slack.message_received",
			SchemaVersion: 1,
			OccurredAt:    time.Date(2026, 5, 10, 13, 0, 0, 0, time.UTC),
			Payload:       []byte(`{"text":"old"}`),
		},
		waffle.EventRecord{
			ID:            "evt_new",
			Type:          "agent.message_received",
			SchemaVersion: 1,
			OccurredAt:    time.Date(2026, 5, 10, 14, 0, 0, 0, time.UTC),
			Payload:       []byte(`{"text":"new"}`),
		},
	)

	result := runCLI(t, "events", "list", "--limit", "1")
	result.ExitCode.Equals(t, 0)
	result.Stdout.Contains(t, "Recent events (1)")
	result.Stdout.Contains(t, "evt_new")
	require.NotContains(t, result.Stdout.String(), "evt_old")
}

func TestEventsList_JSONShape(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "events.db")
	t.Setenv("HECTOR_DB_PATH", dbPath)
	seedEvents(t, dbPath,
		waffle.EventRecord{
			ID:            "evt_1",
			Type:          "agent.reply_generated",
			SchemaVersion: 2,
			OccurredAt:    time.Date(2026, 5, 10, 15, 30, 0, 0, time.UTC),
			Payload:       []byte(`{"ok":true}`),
		},
	)

	result := runCLI(t, "--json", "events", "list", "--limit", "1")
	result.ExitCode.Equals(t, 0)

	var out []map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(result.Stdout.String()), &out))
	require.Len(t, out, 1)
	require.Equal(t, "evt_1", out[0]["id"])
	require.Equal(t, "agent.reply_generated", out[0]["type"])
	require.Equal(t, "2026-05-10T15:30:00Z", out[0]["occurred_at"])
}

func TestEventsList_InvalidBefore(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "events.db")
	t.Setenv("HECTOR_DB_PATH", dbPath)

	result := runCLI(t, "events", "list", "--before", "yesterday")
	result.ExitCode.Equals(t, 1)
	result.Stderr.Contains(t, "invalid --before value")
}

func TestEventsGet_NotFound(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "events.db")
	t.Setenv("HECTOR_DB_PATH", dbPath)

	result := runCLI(t, "events", "get", "evt_missing")
	result.ExitCode.Equals(t, 1)
	result.Stderr.Contains(t, "event not found: evt_missing")
}

func TestEventsGet_JSONShape(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "events.db")
	t.Setenv("HECTOR_DB_PATH", dbPath)
	seedEvents(t, dbPath,
		waffle.EventRecord{
			ID:            "evt_get",
			Type:          "workflow.step_completed",
			SchemaVersion: 3,
			OccurredAt:    time.Date(2026, 5, 10, 16, 0, 0, 0, time.UTC),
			Payload:       []byte(`{"step":"build"}`),
		},
	)

	result := runCLI(t, "--json", "events", "get", "evt_get")
	result.ExitCode.Equals(t, 0)

	var out map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(result.Stdout.String()), &out))
	require.Equal(t, "evt_get", out["id"])
	require.Equal(t, "workflow.step_completed", out["type"])
	require.Equal(t, "2026-05-10T16:00:00Z", out["occurred_at"])
}

func runCLI(t *testing.T, args ...string) *kleetest.Result {
	t.Helper()

	t.Setenv("OPENAI_API_KEY", "test")
	t.Setenv("SLACK_APP_TOKEN", "test")
	t.Setenv("SLACK_BOT_TOKEN", "test")

	app := klee.New[appconfig.Config]("hector", "test", cli.Commands())
	require.NoError(t, app.LoadConfig(klee.ConfigOptions[appconfig.Config]{
		FlagArgs: append([]string{"hector"}, args...),
	}))
	return kleetest.Run(t, app, args...)
}

func seedEvents(t *testing.T, dbPath string, events ...waffle.EventRecord) {
	t.Helper()

	ctx := context.Background()
	db, err := dbsqlite.Open(ctx, dbsqlite.Config{Path: dbPath})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, db.Close())
	})

	require.NoError(t, dbsqlite.Migrate(ctx, db, wafflesqlite.Migrations()))
	store := wafflesqlite.NewStore(db)
	for _, event := range events {
		require.NoError(t, store.Append(ctx, event))
	}
}
