package cli

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	dbsqlite "github.com/iamhectordev/hector/internal/db/sqlite"
	"github.com/iamhectordev/hector/pkg/migrations"
	"github.com/iamhectordev/hector/pkg/waffle"
	wafflesqlite "github.com/iamhectordev/hector/pkg/waffle/sqlite"
)

func newSQLiteBus(ctx context.Context, logger *slog.Logger, cfg dbsqlite.Config) (*waffle.EventBus, *sql.DB, error) {
	db, err := dbsqlite.Open(ctx, cfg)
	if err != nil {
		return nil, nil, err
	}

	runner := migrations.New(db)
	if err := runner.Add(wafflesqlite.Migrations()); err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("waffle migrations: add set: %w", err)
	}
	if err := runner.Run(ctx); err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("waffle migrations: run: %w", err)
	}

	bus, err := waffle.NewEventBus(
		waffle.WithWorkers(2),
		waffle.WithLogger(logger),
		waffle.WithStore(wafflesqlite.NewStore(db)),
	)
	if err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("create event bus: %w", err)
	}

	return bus, db, nil
}
