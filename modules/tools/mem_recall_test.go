package tools_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/iamhectordev/hector/modules/tools"
	"github.com/iamhectordev/hector/pkg/memory"
	"github.com/stretchr/testify/require"
)

type stubMemorySearcher struct {
	results []memory.Object
}

func (s *stubMemorySearcher) Search(_ context.Context, _ string, _ int) ([]memory.Object, error) {
	return s.results, nil
}

func TestMemRecallDefinition(t *testing.T) {
	tool, err := tools.NewMemRecall(&stubMemorySearcher{})
	require.NoError(t, err)

	def := tool.Definition()
	require.Equal(t, "mem_recall", def.Name)
	require.NotEmpty(t, def.Description)
	require.NotEmpty(t, def.Parameters)
}

func TestMemRecallRunReturnsMatchingContent(t *testing.T) {
	searcher := &stubMemorySearcher{
		results: []memory.Object{
			{ID: "1", Content: "the auth service is written in go"},
		},
	}
	tool, err := tools.NewMemRecall(searcher)
	require.NoError(t, err)

	args, _ := json.Marshal(map[string]string{"query": "auth service language"})
	output, err := tool.Run(t.Context(), args)
	require.NoError(t, err)

	var env map[string]any
	require.NoError(t, json.Unmarshal([]byte(output), &env))
	require.Equal(t, "ok", env["status"])
	require.Contains(t, env["result"], "the auth service is written in go")
}

func TestMemRecallRunReturnsErrorEnvelopeOnEmptyQuery(t *testing.T) {
	tool, err := tools.NewMemRecall(&stubMemorySearcher{})
	require.NoError(t, err)

	args, _ := json.Marshal(map[string]string{"query": ""})
	output, err := tool.Run(t.Context(), args)
	require.NoError(t, err)

	var env map[string]any
	require.NoError(t, json.Unmarshal([]byte(output), &env))
	require.Equal(t, "error", env["status"])
}
