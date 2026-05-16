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

func envelopeStatus(t *testing.T, output string) string {
	t.Helper()
	var env map[string]any
	require.NoError(t, json.Unmarshal([]byte(output), &env))
	status, ok := env["status"].(string)
	require.True(t, ok, "envelope missing status field")
	return status
}

func TestReplyRouter_RoutesToCorrectHandler(t *testing.T) {
	slack := &fakeHandler{scheme: "slack"}
	tui := &fakeHandler{scheme: "tui"}
	router, err := comms.NewReplyRouter(slack, tui)
	require.NoError(t, err)

	ctx := session.With(context.Background(), session.Session{
		SourceURI: session.NewSourceURI("slack", "channels", "D123"),
	})
	args, _ := json.Marshal(map[string]string{"text": "hello"})
	out, err := router.Run(ctx, args)

	require.NoError(t, err)
	require.Equal(t, "ok", envelopeStatus(t, out))
	require.Equal(t, []string{"hello"}, slack.replies)
	require.Empty(t, tui.replies)
}

func TestReplyRouter_NoSession_EnvelopesError(t *testing.T) {
	router, err := comms.NewReplyRouter()
	require.NoError(t, err)

	args, _ := json.Marshal(map[string]string{"text": "hello"})
	out, err := router.Run(context.Background(), args)

	require.NoError(t, err)
	require.Equal(t, "error", envelopeStatus(t, out))
}

func TestReplyRouter_UnknownScheme_EnvelopesError(t *testing.T) {
	router, err := comms.NewReplyRouter()
	require.NoError(t, err)

	ctx := session.With(context.Background(), session.Session{
		SourceURI: session.NewSourceURI("email", "inbox", "123"),
	})
	args, _ := json.Marshal(map[string]string{"text": "hello"})
	out, err := router.Run(ctx, args)

	require.NoError(t, err)
	require.Equal(t, "error", envelopeStatus(t, out))
}
