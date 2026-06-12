package agent

// Config holds agent module settings.
type Config struct {
	Perception PerceptionConfig `yaml:"perception"`
}

// PerceptionConfig controls the pre-turn gate.
type PerceptionConfig struct {
	Enabled bool `yaml:"enabled" env:"HECTOR_AGENT_PERCEPTION_ENABLED"`
}
