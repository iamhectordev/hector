package slack

import "context"

type Module struct {
	server *BotServer
}

func NewModule(server *BotServer) (Module, error) {
	return Module{server: server}, nil
}

func (m Module) Name() string {
	return "slack"
}

func (m Module) Start(ctx context.Context) error {
	return nil
}

func (m Module) Stop(ctx context.Context) error {
	return nil
}
