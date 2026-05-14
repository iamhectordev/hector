package slack

import (
	"context"
	"net/url"
	"strings"

	slackgo "github.com/slack-go/slack"
)

// ReplyHandler implements comms.ReplyHandler for the Slack surface.
type ReplyHandler struct{ m *Module }

// NewReplyHandler returns a handler backed by the module's API client.
// Must be called after Init.
func (m *Module) NewReplyHandler() *ReplyHandler { return &ReplyHandler{m: m} }

func (h *ReplyHandler) Scheme() string { return "slack" }

func (h *ReplyHandler) Reply(ctx context.Context, uri *url.URL, text string) error {
	channelID := strings.TrimPrefix(uri.Path, "/")
	_, _, err := h.m.api.PostMessageContext(ctx, channelID, slackgo.MsgOptionText(text, false))
	return err
}
