package tools

import (
	"encoding/json"

	"github.com/iamhectordev/hector/internal/mcp"
	"github.com/iamhectordev/hector/pkg/telem"
)

const (
	spanRegistryRun  = "tool.registry.run"
	spanMemoryRecall = "memory.recall"
	spanMCPCall      = "github.mcp.call"
)

func registryFields(name string, found bool) []telem.Field {
	return []telem.Field{
		telem.String("tool.name", name),
		telem.Bool("tool.found", found),
	}
}

func recallFields(query string, resultCount int) []telem.Field {
	return []telem.Field{
		telem.Int("memory.query_length", len(query)),
		telem.Int("memory.result_count", resultCount),
	}
}

func mcpFields(server, toolName string, result mcp.ToolResult) []telem.Field {
	fields := []telem.Field{
		telem.String("mcp.server", server),
		telem.String("mcp.tool_name", toolName),
		telem.Bool("mcp.is_error", result.IsError),
		telem.Int("mcp.content_count", len(result.Content)),
	}
	return fields
}

func jsonArgsSize(args json.RawMessage) telem.Field {
	return telem.Int("tool.args_bytes", len(args))
}
