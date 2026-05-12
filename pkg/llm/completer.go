package llm

import (
	"context"

	"github.com/iamhectordev/hector/pkg/llm/schema"
)

// Completer produces an assistant reply from a completion request.
type Completer interface {
	Complete(ctx context.Context, req schema.CompletionRequest) (*schema.Message, error)
}
