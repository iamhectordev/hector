package slack

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	slackgo "github.com/slack-go/slack"

	"github.com/iamhectordev/hector/pkg/session"
	"github.com/iamhectordev/hector/pkg/telem"
)

// Replier can post messages to Slack channels.
type Replier interface {
	PostMessageContext(ctx context.Context, channelID string, options ...slackgo.MsgOption) (string, string, error)
}

// ReplyHandler implements comms.ReplyHandler for the Slack surface.
type ReplyHandler struct {
	replier func() Replier
}

// NewReplyHandler returns a ReplyHandler that resolves the Replier lazily via fn.
// Safe to call before the Slack client is initialised — fn is called at Reply time.
func NewReplyHandler(fn func() Replier) *ReplyHandler {
	return &ReplyHandler{replier: fn}
}

func (h *ReplyHandler) Scheme() string { return "slack" }

func (h *ReplyHandler) Reply(ctx context.Context, uri *url.URL, text string) (err error) {
	ctx, span := telem.Trace(ctx, spanReplySend, replyFields(uri)...)
	defer span.End(&err)

	channelID, threadTS, err := ParseOriginURI(uri)
	if err != nil {
		return err
	}
	opts := []slackgo.MsgOption{slackgo.MsgOptionText(text, false)}
	if threadTS != "" {
		opts = append(opts, slackgo.MsgOptionTS(threadTS))
	}
	_, _, err = h.replier().PostMessageContext(ctx, channelID, opts...)
	return err
}

// NewOriginURI builds a slack origin URI: slack://channelID[/threadTS].
func NewOriginURI(channelID, threadTS string) string {
	return session.NewSourceURI("slack", channelID, threadTS)
}

// ParseOriginURI parses a slack origin URI into channelID and threadTS.
func ParseOriginURI(u *url.URL) (string, string, error) {
	if u.Host == "" {
		return "", "", fmt.Errorf("slack: origin URI missing channel ID")
	}
	return u.Host, strings.TrimPrefix(u.Path, "/"), nil
}
