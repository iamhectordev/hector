package cli

import (
	"context"
	"fmt"

	"github.com/doron-cohen/klee"
	"github.com/iamhectordev/hector/internal/app"
)

func configFromContext(ctx context.Context) (*app.Config, error) {
	cfg := klee.Config[app.Config](ctx)
	if cfg == nil {
		return nil, fmt.Errorf("cli: missing config in context")
	}
	return cfg, nil
}
