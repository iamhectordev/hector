package slack

import (
	"encoding/json"
	"testing"
	"time"

	slackgo "github.com/slack-go/slack"
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
				Channel:    Channel{ID: "D024BE91L"},
				ThreadTS:   "1355517523.000005",
				TS:         "1355517523.000005",
				Sender:     Sender{ID: "U2147483697"},
				Text:       "hello",
				SentAt:     sentAt,
				ReceivedAt: now,
			},
			ok: true,
		},
		{
			name: "channel passes through",
			in: &slackevents.MessageEvent{
				Channel:     "C024BE91L",
				User:        "U2147483697",
				Text:        "hello",
				TimeStamp:   "1355517523.000005",
				ChannelType: slackevents.ChannelTypeChannel,
			},
			want: MessageReceivedData{
				Channel:    Channel{ID: "C024BE91L"},
				ThreadTS:   "1355517523.000005",
				TS:         "1355517523.000005",
				Sender:     Sender{ID: "U2147483697"},
				Text:       "hello",
				SentAt:     sentAt,
				ReceivedAt: now,
			},
			ok: true,
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

func TestMessageReceivedData_ForwardsFromAttachments(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 8, 20, 0, 0, 0, time.UTC)
	fwdSentAt := time.Unix(1355517522, 1000).UTC()

	got, ok, err := messageReceivedData(now, &slackevents.MessageEvent{
		Channel:     "D024BE91L",
		User:        "U2147483697",
		Text:        "what do you think?",
		TimeStamp:   "1355517523.000005",
		ChannelType: slackevents.ChannelTypeIM,
		Message: &slackgo.Msg{
			Attachments: []slackgo.Attachment{
				{
					AuthorID:   "U999",
					AuthorName: "Bob",
					Ts:         json.Number("1355517522.000001"),
					Text:       "Original message",
					FromURL:    "https://hector.slack.com/archives/C999/p1355517522000001",
				},
			},
		},
	})

	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, 1, len(got.Forwards))

	fwd := got.Forwards[0]
	require.Equal(t, "1355517522.000001", fwd.TS)
	require.Equal(t, "U999", fwd.Sender.ID)
	require.Equal(t, "Bob", fwd.Sender.Name)
	require.Equal(t, "Original message", fwd.Text)
	require.Equal(t, "C999", fwd.Channel.ID)
	require.Equal(t, fwdSentAt, fwd.SentAt)
	require.Equal(t, now, fwd.ReceivedAt)
}

func TestMessageReceivedData_NoForwardsWhenNoMessage(t *testing.T) {
	t.Parallel()

	got, ok, err := messageReceivedData(time.Now(), &slackevents.MessageEvent{
		Channel:     "D024BE91L",
		User:        "U2147483697",
		Text:        "hello",
		TimeStamp:   "1355517523.000005",
		ChannelType: slackevents.ChannelTypeIM,
	})

	require.NoError(t, err)
	require.True(t, ok)
	require.Empty(t, got.Forwards)
}

func TestMessageReceivedData_NoForwardsWhenAttachmentMissingAuthorID(t *testing.T) {
	t.Parallel()

	got, ok, err := messageReceivedData(time.Now(), &slackevents.MessageEvent{
		Channel:     "D024BE91L",
		User:        "U2147483697",
		Text:        "check this out",
		TimeStamp:   "1355517523.000005",
		ChannelType: slackevents.ChannelTypeIM,
		Message: &slackgo.Msg{
			Attachments: []slackgo.Attachment{
				{
					Text: "some link preview",
					Ts:   json.Number("1355517522.000001"),
				},
			},
		},
	})

	require.NoError(t, err)
	require.True(t, ok)
	require.Empty(t, got.Forwards)
}

func TestParseForwardChannelID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		url  string
		want string
		ok   bool
	}{
		{url: "https://hector.slack.com/archives/C999/p1355517522000001", want: "C999", ok: true},
		{url: "https://hector.slack.com/archives/DAB123/p1747123400000050", want: "DAB123", ok: true},
		{url: "https://hector.slack.com/archives/G888/p1000000000000000", want: "G888", ok: true},
		{url: "", want: "", ok: false},
		{url: "not-a-url", want: "", ok: false},
		{url: "https://example.com/not-archives/C999/p123", want: "", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			t.Parallel()
			got, ok := parseForwardChannelID(tt.url)
			require.Equal(t, tt.ok, ok)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestMessageChangedData(t *testing.T) {
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
			name: "reads from inner message fields",
			in: &slackevents.MessageEvent{
				SubType:     "message_changed",
				Channel:     "D024BE91L",
				TimeStamp:   "1355517523.000005",
				ChannelType: slackevents.ChannelTypeIM,
				Message: &slackgo.Msg{
					User:      "U2147483697",
					Text:      "hello",
					Timestamp: "1355517523.000005",
				},
			},
			want: MessageReceivedData{
				Channel:    Channel{ID: "D024BE91L"},
				ThreadTS:   "1355517523.000005",
				TS:         "1355517523.000005",
				Sender:     Sender{ID: "U2147483697"},
				Text:       "hello",
				SentAt:     sentAt,
				ReceivedAt: now,
			},
			ok: true,
		},
		{
			name: "error when inner message missing",
			in: &slackevents.MessageEvent{
				SubType:     "message_changed",
				Channel:     "D024BE91L",
				TimeStamp:   "1355517523.000005",
				ChannelType: slackevents.ChannelTypeIM,
				Message:     nil,
			},
			ok: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok, err := messageChangedData(now, tt.in)
			if tt.ok {
				require.NoError(t, err)
				require.True(t, ok)
				require.Equal(t, tt.want, got)
			} else {
				require.Error(t, err)
				require.False(t, ok)
			}
		})
	}
}

func TestMessageChangedData_ForwardsFromAttachments(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 8, 20, 0, 0, 0, time.UTC)
	fwdSentAt := time.Unix(1355517522, 1000).UTC()

	got, ok, err := messageChangedData(now, &slackevents.MessageEvent{
		SubType:     "message_changed",
		Channel:     "D024BE91L",
		TimeStamp:   "1355517523.000005",
		ChannelType: slackevents.ChannelTypeIM,
		Message: &slackgo.Msg{
			User:      "U2147483697",
			Text:      "check this",
			Timestamp: "1355517523.000005",
			Attachments: []slackgo.Attachment{
				{
					AuthorID:   "U999",
					AuthorName: "Bob",
					Ts:         json.Number("1355517522.000001"),
					Text:       "Original message",
					FromURL:    "https://hector.slack.com/archives/C999/p1355517522000001",
				},
			},
		},
	})

	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, 1, len(got.Forwards))

	fwd := got.Forwards[0]
	require.Equal(t, "1355517522.000001", fwd.TS)
	require.Equal(t, "U999", fwd.Sender.ID)
	require.Equal(t, "Bob", fwd.Sender.Name)
	require.Equal(t, "Original message", fwd.Text)
	require.Equal(t, "C999", fwd.Channel.ID)
	require.Equal(t, fwdSentAt, fwd.SentAt)
	require.Equal(t, now, fwd.ReceivedAt)
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
