package session_test

import (
	"testing"

	"github.com/iamhectordev/hector/pkg/session"
	"github.com/stretchr/testify/require"
)

func TestNewSourceURI(t *testing.T) {
	got := session.NewSourceURI("slack", "channels", "D1234ABC")
	require.Equal(t, "slack://channels/D1234ABC", got)
}

func TestParseSourceURI(t *testing.T) {
	u, err := session.ParseSourceURI("slack://channels/D1234ABC")
	require.NoError(t, err)
	require.Equal(t, "slack", u.Scheme)
	require.Equal(t, "channels", u.Host)
	require.Equal(t, "/D1234ABC", u.Path)
}

func TestParseSourceURI_NoScheme_Errors(t *testing.T) {
	_, err := session.ParseSourceURI("channels/D1234ABC")
	require.Error(t, err)
}
