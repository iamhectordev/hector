package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/iamhectordev/hector/pkg/memory"
)

type memorySearcher interface {
	Search(ctx context.Context, query string, limit int) ([]memory.Object, error)
}

type memRecallInput struct {
	Query string `json:"query" jsonschema:"what to look up in org memory"`
}

// MemRecallTool returns relevant facts from org memory matching a query.
type MemRecallTool struct {
	store  memorySearcher
	schema json.RawMessage
}

// NewMemRecall creates a mem_recall tool backed by the given searcher.
func NewMemRecall(store memorySearcher) (*MemRecallTool, error) {
	schema, err := SchemaFor[memRecallInput]()
	if err != nil {
		return nil, err
	}
	return &MemRecallTool{store: store, schema: schema}, nil
}

func (t *MemRecallTool) Definition() Definition {
	return Definition{
		Name:        "mem_recall",
		Description: "Returns relevant facts from org memory matching the query. Use when asked about team context, services, people, or past decisions.",
		Parameters:  t.schema,
	}
}

func (t *MemRecallTool) Run(ctx context.Context, args json.RawMessage) (string, error) {
	var input memRecallInput
	if err := json.Unmarshal(args, &input); err != nil {
		return Fail(fmt.Sprintf("invalid args: %s", err))
	}
	if strings.TrimSpace(input.Query) == "" {
		return Fail("query is required")
	}

	results, err := t.store.Search(ctx, input.Query, 3)
	if err != nil {
		return Fail(err.Error())
	}
	if len(results) == 0 {
		return OK("no relevant facts found")
	}

	var lines []string
	for _, obj := range results {
		lines = append(lines, obj.Content)
	}
	return OK(strings.Join(lines, "\n"))
}
