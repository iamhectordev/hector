package sqlite

import "github.com/iamhectordev/hector/pkg/migrations"

const migrationNamespace = "waffle"

// Migrations returns Waffle's schema migrations for use with pkg/migrations.Runner.
func Migrations() migrations.MigrationSet {
	return migrations.NewSet(migrationNamespace,
		migrations.Migration{
			Version: 1,
			Name:    "create_events",
			SQL: `
CREATE TABLE waffle_events (
	id TEXT NOT NULL PRIMARY KEY,
	type TEXT NOT NULL,
	schema_version INTEGER NOT NULL,
	occurred_at TEXT NOT NULL,
	payload BLOB NOT NULL
)
`,
		},
		migrations.Migration{
			Version: 2,
			Name:    "index_occurred_at",
			SQL:     `CREATE INDEX waffle_events_occurred_at_idx ON waffle_events (occurred_at)`,
		},
	)
}
