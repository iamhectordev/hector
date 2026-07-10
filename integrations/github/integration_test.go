package github_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	githubpkg "github.com/iamhectordev/hector/integrations/github"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

func TestNewIntegrationHasAllTools(t *testing.T) {
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

	executable, err := os.Executable()
	require.NoError(t, err)

	integration, err := githubpkg.New(t.Context(), githubpkg.Config{
		Enabled:        true,
		AppID:          123,
		InstallationID: 456,
		PrivateKeyPath: privateKeyPath,
		APIURL:         server.URL,
		MCP: githubpkg.MCPConfig{
			Transport:    "stdio",
			StdioCommand: executable,
			StdioArgs:    []string{"-test.run=TestGitHubFakeMCPServer", "--"},
			StdioEnv:     map[string]string{"HECTOR_GITHUB_MCP_TEST_SERVER": "1"},
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, integration.Close())
	})

	require.Equal(t, "github", integration.Name())

	toolList := integration.Tools()
	names := make([]string, 0, len(toolList))
	for _, tool := range toolList {
		names = append(names, tool.Definition().Name)
	}
	require.Equal(t, []string{
		"get_issue",
		"create_milestone",
		"list_milestones",
		"update_milestone",
		"create_blocked_by_relationship",
		"remove_blocked_by_relationship",
		"github_repos_list",
	}, names)
}

func TestNewIntegrationWithoutMCP(t *testing.T) {
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

	integration, err := githubpkg.New(t.Context(), githubpkg.Config{
		Enabled:        true,
		AppID:          123,
		InstallationID: 456,
		PrivateKeyPath: privateKeyPath,
		APIURL:         server.URL,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, integration.Close())
	})

	toolList := integration.Tools()
	names := make([]string, 0, len(toolList))
	for _, tool := range toolList {
		names = append(names, tool.Definition().Name)
	}
	require.Equal(t, []string{
		"get_issue",
		"create_milestone",
		"list_milestones",
		"update_milestone",
		"create_blocked_by_relationship",
		"remove_blocked_by_relationship",
	}, names)
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
