package cli

import (
	"context"
	"log/slog"

	"github.com/iamhectordev/hector/modules/agent"
	"github.com/iamhectordev/hector/modules/tui"
	"github.com/iamhectordev/hector/pkg/supervisor"
	"github.com/iamhectordev/hector/pkg/waffle"
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

	bus, err := waffle.NewEventBus(waffle.WithWorkers(2))
	if err != nil {
		logger.ErrorContext(ctx, "failed to create event bus", "err", err)
		return err
	}

	sv, err := supervisor.New([]supervisor.Module{
		agent.NewModule(bus, nil),
		tui.NewModule(bus, nil),
	},
		supervisor.WithPreStopHook("bus.drain", bus.Drain),
		supervisor.WithPostStopHook("bus.shutdown", bus.Shutdown),
	)
	if err != nil {
		logger.ErrorContext(ctx, "failed to create supervisor", "err", err)
		return err
	}

	rep := sv.Run(ctx)
	logger.InfoContext(
		ctx,
		"chat command finished",
		"reason", rep.Reason,
		"trigger_module", rep.TriggerModule,
		"signal", rep.Signal,
	)
	return rep.Err()
}
