package anthropic

import "github.com/doron-cohen/klee/secrets"

type Config struct {
	APIKey secrets.Secret `yaml:"api_key" env:"ANTHROPIC_API_KEY" secret:"anthropic_api_key"`
	Model  string         `yaml:"model"   env:"ANTHROPIC_MODEL"   default:"claude-opus-4-7"`
}
