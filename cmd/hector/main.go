package main

import (
	"context"
	"os"

	"github.com/doron-cohen/klee"
	"github.com/iamhectordev/hector/internal/cli"
)

func main() {
	app := klee.New[struct{}]("hector", "dev", cli.Commands())
	os.Exit(app.Run(context.Background(), os.Args))
}
