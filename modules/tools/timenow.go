package tools

import (
	"context"
	"time"
)

func NewTimeNow() (Tool, error) {
	return New[struct{}, string](
		"time.now",
		"Returns the current UTC time as a formatted string. Does not accept any arguments. Does not support timezones — always UTC.",
		func(_ context.Context, _ struct{}) (string, error) {
			return time.Now().UTC().Format("Monday, 2006-01-02 15:04:05 UTC"), nil
		},
	)
}
