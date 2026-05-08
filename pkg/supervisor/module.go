package supervisor

import "context"

// Module is a long-running unit supervised by [Supervisor].
type Module interface {
	Name() string
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}
