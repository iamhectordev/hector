package migrations_test

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/stretchr/testify/require"

	"github.com/iamhectordev/hector/pkg/migrations"
)

func TestRunnerRejectsDuplicateNamespace(t *testing.T) {
	db := openTestDB(t)
	runner := migrations.New(db)

	require.NoError(t, runner.Add(migrations.NewSet("waffle",
		migrations.Migration{Version: 1, Name: "create_events", SQL: `CREATE TABLE events (id TEXT)`},
	)))

	err := runner.Add(migrations.NewSet("waffle",
		migrations.Migration{Version: 1, Name: "create_other", SQL: `CREATE TABLE other (id TEXT)`},
	))

	require.ErrorIs(t, err, migrations.ErrDuplicateNamespace)
}

func TestRunnerAppliesAndSkipsAppliedMigration(t *testing.T) {
	ctx := t.Context()
	db := openTestDB(t)
	runner := migrations.New(db)

	require.NoError(t, runner.Add(migrations.NewSet("waffle",
		migrations.Migration{
			Version: 1,
			Name:    "create_events",
			SQL:     `CREATE TABLE events (id TEXT PRIMARY KEY, name TEXT NOT NULL)`,
		},
	)))

	require.NoError(t, runner.Run(ctx))
	require.NoError(t, runner.Run(ctx))

	require.True(t, tableExists(t, db, "events"))

	var dirty bool
	err := db.QueryRowContext(ctx, `SELECT dirty FROM migrations WHERE namespace = ? AND version = ?`, "waffle", 1).Scan(&dirty)
	require.NoError(t, err)
	require.False(t, dirty)
}

func TestRunnerRejectsDirtyMigration(t *testing.T) {
	ctx := t.Context()
	db := openTestDB(t)
	runner := migrations.New(db)

	_, err := db.ExecContext(ctx, `
CREATE TABLE migrations (
	namespace TEXT NOT NULL,
	version INTEGER NOT NULL,
	name TEXT NOT NULL,
	dirty BOOLEAN NOT NULL,
	applied_at TEXT NOT NULL,
	PRIMARY KEY (namespace, version)
)
`)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `
INSERT INTO migrations (namespace, version, name, dirty, applied_at)
VALUES (?, ?, ?, 1, datetime('now'))
`, "waffle", 1, "create_events")
	require.NoError(t, err)

	require.NoError(t, runner.Add(migrations.NewSet("waffle",
		migrations.Migration{Version: 1, Name: "create_events", SQL: `CREATE TABLE events (id TEXT)`},
	)))

	err = runner.Run(ctx)
	require.ErrorIs(t, err, migrations.ErrDirtyMigration)
}

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(dbPath))
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() {
		require.NoError(t, db.Close())
	})

	return db
}

func tableExists(t *testing.T, db *sql.DB, name string) bool {
	t.Helper()

	var exists int
	err := db.QueryRow(`SELECT 1 FROM sqlite_master WHERE type = 'table' AND name = ?`, name).Scan(&exists)
	require.NoError(t, err)

	return exists == 1
}
