package github_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/iamhectordev/hector/internal/mcp"
	hectorgithub "github.com/iamhectordev/hector/modules/github"
	"github.com/iamhectordev/hector/modules/tools"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

func TestConfigConfigured(t *testing.T) {
	t.Parallel()

	require.False(t, hectorgithub.Config{}.Configured())
	require.True(t, hectorgithub.Config{AppID: 123}.Configured())
}

func TestModuleInitRegistersMCPTools(t *testing.T) {
	t.Parallel()

	privateKeyPath := writeTestPrivateKey(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/app/installations/456/access_tokens":
			w.WriteHeader(http.StatusCreated)
			_, err := fmt.Fprint(w, `{"token":"installation-token","expires_at":"2026-05-22T12:00:00Z"}`)
			require.NoError(t, err)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	registry, err := tools.NewRegistry()
	require.NoError(t, err)
	executable, err := os.Executable()
	require.NoError(t, err)
	module, err := hectorgithub.NewModule(hectorgithub.Config{
		AppID:          123,
		InstallationID: 456,
		PrivateKeyPath: privateKeyPath,
		APIURL:         server.URL,
		MCP: hectorgithub.MCPConfig{
			Transport:    mcp.TransportStdio,
			StdioCommand: executable,
			StdioArgs:    []string{"-test.run=TestGitHubFakeMCPServer", "--"},
			StdioEnv:     map[string]string{"HECTOR_GITHUB_MCP_TEST_SERVER": "1"},
		},
	}, hectorgithub.WithToolRegistrar(registry))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, module.Stop(t.Context()))
	})

	require.NoError(t, module.Init(t.Context()))

	defs := registry.Definitions()
	names := make([]string, 0, len(defs))
	for _, def := range defs {
		names = append(names, def.Name)
	}
	require.Equal(t, []string{
		"create_blocked_by_relationship",
		"create_milestone",
		"get_issue",
		"github_repos_list",
		"list_milestones",
		"remove_blocked_by_relationship",
		"update_milestone",
	}, names)

	output, err := registry.Run(t.Context(), "github_repos_list", json.RawMessage(`{"owner":"iamhectordev"}`))
	require.NoError(t, err)
	require.JSONEq(t, `{"status":"ok","result":{"content":[{"type":"text","text":"listed repos"}],"isError":false}}`, output)
}

func TestGitHubFakeMCPServer(t *testing.T) {
	if os.Getenv("HECTOR_GITHUB_MCP_TEST_SERVER") != "1" {
		return
	}
	if os.Getenv("GITHUB_PERSONAL_ACCESS_TOKEN") != "installation-token" {
		_, _ = fmt.Fprintln(os.Stderr, "missing installation token")
		os.Exit(1)
	}

	server := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "fake-github", Version: "test"}, nil)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "repos.list",
		Description: "Lists repositories.",
	}, func(_ context.Context, _ *sdkmcp.CallToolRequest, input listReposInput) (*sdkmcp.CallToolResult, listReposOutput, error) {
		return &sdkmcp.CallToolResult{
			Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: "listed repos"}},
		}, listReposOutput{OK: true}, nil
	})
	if err := server.Run(context.Background(), &sdkmcp.StdioTransport{}); err != nil {
		t.Fatal(err)
	}
}

type listReposInput struct {
	Owner string `json:"owner"`
}

type listReposOutput struct {
	OK bool `json:"ok"`
}
