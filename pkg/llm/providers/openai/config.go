package openai

import "github.com/doron-cohen/klee/secrets"

type Config struct {
	APIKey secrets.Secret `yaml:"api_key" env:"OPENAI_API_KEY" secret:"openai_api_key"`
	Model  string         `yaml:"model" env:"OPENAI_MODEL" default:"gpt-4o-mini"`
}
