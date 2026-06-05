package embed

import (
	"context"
	"fmt"
	"strings"

	sdkopenai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

const defaultModel = "text-embedding-3-small"

// OpenAIConfig holds credentials and model selection for the OpenAI embeddings API.
type OpenAIConfig struct {
	APIKey string `yaml:"api_key" env:"OPENAI_API_KEY"    validate:"required"`
	Model  string `yaml:"model"   env:"OPENAI_EMBED_MODEL" default:"text-embedding-3-small"`
}

// OpenAIEmbedder calls the OpenAI Embeddings API.
type OpenAIEmbedder struct {
	inner sdkopenai.Client
	model string
}

func newOpenAI(cfg OpenAIConfig) *OpenAIEmbedder {
	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		model = defaultModel
	}
	return &OpenAIEmbedder{
		inner: sdkopenai.NewClient(option.WithAPIKey(cfg.APIKey)),
		model: model,
	}
}

// Embed returns the embedding vector for text.
func (e *OpenAIEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	resp, err := e.inner.Embeddings.New(ctx, sdkopenai.EmbeddingNewParams{
		Input: sdkopenai.EmbeddingNewParamsInputUnion{
			OfString: sdkopenai.String(text),
		},
		Model: sdkopenai.EmbeddingModel(e.model),
	})
	if err != nil {
		return nil, fmt.Errorf("embed: %w", err)
	}
	if resp == nil || len(resp.Data) == 0 {
		return nil, fmt.Errorf("embed: empty response")
	}

	// OpenAI serialises float32 vectors as float64 in JSON.
	// The downcast is safe: text-embedding-3-small produces float32-precision values.
	f64 := resp.Data[0].Embedding
	vec := make([]float32, len(f64))
	for i, v := range f64 {
		vec[i] = float32(v)
	}
	return vec, nil
}
