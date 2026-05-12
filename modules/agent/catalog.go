package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/iamhectordev/hector/modules/tools"
	"github.com/iamhectordev/hector/pkg/llm/schema"
)

// Catalog holds the tools available to the agent loop.
type Catalog struct {
	tools       map[string]tools.Tool
	definitions []schema.ToolDefinition
}

func NewCatalog(ts ...tools.Tool) *Catalog {
	c := &Catalog{tools: make(map[string]tools.Tool, len(ts))}
	for _, t := range ts {
		def := t.Definition()
		c.tools[def.Name] = t
		c.definitions = append(c.definitions, schema.ToolDefinition{
			Name:        def.Name,
			Description: def.Description,
			Parameters:  json.RawMessage(def.Parameters),
		})
	}
	return c
}

// Definitions returns the tool definitions to include in a completion request.
func (c *Catalog) Definitions() []schema.ToolDefinition {
	return c.definitions
}

// Execute runs the named tool with the given arguments.
func (c *Catalog) Execute(ctx context.Context, name string, args json.RawMessage) (string, error) {
	t, ok := c.tools[name]
	if !ok {
		return "", fmt.Errorf("unknown tool %q", name)
	}
	return t.Run(ctx, args)
}
