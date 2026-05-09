package llm

import (
	"fmt"

	"github.com/go-playground/validator/v10"
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
	options := options{
		provider: cfg.DefaultProvider,
	}
	if options.provider == "" {
		options.provider = ProviderEcho
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&options)
		}
	}

	validate := validator.New(validator.WithRequiredStructEnabled())
	if err := validate.Var(string(options.provider), "required,oneof=echo openai"); err != nil {
		return nil, fmt.Errorf("llm: invalid provider %q", options.provider)
	}

	switch options.provider {
	case ProviderEcho:
		return &echo.Completer{}, nil
	case ProviderOpenAI:
		selected := struct {
			APIKey string `validate:"required"`
		}{
			APIKey: cfg.OpenAI.APIKey,
		}
		if err := validate.Struct(selected); err != nil {
			return nil, fmt.Errorf("llm: openai api key is required")
		}
		return openaiprovider.New(cfg.OpenAI.APIKey, cfg.OpenAI.Model), nil
	default:
		return nil, fmt.Errorf("llm: invalid provider %q", options.provider)
	}
}
