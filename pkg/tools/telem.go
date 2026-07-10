package tools

import "github.com/iamhectordev/hector/pkg/telem"

const spanRegistryRun = "tool.registry.run"

func registryFields(name string, found bool) []telem.Field {
	return []telem.Field{
		telem.String("tool.name", name),
		telem.Bool("tool.found", found),
	}
}
