package github_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	gh "github.com/iamhectordev/hector/integrations/github"
	"github.com/stretchr/testify/require"
)

type staticTokenProvider struct {
	token gh.AccessToken
}

func (p staticTokenProvider) Token(context.Context) (gh.AccessToken, error) {
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

	client, err := gh.NewClientWithTokenProvider(gh.ClientConfig{
		APIURL: server.URL,
	}, staticTokenProvider{token: gh.AccessToken{
		Value:     "provided-token",
		ExpiresAt: time.Now().Add(time.Hour),
	}})
	require.NoError(t, err)

	issue, err := client.GetIssue(t.Context(), gh.Repository{
		Owner: "acme",
		Name:  "widgets",
	}, 7)
	require.NoError(t, err)

	require.Equal(t, "Bearer provided-token", issueAuthHeader)
	require.Equal(t, "Fix widget parser", issue.Title)
}

func TestClientGetsIssueWithBlockingContext(t *testing.T) {
	t.Parallel()

	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.RequestURI())
		switch r.URL.Path {
		case "/repos/acme/widgets/issues/7":
			_, err := fmt.Fprint(w, `{"id":99,"number":7,"title":"Build tools","state":"open","body":"Work to do.","html_url":"https://github.com/acme/widgets/issues/7","user":{"login":"alice"}}`)
			require.NoError(t, err)
		case "/repos/acme/widgets/issues/7/dependencies/blocked_by":
			_, err := fmt.Fprint(w, `[{"id":88,"number":6,"title":"Design API","state":"closed","html_url":"https://github.com/acme/widgets/issues/6"}]`)
			require.NoError(t, err)
		case "/repos/acme/widgets/issues/7/dependencies/blocking":
			_, err := fmt.Fprint(w, `[{"id":111,"number":9,"title":"Wire Slack UX","state":"open","html_url":"https://github.com/acme/widgets/issues/9"}]`)
			require.NoError(t, err)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	client, err := gh.NewClientWithTokenProvider(gh.ClientConfig{
		APIURL: server.URL,
	}, staticTokenProvider{token: gh.AccessToken{
		Value:     "provided-token",
		ExpiresAt: time.Now().Add(time.Hour),
	}})
	require.NoError(t, err)

	issue, err := client.GetIssueWithBlocking(t.Context(), gh.Repository{
		Owner: "acme",
		Name:  "widgets",
	}, 7)
	require.NoError(t, err)

	require.Equal(t, []string{
		"/repos/acme/widgets/issues/7",
		"/repos/acme/widgets/issues/7/dependencies/blocked_by",
		"/repos/acme/widgets/issues/7/dependencies/blocking",
	}, paths)
	require.Equal(t, "Build tools", issue.Title)
	require.Equal(t, []gh.IssueSummary{{
		ID:     88,
		Number: 6,
		Title:  "Design API",
		State:  "closed",
		URL:    "https://github.com/acme/widgets/issues/6",
	}}, issue.BlockedBy)
	require.Equal(t, []gh.IssueSummary{{
		ID:     111,
		Number: 9,
		Title:  "Wire Slack UX",
		State:  "open",
		URL:    "https://github.com/acme/widgets/issues/9",
	}}, issue.Blocks)
}

func TestClientManagesMilestones(t *testing.T) {
	t.Parallel()

	var requests []string
	var createBody string
	var updateBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.RequestURI())
		switch {
		case r.Method == http.MethodGet && r.URL.RequestURI() == "/repos/acme/widgets/milestones?state=all":
			_, err := fmt.Fprint(w, `[{"id":10,"number":1,"title":"v1","state":"open","description":"First","html_url":"https://github.com/acme/widgets/milestone/1","open_issues":2,"closed_issues":3}]`)
			require.NoError(t, err)
		case r.Method == http.MethodPost && r.URL.Path == "/repos/acme/widgets/milestones":
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			createBody = string(body)
			w.WriteHeader(http.StatusCreated)
			_, err = fmt.Fprint(w, `{"id":11,"number":2,"title":"v2","state":"open","description":"Second","html_url":"https://github.com/acme/widgets/milestone/2","open_issues":0,"closed_issues":0}`)
			require.NoError(t, err)
		case r.Method == http.MethodPatch && r.URL.Path == "/repos/acme/widgets/milestones/2":
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			updateBody = string(body)
			_, err = fmt.Fprint(w, `{"id":11,"number":2,"title":"v2.1","state":"open","description":"Updated","html_url":"https://github.com/acme/widgets/milestone/2","open_issues":1,"closed_issues":0}`)
			require.NoError(t, err)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	client, err := gh.NewClientWithTokenProvider(gh.ClientConfig{
		APIURL: server.URL,
	}, staticTokenProvider{token: gh.AccessToken{
		Value:     "provided-token",
		ExpiresAt: time.Now().Add(time.Hour),
	}})
	require.NoError(t, err)

	milestones, err := client.ListMilestones(t.Context(), gh.Repository{Owner: "acme", Name: "widgets"}, gh.WithMilestoneState("all"))
	require.NoError(t, err)
	created, err := client.CreateMilestone(t.Context(), gh.Repository{Owner: "acme", Name: "widgets"}, "v2", gh.WithMilestoneDescription("Second"))
	require.NoError(t, err)
	updated, err := client.UpdateMilestone(t.Context(), gh.Repository{Owner: "acme", Name: "widgets"}, 2, gh.WithMilestoneTitle("v2.1"), gh.WithMilestoneDescription("Updated"))
	require.NoError(t, err)

	require.Equal(t, []string{
		"GET /repos/acme/widgets/milestones?state=all",
		"POST /repos/acme/widgets/milestones",
		"PATCH /repos/acme/widgets/milestones/2",
	}, requests)
	require.JSONEq(t, `{"title":"v2","description":"Second"}`, createBody)
	require.JSONEq(t, `{"title":"v2.1","description":"Updated"}`, updateBody)
	require.Equal(t, "v1", milestones[0].Title)
	require.Equal(t, "v2", created.Title)
	require.Equal(t, "v2.1", updated.Title)
	require.Equal(t, 1, updated.OpenIssues)
}

func TestClientManagesBlockedByRelationships(t *testing.T) {
	t.Parallel()

	var requests []string
	var createBody string
	var removeBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.RequestURI())
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/repos/acme/widgets/issues/6":
			_, err := fmt.Fprint(w, `{"id":88,"number":6,"title":"Design API","state":"open","html_url":"https://github.com/acme/widgets/issues/6","user":{"login":"alice"}}`)
			require.NoError(t, err)
		case r.Method == http.MethodPost && r.URL.Path == "/repos/acme/widgets/issues/7/dependencies/blocked_by":
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			createBody = string(body)
			w.WriteHeader(http.StatusCreated)
			_, err = fmt.Fprint(w, `{}`)
			require.NoError(t, err)
		case r.Method == http.MethodDelete && r.URL.Path == "/repos/acme/widgets/issues/7/dependencies/blocked_by":
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			removeBody = string(body)
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodGet && r.URL.Path == "/repos/acme/widgets/issues/7":
			_, err := fmt.Fprint(w, `{"id":99,"number":7,"title":"Build tools","state":"open","body":"Work to do.","html_url":"https://github.com/acme/widgets/issues/7","user":{"login":"alice"}}`)
			require.NoError(t, err)
		case r.Method == http.MethodGet && r.URL.Path == "/repos/acme/widgets/issues/7/dependencies/blocked_by":
			_, err := fmt.Fprint(w, `[{"id":88,"number":6,"title":"Design API","state":"open","html_url":"https://github.com/acme/widgets/issues/6"}]`)
			require.NoError(t, err)
		case r.Method == http.MethodGet && r.URL.Path == "/repos/acme/widgets/issues/7/dependencies/blocking":
			_, err := fmt.Fprint(w, `[]`)
			require.NoError(t, err)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	client, err := gh.NewClientWithTokenProvider(gh.ClientConfig{
		APIURL: server.URL,
	}, staticTokenProvider{token: gh.AccessToken{
		Value:     "provided-token",
		ExpiresAt: time.Now().Add(time.Hour),
	}})
	require.NoError(t, err)

	created, err := client.AddBlockedBy(t.Context(), gh.Repository{Owner: "acme", Name: "widgets"}, 6, 7)
	require.NoError(t, err)
	removed, err := client.RemoveBlockedBy(t.Context(), gh.Repository{Owner: "acme", Name: "widgets"}, 6, 7)
	require.NoError(t, err)

	require.Equal(t, []string{
		"GET /repos/acme/widgets/issues/6",
		"POST /repos/acme/widgets/issues/7/dependencies/blocked_by",
		"GET /repos/acme/widgets/issues/7",
		"GET /repos/acme/widgets/issues/7/dependencies/blocked_by",
		"GET /repos/acme/widgets/issues/7/dependencies/blocking",
		"GET /repos/acme/widgets/issues/6",
		"DELETE /repos/acme/widgets/issues/7/dependencies/blocked_by",
		"GET /repos/acme/widgets/issues/7",
		"GET /repos/acme/widgets/issues/7/dependencies/blocked_by",
		"GET /repos/acme/widgets/issues/7/dependencies/blocking",
	}, requests)
	require.JSONEq(t, `{"issue_id":88}`, createBody)
	require.JSONEq(t, `{"issue_id":88}`, removeBody)
	require.Equal(t, "Build tools", created.Title)
	require.Equal(t, "Build tools", removed.Title)
	require.Equal(t, 6, created.BlockedBy[0].Number)
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

	client, err := gh.NewClient(gh.Config{
		AppID:          123,
		InstallationID: 456,
		PrivateKeyPath: privateKeyPath,
		APIURL:         server.URL,
	})
	require.NoError(t, err)

	issue, err := client.GetIssue(t.Context(), gh.Repository{
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

	_, err := gh.NewClient(gh.Config{Enabled: true})
	require.Error(t, err)
	require.Contains(t, err.Error(), "github: invalid config")
}

func TestClientRejectsUnsupportedAuthType(t *testing.T) {
	t.Parallel()

	_, err := gh.NewClient(gh.Config{
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
