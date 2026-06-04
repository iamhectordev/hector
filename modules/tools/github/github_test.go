package github_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	gh "github.com/iamhectordev/hector/internal/github"
	"github.com/iamhectordev/hector/modules/tools"
	githubtools "github.com/iamhectordev/hector/modules/tools/github"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

func TestRegisterAddsToolsToRegistry(t *testing.T) {
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

	closer, err := githubtools.Register(t.Context(), gh.Config{
		Enabled:        true,
		AppID:          123,
		InstallationID: 456,
		PrivateKeyPath: privateKeyPath,
		APIURL:         server.URL,
		MCP: gh.MCPConfig{
			Transport:    "stdio",
			StdioCommand: executable,
			StdioArgs:    []string{"-test.run=TestGitHubFakeMCPServer", "--"},
			StdioEnv:     map[string]string{"HECTOR_GITHUB_MCP_TEST_SERVER": "1"},
		},
	}, registry)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, closer.Close())
	})

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

func writeTestPrivateKey(t *testing.T) string {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	der := x509.MarshalPKCS1PrivateKey(key)
	block := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: der}
	path := filepath.Join(t.TempDir(), "github-app.pem")
	require.NoError(t, os.WriteFile(path, pem.EncodeToMemory(block), 0o600))
	return path
}
