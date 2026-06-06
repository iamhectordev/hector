package tools_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

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
	ts := time.Date(2026, 3, 15, 9, 30, 0, 0, time.UTC)
	searcher := &stubMemorySearcher{
		results: []memory.Object{
			{ID: "1", Content: "the auth service is written in go", CreatedAt: ts},
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
	require.Contains(t, env["result"], "2026-03-15")
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

func TestMemRecallRunTracesQueryMetadata(t *testing.T) {
	recorder := newSpanRecorder(t)
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
	require.NotEmpty(t, output)

	span := findSpan(t, recorder.Ended(), "memory.recall")
	require.Equal(t, int64(len("auth service language")), requireSpanAttrInt(t, span, "memory.query_length"))
	require.Equal(t, int64(1), requireSpanAttrInt(t, span, "memory.result_count"))
}
