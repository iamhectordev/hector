package vertex

type Config struct {
	ProjectID string `yaml:"project_id" env:"VERTEX_PROJECT_ID" validate:"required"`
	Region    string `yaml:"region"     env:"VERTEX_REGION"     validate:"required"`
	Model     string `yaml:"model"      env:"VERTEX_MODEL"      default:"claude-opus-4-7"`
}
