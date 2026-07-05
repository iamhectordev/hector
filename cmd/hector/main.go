package main

import (
	"context"
	"fmt"
	"os"

	"github.com/doron-cohen/klee"
	"github.com/doron-cohen/klee/secrets"
	appconfig "github.com/iamhectordev/hector/internal/app"
	"github.com/iamhectordev/hector/internal/cli"
	"github.com/iamhectordev/hector/pkg/supervisor"
)

func main() {
	ctx, stopSignals := supervisor.NotifyContext(context.Background())
	defer stopSignals()

	app := klee.New[appconfig.Config]("hector", "dev", cli.Commands()).
		WithSecretStore(secrets.NewKeychain("hector"))
	if err := app.LoadConfig(klee.ConfigOptions[appconfig.Config]{FlagArgs: os.Args, DotEnvFiles: []string{".env"}}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(app.Run(ctx, os.Args))
}
