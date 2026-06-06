package tools_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/iamhectordev/hector/internal/mcp"
	"github.com/iamhectordev/hector/modules/tools"
	"github.com/stretchr/testify/require"
)

func TestMCPToolAdapterNormalizesNameAndCallsOriginalTool(t *testing.T) {
	t.Parallel()

	client := &fakeMCPClient{}
	tool, err := tools.NewMCPTool("github", client, mcp.Tool{
		Name:        "repos.list",
		Description: "Lists repositories.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"owner":{"type":"string"}}}`),
	})
	require.NoError(t, err)

	def := tool.Definition()
	require.Equal(t, "github_repos_list", def.Name)
	require.Equal(t, "Lists repositories.", def.Description)
	require.JSONEq(t, `{"type":"object","properties":{"owner":{"type":"string"}}}`, string(def.Parameters))

	output, err := tool.Run(t.Context(), json.RawMessage(`{"owner":"iamhectordev"}`))
	require.NoError(t, err)
	require.JSONEq(t, `{"status":"ok","result":{"content":[{"type":"text","text":"listed repos"}],"isError":false}}`, output)
	require.Equal(t, "repos.list", client.calledName)
	require.JSONEq(t, `{"owner":"iamhectordev"}`, string(client.calledArgs))
}

func TestMCPToolRunTracesCall(t *testing.T) {
	client := &fakeMCPClient{}
	tool, err := tools.NewMCPTool("github", client, mcp.Tool{
		Name:        "repos.list",
		Description: "Lists repositories.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"owner":{"type":"string"}}}`),
	})
	require.NoError(t, err)
	recorder := newSpanRecorder(t)

	output, err := tool.Run(t.Context(), json.RawMessage(`{"owner":"iamhectordev"}`))
	require.NoError(t, err)
	require.NotEmpty(t, output)

	span := findSpan(t, recorder.Ended(), "github.mcp.call")
	require.Equal(t, "github", requireSpanAttr(t, span, "mcp.server"))
	require.Equal(t, "repos.list", requireSpanAttr(t, span, "mcp.tool_name"))
	require.Equal(t, int64(1), requireSpanAttrInt(t, span, "mcp.content_count"))
}

type fakeMCPClient struct {
	calledName string
	calledArgs json.RawMessage
}

func (f *fakeMCPClient) CallTool(_ context.Context, name string, args json.RawMessage) (mcp.ToolResult, error) {
	f.calledName = name
	f.calledArgs = append(json.RawMessage{}, args...)
	return mcp.ToolResult{
		Content: []mcp.Content{{Type: "text", Text: "listed repos"}},
		IsError: false,
	}, nil
}
