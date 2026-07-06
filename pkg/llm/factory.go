package llm

import (
	"context"
	"fmt"
	"strings"

	"golang.org/x/oauth2/google"

	anthropicprovider "github.com/iamhectordev/hector/pkg/llm/providers/anthropic"
	"github.com/iamhectordev/hector/pkg/llm/providers/echo"
	openaiprovider "github.com/iamhectordev/hector/pkg/llm/providers/openai"
	vertexpkg "github.com/anthropics/anthropic-sdk-go/vertex"
)

type options struct {
	backend Backend
}

// Option overrides how an LLM completer is constructed.
type Option func(*options)

// WithBackend overrides the configured default backend.
func WithBackend(backend Backend) Option {
	return func(opts *options) {
		opts.backend = backend
	}
}

// New constructs a completer from typed config.
func New(ctx context.Context, cfg *Config, opts ...Option) (Completer, error) {
	if cfg == nil {
		return nil, fmt.Errorf("llm: config is required")
	}
	o := options{backend: cfg.DefaultBackend}
	if o.backend == "" {
		o.backend = BackendEcho
	}
	for _, opt := range opts {
		opt(&o)
	}

	if err := validate.Var(string(o.backend), "oneof=echo openai anthropic vertex"); err != nil {
		return nil, fmt.Errorf("llm: invalid backend %q", o.backend)
	}

	switch o.backend {
	case BackendEcho:
		return &echo.Completer{}, nil
	case BackendOpenAI:
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
	case BackendAnthropic:
		apiKey, err := cfg.Anthropic.APIKey.Value()
		if err != nil {
			return nil, fmt.Errorf("llm: anthropic config: api_key: %w", err)
		}
		return anthropicprovider.New(apiKey, cfg.Anthropic.Model), nil
	case BackendVertex:
		if err := validate.Struct(&cfg.Vertex); err != nil {
			return nil, fmt.Errorf("llm: vertex config: %w", err)
		}
		if !strings.HasPrefix(cfg.Vertex.Model, "claude") {
			return nil, fmt.Errorf("llm: vertex: model %q is not supported; only claude-* models are supported via the Anthropic SDK on Vertex", cfg.Vertex.Model)
		}
		creds, err := google.FindDefaultCredentials(ctx)
		if err != nil {
			return nil, fmt.Errorf("llm: vertex: google application default credentials: %w", err)
		}
		opt := vertexpkg.WithCredentials(ctx, cfg.Vertex.Region, cfg.Vertex.ProjectID, creds)
		return anthropicprovider.NewWithOptions(cfg.Vertex.Model, opt), nil
	default:
		return nil, fmt.Errorf("llm: invalid backend %q", o.backend)
	}
}
