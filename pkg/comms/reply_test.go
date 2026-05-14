package comms_test

import (
	"context"
	"encoding/json"
	"net/url"
	"testing"

	"github.com/iamhectordev/hector/pkg/comms"
	"github.com/iamhectordev/hector/pkg/session"
	"github.com/stretchr/testify/require"
)

type fakeHandler struct {
	scheme  string
	replies []string
}

func (f *fakeHandler) Scheme() string { return f.scheme }
func (f *fakeHandler) Reply(_ context.Context, _ *url.URL, text string) error {
	f.replies = append(f.replies, text)
	return nil
}

func TestReplyRouter_RoutesToCorrectHandler(t *testing.T) {
	slack := &fakeHandler{scheme: "slack"}
	tui := &fakeHandler{scheme: "tui"}
	router := comms.NewReplyRouter(slack, tui)

	ctx := session.With(context.Background(), session.Session{
		SourceURI: session.NewSourceURI("slack", "channels", "D123"),
	})
	args, _ := json.Marshal(map[string]string{"text": "hello"})
	_, err := router.Run(ctx, args)

	require.NoError(t, err)
	require.Equal(t, []string{"hello"}, slack.replies)
	require.Empty(t, tui.replies)
}

func TestReplyRouter_NoSession_Errors(t *testing.T) {
	router := comms.NewReplyRouter()
	args, _ := json.Marshal(map[string]string{"text": "hello"})
	_, err := router.Run(context.Background(), args)
	require.Error(t, err)
}

func TestReplyRouter_UnknownScheme_Errors(t *testing.T) {
	router := comms.NewReplyRouter()
	ctx := session.With(context.Background(), session.Session{
		SourceURI: session.NewSourceURI("email", "inbox", "123"),
	})
	args, _ := json.Marshal(map[string]string{"text": "hello"})
	_, err := router.Run(ctx, args)
	require.Error(t, err)
}
