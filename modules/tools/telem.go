package tools

import (
	"github.com/iamhectordev/hector/pkg/telem"
)

const spanMemoryRecall = "memory.recall"

func recallFields(query string, resultCount int) []telem.Field {
	return []telem.Field{
		telem.Int("memory.query_length", len(query)),
		telem.Int("memory.result_count", resultCount),
	}
}
