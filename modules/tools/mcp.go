package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/iamhectordev/hector/internal/mcp"
	"github.com/iamhectordev/hector/pkg/telem"
)

var nonToolNameChar = regexp.MustCompile(`[^a-z0-9]+`)

type mcpClient interface {
	CallTool(context.Context, string, json.RawMessage) (mcp.ToolResult, error)
}

type MCPTool struct {
	server      string
	name        string
	description string
	parameters  json.RawMessage
	mcpName     string
	client      mcpClient
}

func NewMCPTool(prefix string, client mcpClient, tool mcp.Tool) (*MCPTool, error) {
	if client == nil {
		return nil, fmt.Errorf("tools: mcp client is required")
	}
	name := normalizeMCPToolName(prefix, tool.Name)
	if name == "" {
		return nil, fmt.Errorf("tools: mcp tool name is required")
	}
	description := tool.Description
	if description == "" {
		description = "Calls the " + tool.Name + " MCP tool."
	}
	parameters := tool.InputSchema
	if len(parameters) == 0 {
		parameters = json.RawMessage(`{"type":"object","properties":{}}`)
	}
	return &MCPTool{
		server:      prefix,
		name:        name,
		description: description,
		parameters:  parameters,
		mcpName:     tool.Name,
		client:      client,
	}, nil
}

func (t *MCPTool) Definition() Definition {
	return Definition{
		Name:        t.name,
		Description: t.description,
		Parameters:  t.parameters,
	}
}

func (t *MCPTool) Run(ctx context.Context, args json.RawMessage) (string, error) {
	var err error
	if len(args) == 0 {
		args = json.RawMessage(`{}`)
	}
	ctx, span := telem.Trace(ctx, spanMCPCall,
		telem.String("mcp.server", t.server),
		telem.String("mcp.tool_name", t.mcpName),
		jsonArgsSize(args),
	)
	defer span.End(&err)
	result, err := t.client.CallTool(ctx, t.mcpName, args)
	if err != nil {
		err = fmt.Errorf("mcp call: %w", err)
		return Fail(err.Error())
	}
	span.AddFields(mcpFields(t.server, t.mcpName, result)...)
	return OK(result)
}

func normalizeMCPToolName(prefix, name string) string {
	raw := strings.ToLower(strings.TrimSpace(prefix + "_" + name))
	normalized := nonToolNameChar.ReplaceAllString(raw, "_")
	return strings.Trim(normalized, "_")
}
