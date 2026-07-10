package integrations

import (
	"context"

	"github.com/iamhectordev/hector/pkg/comms"
	"github.com/iamhectordev/hector/pkg/tools"
)

// Integration is a capability unit connecting Hector to an external service.
type Integration interface {
	Name() string
}

// ToolProvider is the optional facet for integrations that expose agent tools.
type ToolProvider interface {
	Tools() []tools.Tool
}

// EventSource is the optional facet for integrations that produce inbound events.
// Run must block until ctx is done or a fatal error occurs.
type EventSource interface {
	Run(ctx context.Context) error
}

// Surface is the optional facet for integrations that can receive replies.
type Surface interface {
	ReplyHandler() comms.ReplyHandler
}

// Initializer is the optional facet for integrations that need one-time setup
// such as credential verification or client construction.
type Initializer interface {
	Init(ctx context.Context) error
}
