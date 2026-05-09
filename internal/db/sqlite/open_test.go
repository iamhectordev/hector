package sqlite_test

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	dbsqlite "github.com/iamhectordev/hector/internal/db/sqlite"
)

func TestOpenConfiguresSQLiteDefaults(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "state", "hector.db")

	db, err := dbsqlite.Open(t.Context(), dbsqlite.Config{Path: dbPath})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })

	var mode string
	require.NoError(t, db.QueryRow(`PRAGMA journal_mode`).Scan(&mode))
	require.Equal(t, "wal", mode)

	var foreignKeys int
	require.NoError(t, db.QueryRow(`PRAGMA foreign_keys`).Scan(&foreignKeys))
	require.Equal(t, 1, foreignKeys)
}

func TestOpenPersistsAcrossReopen(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "persist", "events.db")

	db1, err := dbsqlite.Open(t.Context(), dbsqlite.Config{Path: dbPath})
	require.NoError(t, err)
	require.NoError(t, createAndInsert(db1))
	require.NoError(t, db1.Close())

	db2, err := dbsqlite.Open(t.Context(), dbsqlite.Config{Path: dbPath})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db2.Close()) })

	var count int
	require.NoError(t, db2.QueryRow(`SELECT COUNT(*) FROM db_sqlite_test`).Scan(&count))
	require.Equal(t, 1, count)
}

func createAndInsert(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS db_sqlite_test (id INTEGER PRIMARY KEY, name TEXT NOT NULL)`); err != nil {
		return err
	}
	_, err := db.Exec(`INSERT INTO db_sqlite_test (name) VALUES ('ok')`)
	return err
}
