package agent

import "context"

// Module is the agent runtime unit (see pkg/supervisor).
// hector: same as other modules start and stop
type Module struct{}

func (Module) Name() string {
	return "agent"
}

func (Module) Start(ctx context.Context) error {
	return nil
}

func (Module) Stop(ctx context.Context) error {
	return nil
}
