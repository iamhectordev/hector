package llm

import "github.com/go-playground/validator/v10"

var validate = validator.New(validator.WithRequiredStructEnabled())

// Provider selects which completer implementation to construct.
type Provider string

const (
	ProviderEcho   Provider = "echo"
	ProviderOpenAI Provider = "openai"
)

type OpenAIConfig struct {
	APIKey string `yaml:"api_key" env:"OPENAI_API_KEY" validate:"required"`
	Model  string `yaml:"model" env:"OPENAI_MODEL" default:"gpt-4o-mini"`
}

type Config struct {
	DefaultProvider Provider     `yaml:"default_provider" env:"LLM_DEFAULT_PROVIDER" default:"echo" validate:"oneof=echo openai"`
	OpenAI          OpenAIConfig `yaml:"openai"`
}
