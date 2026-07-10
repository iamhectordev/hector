package comms

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/iamhectordev/hector/pkg/session"
	"github.com/iamhectordev/hector/pkg/tools"
	"github.com/iamhectordev/hector/pkg/telem"
)

// ReplyHandler sends a text reply to a specific surface scheme.
type ReplyHandler interface {
	Scheme() string
	Reply(ctx context.Context, uri *url.URL, text string) error
}

type replyInput struct {
	Text string `json:"text" jsonschema:"The reply text to send to the user."`
}

// ReplyRouter implements tools.Tool, routing reply calls to the handler
// whose scheme matches the source URI in the session context.
type ReplyRouter struct {
	handlers map[string]ReplyHandler
	schema   json.RawMessage
}

var _ tools.Tool = (*ReplyRouter)(nil)

func NewReplyRouter(handlers ...ReplyHandler) (*ReplyRouter, error) {
	schema, err := tools.SchemaFor[replyInput]()
	if err != nil {
		return nil, fmt.Errorf("comms: reply: schema: %w", err)
	}
	r := &ReplyRouter{
		handlers: make(map[string]ReplyHandler, len(handlers)),
		schema:   schema,
	}
	for _, h := range handlers {
		r.handlers[h.Scheme()] = h
	}
	return r, nil
}

func (r *ReplyRouter) Definition() tools.Definition {
	return tools.Definition{
		Name:        "reply",
		Description: "Send a text reply back to the user on the surface they messaged from.",
		Parameters:  r.schema,
	}
}

func (r *ReplyRouter) Run(ctx context.Context, args json.RawMessage) (string, error) {
	var err error
	ctx, span := telem.Trace(ctx, spanReplyRoute)
	defer span.End(&err)

	var input replyInput
	if err = json.Unmarshal(args, &input); err != nil {
		err = fmt.Errorf("invalid args: %w", err)
		return tools.Fail(fmt.Sprintf("invalid args: %s", err))
	}

	sess, ok := session.From(ctx)
	if !ok {
		err = fmt.Errorf("no session in context")
		return tools.Fail("no session in context")
	}

	uri, err := session.ParseSourceURI(sess.SourceURI)
	if err != nil {
		err = fmt.Errorf("parse source uri: %w", err)
		return tools.Fail(err.Error())
	}
	span.AddFields(replyRouteFields(sess.SourceURI, uri)...)

	h, ok := r.handlers[uri.Scheme]
	if !ok {
		err = fmt.Errorf("no handler for scheme %q", uri.Scheme)
		return tools.Fail(fmt.Sprintf("no handler for scheme %q", uri.Scheme))
	}

	replyErr := h.Reply(ctx, uri, input.Text)
	if replyErr != nil {
		err = fmt.Errorf("reply handler: %w", replyErr)
		return tools.Fail(replyErr.Error())
	}
	return tools.OK("sent")
}
