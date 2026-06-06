package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/iamhectordev/hector/pkg/memory"
	"github.com/iamhectordev/hector/pkg/telem"
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
	var err error
	var input memRecallInput
	if err = json.Unmarshal(args, &input); err != nil {
		err = fmt.Errorf("invalid args: %w", err)
		return Fail(fmt.Sprintf("invalid args: %s", err))
	}
	query := strings.TrimSpace(input.Query)
	if query == "" {
		err = fmt.Errorf("query is required")
		return Fail("query is required")
	}
	ctx, span := telem.Trace(ctx, spanMemoryRecall, recallFields(query, 0)...)
	defer span.End(&err)

	results, err := t.store.Search(ctx, query, 3)
	if err != nil {
		return Fail(err.Error())
	}
	span.AddFields(recallFields(query, len(results))...)
	if len(results) == 0 {
		return OK("no relevant facts found")
	}

	var lines []string
	for _, obj := range results {
		line := obj.Content
		if !obj.CreatedAt.IsZero() {
			line = fmt.Sprintf("[%s] %s", obj.CreatedAt.UTC().Format("2006-01-02 15:04 UTC"), obj.Content)
		}
		lines = append(lines, line)
	}
	return OK(strings.Join(lines, "\n"))
}
