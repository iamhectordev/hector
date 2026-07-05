package llm

import (
	"fmt"

	anthropicprovider "github.com/iamhectordev/hector/pkg/llm/providers/anthropic"
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

// New constructs a completer from typed config.
func New(cfg *Config, opts ...Option) (Completer, error) {
	if cfg == nil {
		return nil, fmt.Errorf("llm: config is required")
	}
	o := options{provider: cfg.DefaultProvider}
	if o.provider == "" {
		o.provider = ProviderEcho
	}
	for _, opt := range opts {
		opt(&o)
	}

	if err := validate.Var(string(o.provider), "oneof=echo openai anthropic"); err != nil {
		return nil, fmt.Errorf("llm: invalid provider %q", o.provider)
	}

	switch o.provider {
	case ProviderEcho:
		return &echo.Completer{}, nil
	case ProviderOpenAI:
		if err := validate.Struct(&cfg.OpenAI); err != nil {
			return nil, fmt.Errorf("llm: openai config: %w", err)
		}
		apiKey, err := cfg.OpenAI.APIKey.Value()
		if err != nil {
			return nil, fmt.Errorf("llm: openai config: api_key: %w", err)
		}
		return openaiprovider.New(
			apiKey,
			cfg.OpenAI.Model,
			openaiprovider.WithBodyLog(openaiprovider.BodyLogConfig(cfg.BodyLog)),
		), nil
	case ProviderAnthropic:
		apiKey, err := cfg.Anthropic.APIKey.Value()
		if err != nil {
			return nil, fmt.Errorf("llm: anthropic config: api_key: %w", err)
		}
		return anthropicprovider.New(apiKey, cfg.Anthropic.Model), nil
	default:
		return nil, fmt.Errorf("llm: invalid provider %q", o.provider)
	}
}
