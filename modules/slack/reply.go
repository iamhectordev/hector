package slack

import (
	"context"
	"net/url"
	"strings"

	slackgo "github.com/slack-go/slack"
)

type slackReplier interface {
	PostMessageContext(ctx context.Context, channelID string, options ...slackgo.MsgOption) (string, string, error)
}

// ReplyHandler implements comms.ReplyHandler for the Slack surface.
type ReplyHandler struct {
	replier slackReplier
}

// NewReplyHandler returns a handler backed by the module's API client.
// Must be called after Init.
func (m *Module) NewReplyHandler() *ReplyHandler { return &ReplyHandler{replier: m.api} }

func (h *ReplyHandler) Scheme() string { return "slack" }

func (h *ReplyHandler) Reply(ctx context.Context, uri *url.URL, text string) error {
	channelID := strings.TrimPrefix(uri.Path, "/")
	_, _, err := h.replier.PostMessageContext(ctx, channelID, slackgo.MsgOptionText(text, false))
	return err
}
