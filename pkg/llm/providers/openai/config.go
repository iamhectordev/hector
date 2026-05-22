package openai

type Config struct {
	APIKey string `yaml:"api_key" env:"OPENAI_API_KEY" validate:"required"`
	Model  string `yaml:"model" env:"OPENAI_MODEL" default:"gpt-4o-mini"`
}
