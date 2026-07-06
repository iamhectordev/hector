package llm_test

import (
	"testing"

	"golang.org/x/oauth2/google"

	"github.com/iamhectordev/hector/pkg/llm"
	anthropicprovider "github.com/iamhectordev/hector/pkg/llm/providers/anthropic"
	"github.com/iamhectordev/hector/pkg/llm/providers/echo"
	openaiprovider "github.com/iamhectordev/hector/pkg/llm/providers/openai"
	vertexprovider "github.com/iamhectordev/hector/pkg/llm/providers/vertex"
	"github.com/stretchr/testify/require"
)

func TestNew_DefaultsToEcho(t *testing.T) {
	t.Parallel()

	completer, err := llm.New(t.Context(), &llm.Config{})
	require.NoError(t, err)
	require.IsType(t, &echo.Completer{}, completer)
}

func TestNew_WithBackendOverridesDefault(t *testing.T) {
	t.Parallel()

	completer, err := llm.New(
		t.Context(),
		&llm.Config{},
		llm.WithBackend(llm.BackendOpenAI),
	)
	require.Error(t, err)
	require.Nil(t, completer)
}

func TestNew_OpenAIReturnsCompleter(t *testing.T) {
	t.Parallel()

	cfg := &llm.Config{
		DefaultBackend: llm.BackendOpenAI,
		OpenAI:         openaiprovider.Config{},
	}
	require.NoError(t, cfg.OpenAI.APIKey.UnmarshalText([]byte("sk-test")))
	completer, err := llm.New(t.Context(), cfg)
	require.NoError(t, err)
	require.IsType(t, &openaiprovider.Completer{}, completer)
}

func TestNew_OpenAIRequiresAPIKey(t *testing.T) {
	t.Parallel()

	completer, err := llm.New(t.Context(), &llm.Config{
		DefaultBackend: llm.BackendOpenAI,
	})
	require.Error(t, err)
	require.Nil(t, completer)
}

func TestNew_AnthropicReturnsCompleter(t *testing.T) {
	t.Parallel()

	cfg := &llm.Config{
		DefaultBackend: llm.BackendAnthropic,
		Anthropic:      anthropicprovider.Config{},
	}
	require.NoError(t, cfg.Anthropic.APIKey.UnmarshalText([]byte("sk-ant-test")))
	completer, err := llm.New(t.Context(), cfg)
	require.NoError(t, err)
	require.IsType(t, &anthropicprovider.Completer{}, completer)
}

func TestNew_AnthropicRequiresAPIKey(t *testing.T) {
	t.Parallel()

	completer, err := llm.New(t.Context(), &llm.Config{
		DefaultBackend: llm.BackendAnthropic,
	})
	require.Error(t, err)
	require.Nil(t, completer)
}

func TestNew_RejectsUnknownBackend(t *testing.T) {
	t.Parallel()

	completer, err := llm.New(t.Context(), &llm.Config{
		DefaultBackend: llm.Backend("wat"),
	})
	require.Error(t, err)
	require.Nil(t, completer)
}

func TestNew_VertexWithClaudeModelReturnsAnthropicCompleter(t *testing.T) {
	t.Parallel()

	if _, err := google.FindDefaultCredentials(t.Context()); err != nil {
		t.Skip("google application default credentials not available")
	}

	cfg := &llm.Config{
		DefaultBackend: llm.BackendVertex,
		Vertex: vertexprovider.Config{
			ProjectID: "my-project",
			Region:    "us-east5",
			Model:     "claude-opus-4-7",
		},
	}
	completer, err := llm.New(t.Context(), cfg)
	require.NoError(t, err)
	require.IsType(t, &anthropicprovider.Completer{}, completer)
}

func TestNew_VertexRequiresProjectID(t *testing.T) {
	t.Parallel()

	cfg := &llm.Config{
		DefaultBackend: llm.BackendVertex,
		Vertex: vertexprovider.Config{
			Region: "us-east5",
			Model:  "claude-opus-4-7",
		},
	}
	completer, err := llm.New(t.Context(), cfg)
	require.Error(t, err)
	require.Nil(t, completer)
}

func TestNew_VertexRequiresRegion(t *testing.T) {
	t.Parallel()

	cfg := &llm.Config{
		DefaultBackend: llm.BackendVertex,
		Vertex: vertexprovider.Config{
			ProjectID: "my-project",
			Model:     "claude-opus-4-7",
		},
	}
	completer, err := llm.New(t.Context(), cfg)
	require.Error(t, err)
	require.Nil(t, completer)
}

func TestNew_VertexWithNonClaudeModelErrors(t *testing.T) {
	t.Parallel()

	cfg := &llm.Config{
		DefaultBackend: llm.BackendVertex,
		Vertex: vertexprovider.Config{
			ProjectID: "my-project",
			Region:    "us-east5",
			Model:     "gemini-2.0-flash",
		},
	}
	completer, err := llm.New(t.Context(), cfg)
	require.Error(t, err)
	require.Nil(t, completer)
}
