package mcp_test

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/textproto"
	"os"
	"strconv"
	"testing"

	"github.com/iamhectordev/hector/internal/mcp"
	"github.com/stretchr/testify/require"
)

func TestClientListsAndCallsToolsOverStdio(t *testing.T) {
	t.Parallel()

	executable, err := os.Executable()
	require.NoError(t, err)
	client, err := mcp.NewClient(mcp.Config{
		Command: executable,
		Args:    []string{"-test.run=TestFakeMCPServer", "--"},
		Env:     map[string]string{"HECTOR_MCP_TEST_SERVER": "1"},
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
	require.JSONEq(t, `{"type":"object","properties":{"owner":{"type":"string"}}}`, string(tools[0].InputSchema))

	result, err := client.CallTool(t.Context(), "repos.list", json.RawMessage(`{"owner":"iamhectordev"}`))
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.Len(t, result.Content, 1)
	require.Equal(t, "listed repos", result.Content[0].Text)
}

func TestFakeMCPServer(t *testing.T) {
	if os.Getenv("HECTOR_MCP_TEST_SERVER") != "1" {
		return
	}

	reader := bufio.NewReader(os.Stdin)
	for {
		request, err := readMessage(reader)
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
			writeMessage(map[string]any{
				"jsonrpc": "2.0",
				"id":      id,
				"result": map[string]any{
					"tools": []map[string]any{{
						"name":        "repos.list",
						"description": "Lists repositories.",
						"inputSchema": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"owner": map[string]any{"type": "string"},
							},
						},
					}},
				},
			})
		case "tools/call":
			writeMessage(map[string]any{
				"jsonrpc": "2.0",
				"id":      id,
				"result": map[string]any{
					"content": []map[string]any{{
						"type": "text",
						"text": "listed repos",
					}},
					"isError": false,
				},
			})
		default:
			writeMessage(map[string]any{
				"jsonrpc": "2.0",
				"id":      id,
				"error": map[string]any{
					"code":    -32601,
					"message": "unknown method",
				},
			})
		}
	}
}

func readMessage(reader *bufio.Reader) (map[string]any, error) {
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

func writeMessage(message map[string]any) {
	body, err := json.Marshal(message)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	_, _ = fmt.Fprintf(os.Stdout, "Content-Length: %d\r\n\r\n%s", len(body), body)
}

var _ = context.Background
