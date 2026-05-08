package tui

import "context"

type Module struct {
	// hector: I think we need some instructions here that will allow input and output, just like in a tweet chat. So I'm thinking about something very initial and small that could, for example, receive chat messages over to it and return them. Now, I'm not quite sure if we want to do all the tweet manipulation, including rendering here, or do we want to keep it the CLI layer? Although this is a 2D model, so probably this needs a terminal, this needs a TTY. It would be best if we have something to want to see here and we'll do everything here, including the rendering and all. Although I do want some testable interface, if possible.
}

func (Module) Name() string {
	return "tui"
}

func (Module) Start(ctx context.Context) error {
	return nil
}

func (Module) Stop(ctx context.Context) error {
	return nil
}

// hector: We need start and stop just like any other module.
