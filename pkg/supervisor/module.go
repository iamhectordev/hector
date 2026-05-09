package supervisor

import "context"

// Module is a long-running unit supervised by [Supervisor].
type Module interface {
	Name() string
	// Init is called for all modules before any Start. Use it for one-time setup
	// that must complete before the system begins running (e.g. auth, registration).
	Init(ctx context.Context) error
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}
