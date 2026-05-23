package cli

import (
	"context"
	"fmt"

	kleelog "github.com/doron-cohen/klee/log"
	dbsqlite "github.com/iamhectordev/hector/internal/db/sqlite"
	"github.com/iamhectordev/hector/modules/agent"
	"github.com/iamhectordev/hector/modules/github"
	"github.com/iamhectordev/hector/modules/slack"
	"github.com/iamhectordev/hector/modules/tools"
	"github.com/iamhectordev/hector/pkg/comms"
	"github.com/iamhectordev/hector/pkg/llm"
	sessionsqlite "github.com/iamhectordev/hector/pkg/session/sqlite"
	"github.com/iamhectordev/hector/pkg/supervisor"
	"github.com/iamhectordev/hector/pkg/waffle"
	wafflesqlite "github.com/iamhectordev/hector/pkg/waffle/sqlite"
	"github.com/urfave/cli/v3"
)

func serveCommand() *cli.Command {
	return &cli.Command{
		Name:   "serve",
		Usage:  "run Slack bot (Socket Mode)",
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
	if err := dbsqlite.Migrate(ctx, db, wafflesqlite.Migrations(), sessionsqlite.Migrations()); err != nil {
		logger.ErrorContext(ctx, "failed to migrate sqlite database", "err", err)
		return fmt.Errorf("sqlite migrations: %w", err)
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

	replyRouter, err := comms.NewReplyRouter(slackModule.NewReplyHandler())
	if err != nil {
		return err
	}
	timeNow, err := tools.NewTimeNow()
	if err != nil {
		return err
	}
	toolRegistry, err := tools.NewRegistry(replyRouter, timeNow)
	if err != nil {
		return err
	}
	toolsModule, err := tools.NewModule(bus, toolRegistry)
	if err != nil {
		return err
	}
	modules := []supervisor.Module{}
	if cfg.GitHub.Configured() {
		githubModule, err := github.NewModule(
			cfg.GitHub,
			github.WithLogger(logger.With("component", "module", "module", "github")),
			github.WithToolRegistrar(toolRegistry),
		)
		if err != nil {
			return err
		}
		modules = append(modules, githubModule)
	}

	loop := agent.NewLoop(completer,
		agent.WithTools(toolRegistry),
		agent.WithLogger(logger.With("component", "loop")),
	)
	sessionStore := sessionsqlite.NewStore(db)
	modules = append(modules,
		agent.NewModule(bus, loop,
			agent.WithBaseSystem(agent.SystemPrompt),
			agent.WithSessionStore(sessionStore),
		),
		toolsModule,
		slackModule,
	)

	sv, err := supervisor.New(modules,
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
