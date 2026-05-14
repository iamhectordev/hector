package comms

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/iamhectordev/hector/modules/tools"
	"github.com/iamhectordev/hector/pkg/session"
)

// ReplyHandler sends a text reply to a specific surface scheme.
type ReplyHandler interface {
	Scheme() string
	Reply(ctx context.Context, uri *url.URL, text string) error
}

// ReplyRouter implements tools.Tool, routing reply calls to the handler
// whose scheme matches the source URI in the session context.
type ReplyRouter struct {
	handlers map[string]ReplyHandler
}

var _ tools.Tool = (*ReplyRouter)(nil)

func NewReplyRouter(handlers ...ReplyHandler) *ReplyRouter {
	r := &ReplyRouter{handlers: make(map[string]ReplyHandler, len(handlers))}
	for _, h := range handlers {
		r.handlers[h.Scheme()] = h
	}
	return r
}

func (r *ReplyRouter) Register(h ReplyHandler) {
	r.handlers[h.Scheme()] = h
}

func (r *ReplyRouter) Definition() tools.Definition {
	params, _ := json.Marshal(map[string]any{
		"type": "object",
		"properties": map[string]any{
			"text": map[string]any{
				"type":        "string",
				"description": "The reply text to send to the user.",
			},
		},
		"required": []string{"text"},
	})
	return tools.Definition{
		Name:        "reply",
		Description: "Send a text reply back to the user on the surface they messaged from.",
		Parameters:  params,
	}
}

func (r *ReplyRouter) Run(ctx context.Context, args json.RawMessage) (string, error) {
	var input struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(args, &input); err != nil {
		return "", fmt.Errorf("comms: reply: invalid args: %w", err)
	}

	sess, ok := session.From(ctx)
	if !ok {
		return "", fmt.Errorf("comms: reply: no session in context")
	}

	uri, err := session.ParseSourceURI(sess.SourceURI)
	if err != nil {
		return "", fmt.Errorf("comms: reply: %w", err)
	}

	h, ok := r.handlers[uri.Scheme]
	if !ok {
		return "", fmt.Errorf("comms: reply: no handler for scheme %q", uri.Scheme)
	}

	if err := h.Reply(ctx, uri, input.Text); err != nil {
		return "", err
	}
	return "sent", nil
}
