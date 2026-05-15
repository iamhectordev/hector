package cli

import (
	"context"
	"fmt"

	kleelog "github.com/doron-cohen/klee/log"
	dbsqlite "github.com/iamhectordev/hector/internal/db/sqlite"
	"github.com/iamhectordev/hector/modules/agent"
	"github.com/iamhectordev/hector/modules/slack"
	"github.com/iamhectordev/hector/modules/tools"
	"github.com/iamhectordev/hector/pkg/comms"
	"github.com/iamhectordev/hector/pkg/llm"
	"github.com/iamhectordev/hector/pkg/supervisor"
	"github.com/iamhectordev/hector/pkg/waffle"
	wafflesqlite "github.com/iamhectordev/hector/pkg/waffle/sqlite"
	"github.com/urfave/cli/v3"
)

func serveCommand() *cli.Command {
	return &cli.Command{
		Name:  "serve",
		Usage: "run Slack bot (Socket Mode)",
		Action: serveAction,
	}
}

func serveAction(ctx context.Context, _ *cli.Command) error {
	logger := kleelog.FromCtx(ctx).With("command", "serve")
	logger.InfoContext(ctx, "starting serve command")

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

	slackModule, err := slack.NewModule(bus, cfg.Slack)
	if err != nil {
		return err
	}

	toolRegistry, err := tools.NewRegistry(
		comms.NewReplyRouter(slackModule.NewReplyHandler()),
		tools.TimeNow{},
	)
	if err != nil {
		return err
	}
	toolsModule, err := tools.NewModule(bus, toolRegistry)
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
		toolsModule,
		slackModule,
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
	logger.InfoContext(ctx, "serve command finished",
		"reason", rep.Reason,
		"trigger_module", rep.TriggerModule,
		"signal", rep.Signal,
	)
	return rep.Err()
}
