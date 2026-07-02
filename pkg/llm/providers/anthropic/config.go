package anthropic

type Config struct {
	APIKey string `yaml:"api_key" env:"ANTHROPIC_API_KEY"`
	Model  string `yaml:"model"   env:"ANTHROPIC_MODEL"   default:"claude-opus-4-7"`
}
