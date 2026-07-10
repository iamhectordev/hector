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

type Replier interface {
	PostMessageContext(ctx context.Context, channelID string, options ...slackgo.MsgOption) (string, string, error)
}

type ReplyHandler struct {
	replier func() Replier
}

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

func NewOriginURI(channelID, threadTS string) string {
	return session.NewSourceURI("slack", channelID, threadTS)
}

func ParseOriginURI(u *url.URL) (string, string, error) {
	if u.Host == "" {
		return "", "", fmt.Errorf("slack: origin URI missing channel ID")
	}
	return u.Host, strings.TrimPrefix(u.Path, "/"), nil
}
