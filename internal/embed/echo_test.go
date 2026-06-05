package embed

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEchoEmbedder_Embed_ReturnsDeterministicVector(t *testing.T) {
	t.Parallel()

	e := &EchoEmbedder{}
	vec1, err := e.Embed(t.Context(), "hello world")
	require.NoError(t, err)
	require.NotEmpty(t, vec1)

	vec2, err := e.Embed(t.Context(), "hello world")
	require.NoError(t, err)
	require.Equal(t, vec1, vec2)
}

func TestEchoEmbedder_Embed_DifferentInputsDifferentVectors(t *testing.T) {
	t.Parallel()

	e := &EchoEmbedder{}
	vec1, err := e.Embed(t.Context(), "hello")
	require.NoError(t, err)

	vec2, err := e.Embed(t.Context(), "world")
	require.NoError(t, err)

	require.NotEqual(t, vec1, vec2)
}

func TestNew_EchoProvider(t *testing.T) {
	t.Parallel()

	embedder, err := New(Config{Provider: ProviderEcho})
	require.NoError(t, err)
	require.NotNil(t, embedder)
}
