package cli

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/go-playground/validator/v10"
	"github.com/iamhectordev/hector/modules/agent"
	"github.com/iamhectordev/hector/modules/slack"
	"github.com/iamhectordev/hector/pkg/llm"
	"github.com/iamhectordev/hector/pkg/supervisor"
	"github.com/iamhectordev/hector/pkg/waffle"
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
	logger := slog.Default().With("command", "serve")
	logger.InfoContext(ctx, "starting serve command")

	cfg, err := configFromContext(ctx)
	if err != nil {
		return err
	}
	completer, err := llm.New(cfg.LLM)
	if err != nil {
		return err
	}
	validate := validator.New(validator.WithRequiredStructEnabled())
	if err := validate.Struct(cfg.Slack); err != nil {
		return fmt.Errorf("slack: invalid config: %w", err)
	}

	bus, err := waffle.NewEventBus(
		waffle.WithWorkers(2),
		waffle.WithLogger(logger),
	)
	if err != nil {
		logger.ErrorContext(ctx, "failed to create event bus", "err", err)
		return err
	}

	sv, err := supervisor.New([]supervisor.Module{
		agent.NewModule(bus, completer),
		slack.NewModule(bus, cfg.Slack.AppToken, cfg.Slack.BotToken),
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
	logger.InfoContext(ctx, "serve command finished",
		"reason", rep.Reason,
		"trigger_module", rep.TriggerModule,
		"signal", rep.Signal,
	)
	return rep.Err()
}
