package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Repository identifies a GitHub repository.
type Repository struct {
	Owner string
	Name  string
}

// Issue is the normalized issue shape returned by the GitHub integration.
type Issue struct {
	ID     int64  `json:"id"`
	Number int    `json:"number"`
	Title  string `json:"title"`
	State  string `json:"state"`
	Body   string `json:"body"`
	URL    string `json:"url"`
	Author string `json:"author"`
}

// AccessToken is a bearer token that can authorize GitHub API requests.
type AccessToken struct {
	Value     string
	ExpiresAt time.Time
}

// TokenProvider supplies GitHub bearer tokens to the client.
type TokenProvider interface {
	Token(context.Context) (AccessToken, error)
}

// ClientConfig contains the non-auth settings needed by the GitHub REST client.
type ClientConfig struct {
	// APIURL overrides the GitHub REST API base URL. Leave empty to use the default.
	APIURL string
}

// Client reads GitHub data through an injected token provider.
type Client struct {
	apiURL     string
	httpClient *http.Client
	tokens     TokenProvider
}

// Option customizes a Client.
type Option func(*Client)

// WithHTTPClient replaces the HTTP client used for GitHub API calls.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		if httpClient != nil {
			c.httpClient = httpClient
		}
	}
}

// NewClient validates config, builds the configured token provider, and returns a client.
func NewClient(cfg Config, opts ...Option) (*Client, error) {
	if err := validate.Struct(cfg); err != nil {
		return nil, fmt.Errorf("github: invalid config: %w", err)
	}
	tokenProvider, err := NewTokenProvider(cfg)
	if err != nil {
		return nil, err
	}
	return NewClientWithTokenProvider(ClientConfig{APIURL: cfg.APIURL}, tokenProvider, opts...)
}

// NewClientWithTokenProvider returns a GitHub client using the supplied token provider.
func NewClientWithTokenProvider(cfg ClientConfig, tokenProvider TokenProvider, opts ...Option) (*Client, error) {
	if tokenProvider == nil {
		return nil, fmt.Errorf("github: token provider is required")
	}
	apiURL, err := apiURL(cfg.APIURL)
	if err != nil {
		return nil, fmt.Errorf("github: invalid client config: %w", err)
	}
	c := &Client{
		apiURL:     strings.TrimRight(apiURL, "/"),
		httpClient: http.DefaultClient,
		tokens:     tokenProvider,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c, nil
}

// GetIssue fetches a single issue by repository and issue number.
func (c *Client) GetIssue(ctx context.Context, repo Repository, number int) (Issue, error) {
	if repo.Owner == "" {
		return Issue{}, fmt.Errorf("github: repository owner is required")
	}
	if repo.Name == "" {
		return Issue{}, fmt.Errorf("github: repository name is required")
	}
	if number <= 0 {
		return Issue{}, fmt.Errorf("github: issue number must be positive")
	}
	token, err := c.tokens.Token(ctx)
	if err != nil {
		return Issue{}, err
	}
	path := fmt.Sprintf("/repos/%s/%s/issues/%d", url.PathEscape(repo.Owner), url.PathEscape(repo.Name), number)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.apiURL+path, nil)
	if err != nil {
		return Issue{}, fmt.Errorf("github: create issue request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+token.Value)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	var response githubIssueResponse
	if err := doJSON(c.httpClient, req, http.StatusOK, &response); err != nil {
		return Issue{}, err
	}
	return response.issue(), nil
}

// VerifyIssueRead checks that GitHub App authentication can read one issue.
func (c *Client) VerifyIssueRead(ctx context.Context, repo Repository, number int) error {
	_, err := c.GetIssue(ctx, repo, number)
	return err
}

func doJSON(httpClient *http.Client, req *http.Request, wantStatus int, out any) error {
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("github: %s %s: %w", req.Method, req.URL.Path, err)
	}
	if resp == nil {
		return fmt.Errorf("github: %s %s: empty response", req.Method, req.URL.Path)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("github: read response: %w", err)
	}
	if resp.StatusCode != wantStatus {
		return fmt.Errorf("github: %s %s: unexpected status %d: %s", req.Method, req.URL.Path, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("github: decode response: %w", err)
	}
	return nil
}

type githubIssueResponse struct {
	ID      int64  `json:"id"`
	Number  int    `json:"number"`
	Title   string `json:"title"`
	State   string `json:"state"`
	Body    string `json:"body"`
	HTMLURL string `json:"html_url"`
	User    struct {
		Login string `json:"login"`
	} `json:"user"`
}

func (r githubIssueResponse) issue() Issue {
	return Issue{
		ID:     r.ID,
		Number: r.Number,
		Title:  r.Title,
		State:  r.State,
		Body:   r.Body,
		URL:    r.HTMLURL,
		Author: r.User.Login,
	}
}
