package slack

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	slackgo "github.com/slack-go/slack"

	"github.com/iamhectordev/hector/pkg/session"
)

type slackReplier interface {
	PostMessageContext(ctx context.Context, channelID string, options ...slackgo.MsgOption) (string, string, error)
}

// ReplyHandler implements comms.ReplyHandler for the Slack surface.
type ReplyHandler struct {
	replier slackReplier
}

// PostMessageContext forwards to the API client, satisfying slackReplier.
// The module is used as a lazy proxy so ReplyHandler can be constructed before Init.
func (m *Module) PostMessageContext(ctx context.Context, channelID string, options ...slackgo.MsgOption) (string, string, error) {
	return m.api.PostMessageContext(ctx, channelID, options...)
}

// NewReplyHandler returns a ReplyHandler backed by this module.
// Safe to call before Init — the module proxies to m.api at call time.
func (m *Module) NewReplyHandler() *ReplyHandler { return &ReplyHandler{replier: m} }

func (h *ReplyHandler) Scheme() string { return "slack" }

func (h *ReplyHandler) Reply(ctx context.Context, uri *url.URL, text string) error {
	channelID, threadTS, err := ParseOriginURI(uri)
	if err != nil {
		return err
	}
	opts := []slackgo.MsgOption{slackgo.MsgOptionText(text, false)}
	if threadTS != "" {
		opts = append(opts, slackgo.MsgOptionTS(threadTS))
	}
	_, _, err = h.replier.PostMessageContext(ctx, channelID, opts...)
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
