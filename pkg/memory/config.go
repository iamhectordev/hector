package memory

import "github.com/iamhectordev/hector/internal/embed"

// Config holds memory subsystem settings.
type Config struct {
	EmbedEnabled bool         `yaml:"embed_enabled" env:"MEMORY_EMBED_ENABLED" default:"false"`
	Embed        embed.Config `yaml:"embed"`
}
