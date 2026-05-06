package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/urfave/cli/v3"
)

func chatCommand() *cli.Command {
	return &cli.Command{
		Name:      "chat",
		Usage:     "send a message to Hector",
		ArgsUsage: "<message>",
		Action:    chatAction,
	}
}

func chatAction(_ context.Context, cmd *cli.Command) error {
	fmt.Println(strings.Join(cmd.Args().Slice(), " "))
	return nil
}
