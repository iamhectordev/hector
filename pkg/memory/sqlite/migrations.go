package sqlite

import "github.com/iamhectordev/hector/pkg/migrations"

const migrationNamespace = "memory"

// Migrations returns memory schema migrations for use with pkg/migrations.Runner.
func Migrations() migrations.MigrationSet {
	return migrations.NewSet(migrationNamespace,
		migrations.Migration{
			Version: 1,
			Name:    "create_objects",
			SQL: `CREATE TABLE memory_objects (
	id      TEXT PRIMARY KEY,
	content TEXT NOT NULL
)`,
		},
		migrations.Migration{
			Version: 2,
			Name:    "create_objects_fts",
			SQL:     `CREATE VIRTUAL TABLE memory_objects_fts USING fts5(id UNINDEXED, content)`,
		},
		migrations.Migration{
			Version: 3,
			Name:    "create_objects_vec",
			SQL: `CREATE TABLE memory_objects_vec (
	id  TEXT PRIMARY KEY,
	vec BLOB NOT NULL
)`,
		},
	)
}
