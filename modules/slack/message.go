package slack

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/slack-go/slack/slackevents"
)

func messageReceivedData(now time.Time, e *slackevents.MessageEvent) (MessageReceivedData, bool, error) {
	if e == nil {
		return MessageReceivedData{}, false, fmt.Errorf("slack: message event cannot be nil")
	}
	if !e.IsIM() {
		return MessageReceivedData{}, false, nil
	}

	sentAt, err := parseSlackTimestamp(e.TimeStamp)
	if err != nil {
		return MessageReceivedData{}, false, fmt.Errorf("slack: parse message timestamp: %w", err)
	}

	threadTS := e.ThreadTimeStamp
	if threadTS == "" {
		threadTS = e.TimeStamp
	}
	return MessageReceivedData{
		Origin:     Origin{ChannelID: e.Channel, ThreadTS: threadTS},
		SenderID:   e.User,
		Text:       e.Text,
		SentAt:     sentAt,
		ReceivedAt: now,
	}, true, nil
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
