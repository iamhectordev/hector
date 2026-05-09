package cli

import (
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	dbsqlite "github.com/iamhectordev/hector/internal/db/sqlite"
	"github.com/iamhectordev/hector/pkg/waffle"
)

func TestNewSQLiteBus_PersistsRecordedEvents(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "hector.db")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	bus, db, err := newSQLiteBus(t.Context(), logger, dbsqlite.Config{Path: dbPath})
	require.NoError(t, err)

	def, err := waffle.Define[testPayload]("test.persisted", 1)
	require.NoError(t, err)
	require.NoError(t, bus.Record(t.Context(), def.New(testPayload{Value: "ok"})))
	require.NoError(t, bus.Drain(t.Context()))
	require.NoError(t, bus.Shutdown(t.Context()))
	require.NoError(t, db.Close())

	reopened, err := dbsqlite.Open(t.Context(), dbsqlite.Config{Path: dbPath})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, reopened.Close()) })

	var count int
	require.NoError(t, reopened.QueryRow(`SELECT COUNT(*) FROM waffle_events`).Scan(&count))
	require.Equal(t, 1, count)
}

type testPayload struct {
	Value string `json:"value"`
}
