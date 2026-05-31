package cli

import (
	"context"

	kleelog "github.com/doron-cohen/klee/log"
	"github.com/iamhectordev/hector/internal/app"
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
	logger := kleelog.FromCtx(ctx).With("command", "chat")
	logger.InfoContext(ctx, "starting chat command")

	cfg, err := configFromContext(ctx)
	if err != nil {
		return err
	}
	runtime, err := app.NewRuntime(*cfg, app.WithLogger(logger), app.WithProfile(app.ProfileChat))
	if err != nil {
		return err
	}
	return runtime.Start(ctx)
}
