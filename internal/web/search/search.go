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

type Result struct {
	Provider ProviderName `json:"provider"`
	URL      string       `json:"url"`
	Title    string       `json:"title,omitempty"`
	Snippet  string       `json:"snippet,omitempty"`
	Score    *float64     `json:"score,omitempty"`
}
