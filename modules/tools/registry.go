package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/iamhectordev/hector/pkg/llm/schema"
)

// Registry owns tool registration, lookup, and synchronous execution.
type Registry struct {
	tools map[string]Tool
}

func NewRegistry(tools ...Tool) (*Registry, error) {
	r := &Registry{tools: make(map[string]Tool, len(tools))}
	for _, tool := range tools {
		if err := r.Register(tool); err != nil {
			return nil, err
		}
	}
	return r, nil
}

func (r *Registry) Register(tool Tool) error {
	if tool == nil {
		return nil
	}
	if r.tools == nil {
		r.tools = make(map[string]Tool)
	}
	def := tool.Definition()
	if err := validateDefinition(def); err != nil {
		return err
	}
	if _, exists := r.tools[def.Name]; exists {
		return fmt.Errorf("tools: duplicate tool name %q", def.Name)
	}

	r.tools[def.Name] = tool
	return nil
}

func (r *Registry) Definitions() []schema.ToolDefinition {
	defs := make([]schema.ToolDefinition, 0, len(r.tools))
	for _, tool := range r.tools {
		def := tool.Definition()
		defs = append(defs, schema.ToolDefinition{
			Name:        def.Name,
			Description: def.Description,
			Parameters:  json.RawMessage(def.Parameters),
		})
	}
	sort.Slice(defs, func(i, j int) bool {
		return defs[i].Name < defs[j].Name
	})
	return defs
}

func (r *Registry) Run(ctx context.Context, name string, args json.RawMessage) (string, error) {
	tool, ok := r.tools[name]
	if !ok {
		return "", fmt.Errorf("unknown tool %q", name)
	}
	return tool.Run(ctx, args)
}

func validateDefinition(def Definition) error {
	if def.Name == "" {
		return fmt.Errorf("tools: cannot register tool with empty name")
	}
	if def.Description == "" {
		return fmt.Errorf("tools: tool %q has empty description", def.Name)
	}
	if def.Parameters == nil {
		return fmt.Errorf("tools: tool %q has nil parameters", def.Name)
	}
	return nil
}
