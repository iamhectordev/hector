package cli

import (
	"context"
	"fmt"

	"github.com/doron-cohen/klee"
	"github.com/iamhectordev/hector/internal/app"
)

// Config is the typed application config loaded by klee.
type Config = app.Config

func configFromContext(ctx context.Context) (*Config, error) {
	cfg := klee.Config[Config](ctx)
	if cfg == nil {
		return nil, fmt.Errorf("cli: missing config in context")
	}
	return cfg, nil
}
