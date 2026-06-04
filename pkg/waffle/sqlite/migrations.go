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
		migrations.Migration{
			Version: 3,
			Name:    "create_reactions",
			SQL: `
CREATE TABLE waffle_reactions (
	id TEXT NOT NULL PRIMARY KEY,
	event_id TEXT NOT NULL REFERENCES waffle_events(id),
	handler_name TEXT NOT NULL,
	status TEXT NOT NULL,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	UNIQUE(event_id, handler_name)
)
`,
		},
		migrations.Migration{
			Version: 4,
			Name:    "index_pending_reactions",
			SQL:     `CREATE INDEX waffle_reactions_pending_idx ON waffle_reactions (status, created_at)`,
		},
		migrations.Migration{
			Version: 5,
			Name:    "add_reaction_running_status_constraint",
			SQL:     `CREATE INDEX waffle_reactions_running_idx ON waffle_reactions (status, updated_at)`,
		},
		migrations.Migration{
			Version: 6,
			Name:    "add_event_headers",
			SQL:     `ALTER TABLE waffle_events ADD COLUMN headers BLOB NOT NULL DEFAULT '{}'`,
		},
	)
}
