package github_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	hectorgithub "github.com/iamhectordev/hector/modules/github"
	"github.com/stretchr/testify/require"
)

type staticTokenProvider struct {
	token hectorgithub.AccessToken
}

func (p staticTokenProvider) Token(context.Context) (hectorgithub.AccessToken, error) {
	return p.token, nil
}

func TestClientUsesInjectedTokenProvider(t *testing.T) {
	t.Parallel()

	var issueAuthHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/repos/acme/widgets/issues/7", r.URL.Path)
		issueAuthHeader = r.Header.Get("Authorization")
		_, err := fmt.Fprint(w, `{"id":99,"number":7,"title":"Fix widget parser","state":"open","body":"Parser fails.","html_url":"https://github.com/acme/widgets/issues/7","user":{"login":"alice"}}`)
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	client, err := hectorgithub.NewClientWithTokenProvider(hectorgithub.ClientConfig{
		APIURL: server.URL,
	}, staticTokenProvider{token: hectorgithub.AccessToken{
		Value:     "provided-token",
		ExpiresAt: time.Now().Add(time.Hour),
	}})
	require.NoError(t, err)

	issue, err := client.GetIssue(t.Context(), hectorgithub.Repository{
		Owner: "acme",
		Name:  "widgets",
	}, 7)
	require.NoError(t, err)

	require.Equal(t, "Bearer provided-token", issueAuthHeader)
	require.Equal(t, "Fix widget parser", issue.Title)
}

func TestClientCanFetchIssueWithGitHubAppInstallationAuth(t *testing.T) {
	t.Parallel()

	privateKeyPath := writeTestPrivateKey(t)

	var tokenAuthHeader string
	var issueAuthHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/app/installations/456/access_tokens":
			require.Equal(t, http.MethodPost, r.Method)
			tokenAuthHeader = r.Header.Get("Authorization")
			require.NotEmpty(t, tokenAuthHeader)
			require.Equal(t, "application/vnd.github+json", r.Header.Get("Accept"))
			w.WriteHeader(http.StatusCreated)
			_, err := fmt.Fprint(w, `{"token":"installation-token","expires_at":"2026-05-22T12:00:00Z"}`)
			require.NoError(t, err)
		case "/repos/acme/widgets/issues/7":
			require.Equal(t, http.MethodGet, r.Method)
			issueAuthHeader = r.Header.Get("Authorization")
			require.Equal(t, "Bearer installation-token", issueAuthHeader)
			require.Equal(t, "application/vnd.github+json", r.Header.Get("Accept"))
			_, err := fmt.Fprint(w, `{"id":99,"number":7,"title":"Fix widget parser","state":"open","body":"Parser fails.","html_url":"https://github.com/acme/widgets/issues/7","user":{"login":"alice"}}`)
			require.NoError(t, err)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	client, err := hectorgithub.NewClient(hectorgithub.Config{
		AppID:          123,
		InstallationID: 456,
		PrivateKeyPath: privateKeyPath,
		APIURL:         server.URL,
	})
	require.NoError(t, err)

	issue, err := client.GetIssue(t.Context(), hectorgithub.Repository{
		Owner: "acme",
		Name:  "widgets",
	}, 7)
	require.NoError(t, err)

	require.NotEmpty(t, tokenAuthHeader)
	require.Equal(t, "Bearer installation-token", issueAuthHeader)
	require.Equal(t, 7, issue.Number)
	require.Equal(t, "Fix widget parser", issue.Title)
	require.Equal(t, "open", issue.State)
	require.Equal(t, "alice", issue.Author)
}

func TestClientRejectsInvalidConfig(t *testing.T) {
	t.Parallel()

	_, err := hectorgithub.NewClient(hectorgithub.Config{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "github: invalid config")
}

func TestClientRejectsUnsupportedAuthType(t *testing.T) {
	t.Parallel()

	_, err := hectorgithub.NewClient(hectorgithub.Config{
		AuthType:       "oauth_user",
		AppID:          123,
		InstallationID: 456,
		PrivateKeyPath: writeTestPrivateKey(t),
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), `github: unsupported auth type "oauth_user"`)
}

func writeTestPrivateKey(t *testing.T) string {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	der := x509.MarshalPKCS1PrivateKey(key)
	block := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: der}
	path := filepath.Join(t.TempDir(), "github-app.pem")
	require.NoError(t, os.WriteFile(path, pem.EncodeToMemory(block), 0o600))
	return path
}
