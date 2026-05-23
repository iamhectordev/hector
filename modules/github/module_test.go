package github_test

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	hectorgithub "github.com/iamhectordev/hector/modules/github"
	"github.com/stretchr/testify/require"
)

func TestModuleInitLogsVerificationIssueTitle(t *testing.T) {
	t.Parallel()

	privateKeyPath := writeTestPrivateKey(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/app/installations/456/access_tokens":
			w.WriteHeader(http.StatusCreated)
			_, err := fmt.Fprint(w, `{"token":"installation-token","expires_at":"2026-05-22T12:00:00Z"}`)
			require.NoError(t, err)
		case "/repos/iamhectordev/hector/issues/1":
			_, err := fmt.Fprint(w, `{"id":99,"number":1,"title":"Replace me before real use","state":"open","html_url":"https://github.com/replace-owner/replace-repo/issues/1","user":{"login":"alice"}}`)
			require.NoError(t, err)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	module, err := hectorgithub.NewModule(hectorgithub.Config{
		AppID:          123,
		InstallationID: 456,
		PrivateKeyPath: privateKeyPath,
		APIURL:         server.URL,
	}, hectorgithub.WithLogger(logger))
	require.NoError(t, err)

	require.NoError(t, module.Init(t.Context()))
	require.Contains(t, logs.String(), "github integration verified")
	require.Contains(t, logs.String(), `issue_title="Replace me before real use"`)
}

func TestConfigConfigured(t *testing.T) {
	t.Parallel()

	require.False(t, hectorgithub.Config{}.Configured())
	require.True(t, hectorgithub.Config{AppID: 123}.Configured())
}
