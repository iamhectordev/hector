package llm

import (
	"fmt"

	"github.com/iamhectordev/hector/modules/agent"
	"github.com/iamhectordev/hector/pkg/llm/providers/echo"
	openaiprovider "github.com/iamhectordev/hector/pkg/llm/providers/openai"
)

type options struct {
	provider Provider
}

// Option overrides how an LLM completer is constructed.
type Option func(*options)

// WithProvider overrides the configured default provider.
func WithProvider(provider Provider) Option {
	return func(opts *options) {
		opts.provider = provider
	}
}

// New constructs an agent completer from typed config.
func New(cfg Config, opts ...Option) (agent.Completer, error) {
	o := options{provider: cfg.DefaultProvider}
	if o.provider == "" {
		o.provider = ProviderEcho
	}
	for _, opt := range opts {
		opt(&o)
	}

	if err := validate.Var(string(o.provider), "oneof=echo openai"); err != nil {
		return nil, fmt.Errorf("llm: invalid provider %q", o.provider)
	}

	switch o.provider {
	case ProviderEcho:
		return &echo.Completer{}, nil
	case ProviderOpenAI:
		if err := validate.Struct(cfg.OpenAI); err != nil {
			return nil, fmt.Errorf("llm: openai config: %w", err)
		}
		return openaiprovider.New(cfg.OpenAI.APIKey, cfg.OpenAI.Model), nil
	default:
		return nil, fmt.Errorf("llm: invalid provider %q", o.provider)
	}
}
