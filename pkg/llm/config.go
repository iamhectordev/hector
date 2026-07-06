package llm

import (
	"github.com/go-playground/validator/v10"
	anthropicprovider "github.com/iamhectordev/hector/pkg/llm/providers/anthropic"
	openaiprovider "github.com/iamhectordev/hector/pkg/llm/providers/openai"
	vertexprovider "github.com/iamhectordev/hector/pkg/llm/providers/vertex"
)

var validate = validator.New(validator.WithRequiredStructEnabled())

// Backend selects the deployment platform (Anthropic direct, OpenAI, Vertex, etc.)
// and determines both the SDK client and the authentication method.
type Backend string

const (
	BackendEcho      Backend = "echo"
	BackendOpenAI    Backend = "openai"
	BackendAnthropic Backend = "anthropic"
	BackendVertex    Backend = "vertex"
)

type BodyLogConfig struct {
	Enabled bool   `yaml:"enabled" env:"LLM_BODY_LOG_ENABLED" default:"false"`
	Dir     string `yaml:"dir" env:"LLM_BODY_LOG_DIR" default:"sessions/llm"`
}

type Config struct {
	DefaultBackend Backend                  `yaml:"default_backend" env:"LLM_DEFAULT_BACKEND" default:"echo" validate:"oneof=echo openai anthropic vertex"`
	BodyLog        BodyLogConfig            `yaml:"body_log"`
	OpenAI         openaiprovider.Config    `yaml:"openai"`
	Anthropic      anthropicprovider.Config `yaml:"anthropic"`
	Vertex         vertexprovider.Config    `yaml:"vertex"`
}
