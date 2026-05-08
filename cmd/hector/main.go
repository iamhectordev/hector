package main

import (
	"context"
	"os"

	"github.com/doron-cohen/klee"
	"github.com/iamhectordev/hector/internal/cli"
	"github.com/iamhectordev/hector/pkg/supervisor"
)

func main() {
	ctx, stopSignals := supervisor.NotifyContext(context.Background())
	defer stopSignals()

	app := klee.New[struct{}]("hector", "dev", cli.Commands())
	os.Exit(app.Run(ctx, os.Args))
}
