package llm

import (
	"github.com/go-playground/validator/v10"
	anthropicprovider "github.com/iamhectordev/hector/pkg/llm/providers/anthropic"
	openaiprovider "github.com/iamhectordev/hector/pkg/llm/providers/openai"
)

var validate = validator.New(validator.WithRequiredStructEnabled())

// Provider selects which completer implementation to construct.
type Provider string

const (
	ProviderEcho      Provider = "echo"
	ProviderOpenAI    Provider = "openai"
	ProviderAnthropic Provider = "anthropic"
)

type BodyLogConfig struct {
	Enabled bool   `yaml:"enabled" env:"LLM_BODY_LOG_ENABLED" default:"false"`
	Dir     string `yaml:"dir" env:"LLM_BODY_LOG_DIR" default:"sessions/llm"`
}

type Config struct {
	DefaultProvider Provider               `yaml:"default_provider" env:"LLM_DEFAULT_PROVIDER" default:"echo" validate:"oneof=echo openai anthropic"`
	BodyLog         BodyLogConfig          `yaml:"body_log"`
	OpenAI          openaiprovider.Config  `yaml:"openai"`
	Anthropic       anthropicprovider.Config `yaml:"anthropic"`
}
