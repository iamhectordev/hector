package embed

import (
	"context"
	"fmt"
)

// Embedder converts text to a vector representation.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

// Provider selects which Embedder implementation to construct.
type Provider string

const (
	ProviderEcho   Provider = "echo"
	ProviderOpenAI Provider = "openai"
)

// Config selects a provider and holds its settings.
type Config struct {
	Provider Provider    `yaml:"provider" env:"EMBED_PROVIDER" default:"openai"`
	OpenAI   OpenAIConfig `yaml:"openai"`
}

// New constructs an Embedder from cfg.
func New(cfg Config) (Embedder, error) {
	switch cfg.Provider {
	case ProviderEcho:
		return &EchoEmbedder{}, nil
	case ProviderOpenAI, "":
		return newOpenAI(cfg.OpenAI), nil
	default:
		return nil, fmt.Errorf("embed: unknown provider %q", cfg.Provider)
	}
}
