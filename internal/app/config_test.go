package app_test

import (
	"testing"

	"github.com/iamhectordev/hector/internal/app"
	"github.com/iamhectordev/hector/internal/tracing"
	"github.com/iamhectordev/hector/internal/web/search"
	"github.com/stretchr/testify/require"
)

func TestConfigComposesWebSearchConfig(t *testing.T) {
	t.Parallel()

	cfg := app.Config{
		WebSearch: search.Config{
			Provider: search.ProviderTavily,
			Tavily: search.TavilyConfig{
				APIKey: "test-key",
			},
		},
	}

	require.True(t, cfg.WebSearch.Enabled())
}

func TestConfigComposesTracingConfig(t *testing.T) {
	t.Parallel()

	cfg := app.Config{
		Tracing: tracing.Config{
			Enabled:     true,
			ServiceName: "hector",
			SampleRatio: 0.5,
		},
	}

	require.True(t, cfg.Tracing.Enabled)
	require.Equal(t, "hector", cfg.Tracing.ServiceName)
	require.Equal(t, 0.5, cfg.Tracing.SampleRatio)
}
