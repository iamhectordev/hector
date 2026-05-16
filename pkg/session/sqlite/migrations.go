package sqlite

import "github.com/iamhectordev/hector/pkg/migrations"

const migrationNamespace = "session"

// Migrations returns session schema migrations for use with pkg/migrations.Runner.
func Migrations() migrations.MigrationSet {
	return migrations.NewSet(migrationNamespace,
		migrations.Migration{
			Version: 1,
			Name:    "create_sessions",
			SQL: `
CREATE TABLE session_sessions (
	id TEXT NOT NULL PRIMARY KEY,
	source_uri TEXT NOT NULL UNIQUE,
	created_at TEXT NOT NULL
)
`,
		},
		migrations.Migration{
			Version: 2,
			Name:    "create_records",
			SQL: `
CREATE TABLE session_records (
	seq INTEGER PRIMARY KEY AUTOINCREMENT,
	id TEXT NOT NULL UNIQUE,
	session_id TEXT NOT NULL REFERENCES session_sessions(id),
	role TEXT NOT NULL,
	message_json TEXT NOT NULL,
	created_at TEXT NOT NULL
)
`,
		},
		migrations.Migration{
			Version: 3,
			Name:    "index_records_session_seq",
			SQL:     `CREATE INDEX session_records_session_seq_idx ON session_records (session_id, seq)`,
		},
	)
}
