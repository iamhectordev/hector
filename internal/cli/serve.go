package cli

import (
	"context"

	kleelog "github.com/doron-cohen/klee/log"
	"github.com/iamhectordev/hector/internal/app"
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
	runtime, err := app.NewRuntime(cfg, app.WithLogger(logger), app.WithProfile(app.ProfileServe))
	if err != nil {
		return err
	}
	return runtime.Start(ctx)
}
