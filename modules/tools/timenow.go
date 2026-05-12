package tools

import (
	"context"
	"encoding/json"
	"time"
)

type TimeNow struct{}

func (TimeNow) Definition() Definition {
	return Definition{
		Name:        "time.now",
		Description: "Returns the current UTC time as a formatted string. Does not accept any arguments. Does not support timezones — always UTC.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`),
	}
}

func (TimeNow) Run(context.Context, json.RawMessage) (string, error) {
	now := time.Now().UTC()
	return now.Format("Monday, 2006-01-02 15:04:05 UTC"), nil
}
