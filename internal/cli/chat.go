package cli

import (
	"context"
	"fmt"
	"os"

	dbsqlite "github.com/iamhectordev/hector/internal/db/sqlite"
	"github.com/iamhectordev/hector/modules/agent"
	"github.com/iamhectordev/hector/modules/tools"
	"github.com/iamhectordev/hector/modules/tui"
	"github.com/iamhectordev/hector/pkg/comms"
	"github.com/iamhectordev/hector/pkg/llm"
	"github.com/iamhectordev/hector/pkg/supervisor"
	"github.com/iamhectordev/hector/pkg/waffle"
	wafflesqlite "github.com/iamhectordev/hector/pkg/waffle/sqlite"
	"github.com/urfave/cli/v3"
)

func chatCommand() *cli.Command {
	return &cli.Command{
		Name:  "chat",
		Usage: "interactive chat session (Ctrl-C to exit)",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "log-level",
				Value: "info",
				Usage: "log level (debug, info, warn, error)",
			},
		},
		Action: chatAction,
	}
}

func chatAction(ctx context.Context, cmd *cli.Command) error {
	logger := setupLogger(cmd.String("log-level")).With("command", "chat")
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
		waffle.WithPersistentReactions(),
	)
	if err != nil {
		logger.ErrorContext(ctx, "failed to create event bus", "err", err)
		return err
	}

	toolRegistry, err := tools.NewRegistry(
		comms.NewReplyRouter(tui.NewReplyHandler(os.Stdout)),
	)
	if err != nil {
		return err
	}
	loop := agent.NewLoop(completer,
		agent.WithTools(toolRegistry),
		agent.WithSystem(agent.SystemPrompt),
		agent.WithLogger(logger.With("component", "loop")),
	)

	sv, err := supervisor.New([]supervisor.Module{
		agent.NewModule(bus, loop),
		tui.NewModule(bus),
	},
		supervisor.WithLogger(logger),
		supervisor.WithPostInitHook("bus.start", bus.Start),
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
