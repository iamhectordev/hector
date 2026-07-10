package slack

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/slack-go/slack/slackevents"
)

func ParseReceivedEvent(now time.Time, e *slackevents.MessageEvent) (MessageReceivedData, bool, error) {
	if e == nil {
		return MessageReceivedData{}, false, fmt.Errorf("slack: message event cannot be nil")
	}

	sentAt, err := parseSlackTimestamp(e.TimeStamp)
	if err != nil {
		return MessageReceivedData{}, false, fmt.Errorf("slack: parse message timestamp: %w", err)
	}

	threadTS := e.ThreadTimeStamp
	if threadTS == "" {
		threadTS = e.TimeStamp
	}
	data := MessageReceivedData{
		TS:         e.TimeStamp,
		Channel:    Channel{ID: e.Channel},
		ThreadTS:   threadTS,
		Sender:     Sender{ID: e.User},
		Text:       e.Text,
		SentAt:     sentAt,
		ReceivedAt: now,
	}
	data.Forwards = detectForwards(e, now)
	return data, true, nil
}

func ParseChangedEvent(now time.Time, e *slackevents.MessageEvent) (MessageReceivedData, bool, error) {
	if e == nil {
		return MessageReceivedData{}, false, fmt.Errorf("slack: message event cannot be nil")
	}

	if e.Message == nil {
		return MessageReceivedData{}, false, fmt.Errorf("slack: message_changed has no inner message")
	}

	sentAt, err := parseSlackTimestamp(e.Message.Timestamp)
	if err != nil {
		return MessageReceivedData{}, false, fmt.Errorf("slack: parse message_changed timestamp: %w", err)
	}

	threadTS := e.Message.ThreadTimestamp
	if threadTS == "" {
		threadTS = e.Message.Timestamp
	}

	data := MessageReceivedData{
		TS:         e.Message.Timestamp,
		Channel:    Channel{ID: e.Channel},
		ThreadTS:   threadTS,
		Sender:     Sender{ID: e.Message.User},
		Text:       e.Message.Text,
		SentAt:     sentAt,
		ReceivedAt: now,
	}
	data.Forwards = detectForwards(e, now)
	return data, true, nil
}

func detectForwards(e *slackevents.MessageEvent, now time.Time) []MessageReceivedData {
	if e.Message == nil || len(e.Message.Attachments) == 0 {
		return nil
	}
	var forwards []MessageReceivedData
	for _, a := range e.Message.Attachments {
		if a.AuthorID == "" {
			continue
		}
		ts := a.Ts.String()
		if ts == "" {
			continue
		}
		channelID, _ := parseForwardChannelID(a.FromURL)
		sentAt, _ := parseSlackTimestamp(ts)
		forwards = append(forwards, MessageReceivedData{
			TS:         ts,
			Channel:    Channel{ID: channelID},
			Sender:     Sender{ID: a.AuthorID, Name: a.AuthorName},
			Text:       a.Text,
			SentAt:     sentAt,
			ReceivedAt: now,
		})
	}
	return forwards
}

func parseForwardChannelID(fromURL string) (string, bool) {
	u, err := url.Parse(fromURL)
	if err != nil {
		return "", false
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) >= 3 && parts[0] == "archives" {
		return parts[1], true
	}
	return "", false
}

func parseSlackTimestamp(ts string) (time.Time, error) {
	if ts == "" {
		return time.Time{}, fmt.Errorf("empty timestamp")
	}

	secondsText, fractionText, ok := strings.Cut(ts, ".")
	seconds, err := strconv.ParseInt(secondsText, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("seconds %q: %w", secondsText, err)
	}
	if !ok {
		return time.Unix(seconds, 0).UTC(), nil
	}
	if fractionText == "" {
		return time.Time{}, fmt.Errorf("empty fractional seconds")
	}
	if len(fractionText) > 9 {
		return time.Time{}, fmt.Errorf("fractional seconds %q exceed nanosecond precision", fractionText)
	}

	nanosText := fractionText + strings.Repeat("0", 9-len(fractionText))
	nanos, err := strconv.ParseInt(nanosText, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("fractional seconds %q: %w", fractionText, err)
	}

	return time.Unix(seconds, nanos).UTC(), nil
}
