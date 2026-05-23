package github_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"strconv"
	"testing"

	hectorgithub "github.com/iamhectordev/hector/modules/github"
	"github.com/iamhectordev/hector/modules/tools"
	"github.com/stretchr/testify/require"
)

func TestModuleInitLogsVerificationIssueTitle(t *testing.T) {
	t.Parallel()

	privateKeyPath := writeTestPrivateKey(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/app/installations/456/access_tokens":
			w.WriteHeader(http.StatusCreated)
			_, err := fmt.Fprint(w, `{"token":"installation-token","expires_at":"2026-05-22T12:00:00Z"}`)
			require.NoError(t, err)
		case "/repos/iamhectordev/hector/issues/1":
			_, err := fmt.Fprint(w, `{"id":99,"number":1,"title":"Replace me before real use","state":"open","html_url":"https://github.com/replace-owner/replace-repo/issues/1","user":{"login":"alice"}}`)
			require.NoError(t, err)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	module, err := hectorgithub.NewModule(hectorgithub.Config{
		AppID:          123,
		InstallationID: 456,
		PrivateKeyPath: privateKeyPath,
		APIURL:         server.URL,
	}, hectorgithub.WithLogger(logger))
	require.NoError(t, err)

	require.NoError(t, module.Init(t.Context()))
	require.Contains(t, logs.String(), "github integration verified")
	require.Contains(t, logs.String(), `issue_title="Replace me before real use"`)
}

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
		case "/repos/iamhectordev/hector/issues/1":
			_, err := fmt.Fprint(w, `{"id":99,"number":1,"title":"GitHub MCP wiring","state":"open","html_url":"https://github.com/iamhectordev/hector/issues/1","user":{"login":"alice"}}`)
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
			Command: executable,
			Args:    []string{"-test.run=TestGitHubFakeMCPServer", "--"},
			Env:     map[string]string{"HECTOR_GITHUB_MCP_TEST_SERVER": "1"},
		},
	}, hectorgithub.WithToolRegistrar(registry))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, module.Stop(t.Context()))
	})

	require.NoError(t, module.Init(t.Context()))

	defs := registry.Definitions()
	require.Len(t, defs, 1)
	require.Equal(t, "github_repos_list", defs[0].Name)

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

	reader := bufio.NewReader(os.Stdin)
	for {
		request, err := readMCPMessage(reader)
		if err != nil {
			if err == io.EOF {
				os.Exit(0)
			}
			_, _ = fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		id := request["id"]
		switch request["method"] {
		case "tools/list":
			writeMCPMessage(map[string]any{
				"jsonrpc": "2.0",
				"id":      id,
				"result": map[string]any{
					"tools": []map[string]any{{
						"name":        "repos.list",
						"description": "Lists repositories.",
						"inputSchema": map[string]any{
							"type":       "object",
							"properties": map[string]any{"owner": map[string]any{"type": "string"}},
						},
					}},
				},
			})
		case "tools/call":
			writeMCPMessage(map[string]any{
				"jsonrpc": "2.0",
				"id":      id,
				"result": map[string]any{
					"content": []map[string]any{{"type": "text", "text": "listed repos"}},
					"isError": false,
				},
			})
		default:
			writeMCPMessage(map[string]any{
				"jsonrpc": "2.0",
				"id":      id,
				"error":   map[string]any{"code": -32601, "message": "unknown method"},
			})
		}
	}
}

func readMCPMessage(reader *bufio.Reader) (map[string]any, error) {
	headers, err := textproto.NewReader(reader).ReadMIMEHeader()
	if err != nil {
		return nil, err
	}
	length, err := strconv.Atoi(headers.Get("Content-Length"))
	if err != nil {
		return nil, err
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(reader, body); err != nil {
		return nil, err
	}
	var message map[string]any
	if err := json.Unmarshal(body, &message); err != nil {
		return nil, err
	}
	return message, nil
}

func writeMCPMessage(message map[string]any) {
	body, err := json.Marshal(message)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	_, _ = fmt.Fprintf(os.Stdout, "Content-Length: %d\r\n\r\n%s", len(body), body)
}
