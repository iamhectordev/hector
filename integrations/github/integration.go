package github

import (
	"context"
	"fmt"
	"io"

	"github.com/iamhectordev/hector/pkg/mcp"
	"github.com/iamhectordev/hector/pkg/tools"
	"github.com/iamhectordev/hector/pkg/telem"
)

// Integration implements integrations.Integration, integrations.ToolProvider, io.Closer.
type Integration struct {
	tools    []tools.Tool
	mcpClose io.Closer
}

// New builds the GitHub integration: validates config, creates token provider
// and REST client, builds native tools, and optionally starts the MCP client
// to discover additional tools. It performs no registry writes.
func New(ctx context.Context, cfg Config) (*Integration, error) {
	if err := validate.Struct(cfg); err != nil {
		return nil, fmt.Errorf("github: invalid config: %w", err)
	}
	tokens, err := NewTokenProvider(cfg)
	if err != nil {
		return nil, err
	}
	client, err := NewClientWithTokenProvider(ClientConfig{APIURL: cfg.APIURL}, tokens)
	if err != nil {
		return nil, err
	}
	toolList, err := newTools(client)
	if err != nil {
		return nil, err
	}
	integration := &Integration{tools: toolList}
	if cfg.MCP.Configured() {
		mcpClient, mcpTools, err := startMCP(ctx, cfg, tokens)
		if err != nil {
			return nil, err
		}
		integration.tools = append(integration.tools, mcpTools...)
		integration.mcpClose = mcpClient
	}
	return integration, nil
}

func (i *Integration) Name() string { return "github" }

func (i *Integration) Tools() []tools.Tool { return i.tools }

func (i *Integration) Close() error {
	if i.mcpClose != nil {
		return i.mcpClose.Close()
	}
	return nil
}

func startMCP(ctx context.Context, cfg Config, tokens TokenProvider) (*mcp.Client, []tools.Tool, error) {
	token, err := tokens.Token(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("github: get token for mcp: %w", err)
	}
	mcpClient, err := mcp.NewClient(mcpClientConfig(cfg.MCP, token))
	if err != nil {
		return nil, nil, err
	}
	if err := mcpClient.Start(ctx); err != nil {
		return nil, nil, err
	}
	discovered, err := mcpClient.ListTools(ctx)
	if err != nil {
		_ = mcpClient.Close()
		return nil, nil, err
	}
	result := make([]tools.Tool, 0, len(discovered))
	for _, discoveredTool := range discovered {
		t, err := tools.NewMCPTool("github", mcpClient, discoveredTool)
		if err != nil {
			_ = mcpClient.Close()
			return nil, nil, err
		}
		result = append(result, t)
	}
	telem.Logger(ctx).InfoContext(ctx, "github mcp tools registered", telem.Int("count", len(discovered)))
	return mcpClient, result, nil
}
