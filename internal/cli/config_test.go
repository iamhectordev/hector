package cli_test

import (
	"testing"

	"github.com/iamhectordev/hector/internal/cli"
	"github.com/iamhectordev/hector/internal/web/search"
	"github.com/stretchr/testify/require"
)

func TestConfigComposesWebSearchConfig(t *testing.T) {
	t.Parallel()

	cfg := cli.Config{
		WebSearch: search.Config{
			Provider: search.ProviderTavily,
			Tavily: search.TavilyConfig{
				APIKey: "test-key",
			},
		},
	}

	require.True(t, cfg.WebSearch.Configured())
}
