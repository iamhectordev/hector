package cli

import (
	"context"
	"fmt"
	"log/slog"

	dbsqlite "github.com/iamhectordev/hector/internal/db/sqlite"
	"github.com/iamhectordev/hector/modules/agent"
	"github.com/iamhectordev/hector/modules/tui"
	"github.com/iamhectordev/hector/pkg/llm"
	"github.com/iamhectordev/hector/pkg/supervisor"
	"github.com/iamhectordev/hector/pkg/waffle"
	wafflesqlite "github.com/iamhectordev/hector/pkg/waffle/sqlite"
	"github.com/urfave/cli/v3"
)

func chatCommand() *cli.Command {
	return &cli.Command{
		Name:   "chat",
		Usage:  "interactive chat session (Ctrl-C to exit)",
		Action: chatAction,
	}
}

func chatAction(ctx context.Context, _ *cli.Command) error {
	logger := slog.Default().With("command", "chat")
	logger.InfoContext(ctx, "starting chat command")

	cfg, err := configFromContext(ctx)
	if err != nil {
		return err
	}
	completer, err := llm.New(cfg.LLM)
	if err != nil {
		return err
	}

	db, err := dbsqlite.Open(ctx, cfg.DB)
	if err != nil {
		logger.ErrorContext(ctx, "failed to open sqlite database", "err", err)
		return err
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			logger.ErrorContext(ctx, "failed to close sqlite database", "err", closeErr)
		}
	}()
	if err := dbsqlite.Migrate(ctx, db, wafflesqlite.Migrations()); err != nil {
		logger.ErrorContext(ctx, "failed to migrate sqlite database", "err", err)
		return fmt.Errorf("waffle migrations: %w", err)
	}
	bus, err := waffle.NewEventBus(
		waffle.WithWorkers(2),
		waffle.WithLogger(logger),
		waffle.WithStore(wafflesqlite.NewStore(db)),
	)
	if err != nil {
		logger.ErrorContext(ctx, "failed to create event bus", "err", err)
		return err
	}

	sv, err := supervisor.New([]supervisor.Module{
		agent.NewModule(bus, agent.NewLoop(completer)),
		tui.NewModule(bus),
	},
		supervisor.WithLogger(logger),
		supervisor.WithPreStopHook("bus.drain", bus.Drain),
		supervisor.WithPostStopHook("bus.shutdown", bus.Shutdown),
	)
	if err != nil {
		logger.ErrorContext(ctx, "failed to create supervisor", "err", err)
		return err
	}

	rep := sv.Run(ctx)
	logger.InfoContext(ctx, "chat command finished",
		"reason", rep.Reason,
		"trigger_module", rep.TriggerModule,
		"signal", rep.Signal,
	)
	return rep.Err()
}
