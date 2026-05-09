package main

import (
	"context"
	"fmt"
	"os"

	"github.com/doron-cohen/klee"
	"github.com/iamhectordev/hector/internal/cli"
	"github.com/iamhectordev/hector/pkg/supervisor"
)

func main() {
	ctx, stopSignals := supervisor.NotifyContext(context.Background())
	defer stopSignals()

	app := klee.New[cli.Config]("hector", "dev", cli.Commands())
	if err := app.LoadConfig(klee.ConfigOptions[cli.Config]{FlagArgs: os.Args}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(app.Run(ctx, os.Args))
}
