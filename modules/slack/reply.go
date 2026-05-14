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
	origin, err := ParseOriginURI(uri)
	if err != nil {
		return err
	}
	opts := []slackgo.MsgOption{slackgo.MsgOptionText(text, false)}
	if origin.ThreadTS != "" {
		opts = append(opts, slackgo.MsgOptionTS(origin.ThreadTS))
	}
	_, _, err = h.replier.PostMessageContext(ctx, origin.ChannelID, opts...)
	return err
}

// NewOriginURI builds a slack origin URI: slack://channelID[/threadTS].
func NewOriginURI(channelID, threadTS string) string {
	return session.NewSourceURI("slack", channelID, threadTS)
}

// ParseOriginURI parses a slack origin URI into an Origin.
func ParseOriginURI(u *url.URL) (Origin, error) {
	if u.Host == "" {
		return Origin{}, fmt.Errorf("slack: origin URI missing channel ID")
	}
	return Origin{
		ChannelID: u.Host,
		ThreadTS:  strings.TrimPrefix(u.Path, "/"),
	}, nil
}
