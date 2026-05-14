package tui

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"

	"github.com/iamhectordev/hector/pkg/session"
)

// ReplyHandler implements comms.ReplyHandler for the TUI surface.
type ReplyHandler struct{ out io.Writer }

// NewReplyHandler returns a handler that writes replies to out, defaulting to stdout.
func NewReplyHandler(out io.Writer) *ReplyHandler {
	if out == nil {
		out = os.Stdout
	}
	return &ReplyHandler{out: out}
}

// NewOriginURI returns the fixed origin URI for the TUI surface.
func NewOriginURI() string { return session.NewSourceURI("tui", "stdout", "") }

func (h *ReplyHandler) Scheme() string { return "tui" }

func (h *ReplyHandler) Reply(_ context.Context, _ *url.URL, text string) error {
	_, err := fmt.Fprintln(h.out, text)
	return err
}
