package slack

import (
	"testing"
	"time"

	"github.com/slack-go/slack/slackevents"
	"github.com/stretchr/testify/require"
)

func TestMessageReceivedData(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 8, 20, 0, 0, 0, time.UTC)
	sentAt := time.Unix(1355517523, 5000).UTC()

	tests := []struct {
		name string
		in   *slackevents.MessageEvent
		want MessageReceivedData
		ok   bool
	}{
		{
			name: "dm maps fields",
			in: &slackevents.MessageEvent{
				Channel:     "D024BE91L",
				User:        "U2147483697",
				Text:        "hello",
				TimeStamp:   "1355517523.000005",
				ChannelType: slackevents.ChannelTypeIM,
			},
			want: MessageReceivedData{
				Origin:     Origin{ChannelID: "D024BE91L"},
				SenderID:   "U2147483697",
				Text:       "hello",
				SentAt:     sentAt,
				ReceivedAt: now,
			},
			ok: true,
		},
		{
			name: "channel drops",
			in: &slackevents.MessageEvent{
				Channel:     "C024BE91L",
				User:        "U2147483697",
				Text:        "hello",
				TimeStamp:   "1355517523.000005",
				ChannelType: slackevents.ChannelTypeChannel,
			},
			ok: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok, err := messageReceivedData(now, tt.in)

			require.NoError(t, err)
			require.Equal(t, tt.ok, ok)
			if tt.ok {
				require.Equal(t, tt.want, got)
			}
		})
	}
}

func TestMessageReceivedData_BadTimestampReturnsError(t *testing.T) {
	t.Parallel()

	_, ok, err := messageReceivedData(time.Now(), &slackevents.MessageEvent{
		TimeStamp:   "not-a-timestamp",
		ChannelType: slackevents.ChannelTypeIM,
	})

	require.Error(t, err)
	require.False(t, ok)
}
