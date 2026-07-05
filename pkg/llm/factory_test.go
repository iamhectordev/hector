package llm_test

import (
	"testing"

	"github.com/iamhectordev/hector/pkg/llm"
	anthropicprovider "github.com/iamhectordev/hector/pkg/llm/providers/anthropic"
	"github.com/iamhectordev/hector/pkg/llm/providers/echo"
	openaiprovider "github.com/iamhectordev/hector/pkg/llm/providers/openai"
	"github.com/stretchr/testify/require"
)

func TestNew_DefaultsToEcho(t *testing.T) {
	t.Parallel()

	completer, err := llm.New(&llm.Config{})
	require.NoError(t, err)
	require.IsType(t, &echo.Completer{}, completer)
}

func TestNew_WithProviderOverridesDefault(t *testing.T) {
	t.Parallel()

	completer, err := llm.New(
		&llm.Config{},
		llm.WithProvider(llm.ProviderOpenAI),
	)
	require.Error(t, err)
	require.Nil(t, completer)
}

func TestNew_OpenAIReturnsProvider(t *testing.T) {
	t.Parallel()

	cfg := &llm.Config{
		DefaultProvider: llm.ProviderOpenAI,
		OpenAI:          openaiprovider.Config{},
	}
	require.NoError(t, cfg.OpenAI.APIKey.UnmarshalText([]byte("sk-test")))
	completer, err := llm.New(cfg)
	require.NoError(t, err)
	require.IsType(t, &openaiprovider.Completer{}, completer)
}

func TestNew_OpenAIRequiresAPIKey(t *testing.T) {
	t.Parallel()

	completer, err := llm.New(&llm.Config{
		DefaultProvider: llm.ProviderOpenAI,
	})
	require.Error(t, err)
	require.Nil(t, completer)
}

func TestNew_AnthropicReturnsProvider(t *testing.T) {
	t.Parallel()

	cfg := &llm.Config{
		DefaultProvider: llm.ProviderAnthropic,
		Anthropic:       anthropicprovider.Config{},
	}
	require.NoError(t, cfg.Anthropic.APIKey.UnmarshalText([]byte("sk-ant-test")))
	completer, err := llm.New(cfg)
	require.NoError(t, err)
	require.IsType(t, &anthropicprovider.Completer{}, completer)
}

func TestNew_AnthropicRequiresAPIKey(t *testing.T) {
	t.Parallel()

	completer, err := llm.New(&llm.Config{
		DefaultProvider: llm.ProviderAnthropic,
	})
	require.Error(t, err)
	require.Nil(t, completer)
}

func TestNew_RejectsUnknownProvider(t *testing.T) {
	t.Parallel()

	completer, err := llm.New(&llm.Config{
		DefaultProvider: llm.Provider("wat"),
	})
	require.Error(t, err)
	require.Nil(t, completer)
}
