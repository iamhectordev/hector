package search

type ProviderName string

const ProviderTavily ProviderName = "tavily"

type Config struct {
	Provider ProviderName `yaml:"provider" env:"WEB_SEARCH_PROVIDER" validate:"omitempty,oneof=tavily"`
	Tavily   TavilyConfig `yaml:"tavily"`
}

func (c Config) Enabled() bool {
	return c.Provider != ""
}
