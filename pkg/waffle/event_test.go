package waffle_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iamhectordev/hector/pkg/waffle"
)

func TestDefinitionCreatesEvent(t *testing.T) {
	type messageReceived struct {
		Text string
	}

	before := time.Now().UTC()
	def, err := waffle.Define[messageReceived]("gateway.slack_message_received", 1)
	require.NoError(t, err)
	event := def.New(messageReceived{Text: "hello"})
	after := time.Now().UTC()

	require.True(t, strings.HasPrefix(event.ID(), "evt_"))
	require.False(t, event.OccurredAt().Before(before))
	require.False(t, event.OccurredAt().After(after))

	tests := []struct {
		name string
		want any
		got  any
	}{
		{"type", "gateway.slack_message_received", event.Type()},
		{"schema_version", 1, event.SchemaVersion()},
		{"data.text", "hello", event.Data().Text},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, tt.got)
		})
	}
}
