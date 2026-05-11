package tools

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
)

type TimeNow struct{}

func (TimeNow) Definition() Definition {
	return Definition{
		Name:        "time.now",
		Description: "Returns the current UTC time with calendar context.",
		InputSchema: &jsonschema.Schema{
			Type:                 "object",
			Properties:           map[string]*jsonschema.Schema{},
			AdditionalProperties: &jsonschema.Schema{Not: &jsonschema.Schema{}},
		},
	}
}

func (TimeNow) Run(context.Context, json.RawMessage) (string, error) {
	now := time.Now().UTC()
	return now.Format("Monday, 2006-01-02 15:04:05 UTC"), nil
}
