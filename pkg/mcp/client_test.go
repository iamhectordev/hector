package mcp_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	hectormcp "github.com/iamhectordev/hector/pkg/mcp"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

func TestClientListsAndCallsToolsOverStdio(t *testing.T) {
	t.Parallel()

	executable, err := os.Executable()
	require.NoError(t, err)
	client, err := hectormcp.NewClient(hectormcp.Config{
		Transport: hectormcp.TransportStdio,
		Stdio: hectormcp.StdioConfig{
			Command: executable,
			Args:    []string{"-test.run=TestFakeMCPServer", "--"},
			Env:     map[string]string{"HECTOR_MCP_TEST_SERVER": "1"},
		},
	})
	require.NoError(t, err)
	require.NoError(t, client.Start(t.Context()))
	t.Cleanup(func() {
		require.NoError(t, client.Close())
	})

	tools, err := client.ListTools(t.Context())
	require.NoError(t, err)
	require.Len(t, tools, 1)
	require.Equal(t, "repos.list", tools[0].Name)
	require.JSONEq(t, `{"type":"object","properties":{"owner":{"type":"string"}},"required":["owner"],"additionalProperties":false}`, string(tools[0].InputSchema))

	result, err := client.CallTool(t.Context(), "repos.list", json.RawMessage(`{"owner":"iamhectordev"}`))
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.Len(t, result.Content, 1)
	require.Equal(t, "listed repos", result.Content[0].Text)
}

func TestClientRejectsInvalidTaggedUnionConfig(t *testing.T) {
	t.Parallel()

	_, err := hectormcp.NewClient(hectormcp.Config{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "mcp: transport is required")

	_, err = hectormcp.NewClient(hectormcp.Config{Transport: hectormcp.TransportStreamableHTTP})
	require.Error(t, err)
	require.Contains(t, err.Error(), "mcp: streamable_http url is required")
}

func TestClientListsAndCallsToolsOverStreamableHTTP(t *testing.T) {
	t.Parallel()

	var gotAuthorization string
	server := newFakeSDKServer()
	handler := sdkmcp.NewStreamableHTTPHandler(func(*http.Request) *sdkmcp.Server {
		return server
	}, nil)
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuthorization = r.Header.Get("Authorization")
		handler.ServeHTTP(w, r)
	}))
	t.Cleanup(httpServer.Close)

	client, err := hectormcp.NewClient(hectormcp.Config{
		Transport: hectormcp.TransportStreamableHTTP,
		StreamableHTTP: hectormcp.StreamableHTTPConfig{
			URL:                  httpServer.URL,
			Headers:              map[string]string{"Authorization": "Bearer test-token"},
			DisableStandaloneSSE: true,
		},
	})
	require.NoError(t, err)
	require.NoError(t, client.Start(t.Context()))
	t.Cleanup(func() {
		require.NoError(t, client.Close())
	})

	tools, err := client.ListTools(t.Context())
	require.NoError(t, err)
	require.Len(t, tools, 1)
	require.Equal(t, "Bearer test-token", gotAuthorization)

	result, err := client.CallTool(t.Context(), "repos.list", json.RawMessage(`{"owner":"iamhectordev"}`))
	require.NoError(t, err)
	require.Equal(t, "listed repos", result.Content[0].Text)
}

func TestFakeMCPServer(t *testing.T) {
	if os.Getenv("HECTOR_MCP_TEST_SERVER") != "1" {
		return
	}

	server := newFakeSDKServer()
	if err := server.Run(context.Background(), &sdkmcp.StdioTransport{}); err != nil {
		t.Fatal(err)
	}
}

func newFakeSDKServer() *sdkmcp.Server {
	server := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "fake", Version: "test"}, nil)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "repos.list",
		Description: "Lists repositories.",
	}, func(_ context.Context, _ *sdkmcp.CallToolRequest, input listReposInput) (*sdkmcp.CallToolResult, listReposOutput, error) {
		return &sdkmcp.CallToolResult{
			Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: "listed repos"}},
		}, listReposOutput{OK: true}, nil
	})
	return server
}

type listReposInput struct {
	Owner string `json:"owner"`
}

type listReposOutput struct {
	OK bool `json:"ok"`
}
