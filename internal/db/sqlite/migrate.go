package sqlite

import (
	"context"
	"database/sql"

	"github.com/iamhectordev/hector/pkg/migrations"
)

// Migrate applies the provided migration sets against the opened database.
func Migrate(ctx context.Context, db *sql.DB, sets ...migrations.MigrationSet) error {
	runner := migrations.New(db)
	for _, set := range sets {
		if err := runner.Add(set); err != nil {
			return err
		}
	}
	return runner.Run(ctx)
}
