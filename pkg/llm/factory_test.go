package llm_test

import (
	"testing"

	"github.com/iamhectordev/hector/pkg/llm"
	"github.com/iamhectordev/hector/pkg/llm/providers/echo"
	openaiprovider "github.com/iamhectordev/hector/pkg/llm/providers/openai"
	"github.com/stretchr/testify/require"
)

func TestNew_DefaultsToEcho(t *testing.T) {
	t.Parallel()

	completer, err := llm.New(llm.Config{})
	require.NoError(t, err)
	require.IsType(t, &echo.Completer{}, completer)
}

func TestNew_WithProviderOverridesDefault(t *testing.T) {
	t.Parallel()

	completer, err := llm.New(
		llm.Config{},
		llm.WithProvider(llm.ProviderOpenAI),
	)
	require.Error(t, err)
	require.Nil(t, completer)
}

func TestNew_OpenAIReturnsProvider(t *testing.T) {
	t.Parallel()

	completer, err := llm.New(llm.Config{
		DefaultProvider: llm.ProviderOpenAI,
		OpenAI: openaiprovider.Config{
			APIKey: "sk-test",
		},
	})
	require.NoError(t, err)
	require.IsType(t, &openaiprovider.Completer{}, completer)
}

func TestNew_OpenAIRequiresAPIKey(t *testing.T) {
	t.Parallel()

	completer, err := llm.New(llm.Config{
		DefaultProvider: llm.ProviderOpenAI,
	})
	require.Error(t, err)
	require.Nil(t, completer)
}

func TestNew_RejectsUnknownProvider(t *testing.T) {
	t.Parallel()

	completer, err := llm.New(llm.Config{
		DefaultProvider: llm.Provider("wat"),
	})
	require.Error(t, err)
	require.Nil(t, completer)
}
