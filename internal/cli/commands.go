package cli

import "github.com/urfave/cli/v3"

func Commands() []*cli.Command {
	return []*cli.Command{
		chatCommand(),
		eventsCommand(),
		serveCommand(),
	}
}
