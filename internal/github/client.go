package github

import (
	"bytes"
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

// IssueSummary is the compact issue shape used for dependency context.
type IssueSummary struct {
	ID     int64  `json:"id"`
	Number int    `json:"number"`
	Title  string `json:"title"`
	State  string `json:"state"`
	URL    string `json:"url"`
}

// IssueWithBlocking is an issue enriched with its blocking relationships.
type IssueWithBlocking struct {
	Issue
	BlockedBy []IssueSummary `json:"blocked_by"`
	Blocks    []IssueSummary `json:"blocks"`
}

// Milestone is the normalized milestone shape returned by the GitHub integration.
type Milestone struct {
	ID           int64  `json:"id"`
	Number       int    `json:"number"`
	Title        string `json:"title"`
	State        string `json:"state"`
	Description  string `json:"description"`
	URL          string `json:"url"`
	OpenIssues   int    `json:"open_issues"`
	ClosedIssues int    `json:"closed_issues"`
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

type listMilestonesConfig struct {
	state string
}

// ListMilestonesOption customizes a milestone listing request.
type ListMilestonesOption func(*listMilestonesConfig)

// WithMilestoneState filters milestones by state: open, closed, or all.
func WithMilestoneState(state string) ListMilestonesOption {
	return func(cfg *listMilestonesConfig) {
		cfg.state = state
	}
}

type milestoneConfig struct {
	title       *string
	description *string
}

// MilestoneOption customizes a create or update milestone request.
type MilestoneOption func(*milestoneConfig)

// WithMilestoneTitle sets the milestone title.
func WithMilestoneTitle(title string) MilestoneOption {
	return func(cfg *milestoneConfig) {
		cfg.title = &title
	}
}

// WithMilestoneDescription sets the milestone description.
func WithMilestoneDescription(description string) MilestoneOption {
	return func(cfg *milestoneConfig) {
		cfg.description = &description
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
	path := fmt.Sprintf("/repos/%s/%s/issues/%d", url.PathEscape(repo.Owner), url.PathEscape(repo.Name), number)
	var response githubIssueResponse
	if err := c.do(ctx, http.MethodGet, path, nil, http.StatusOK, &response); err != nil {
		return Issue{}, err
	}
	return response.issue(), nil
}

// GetIssueWithBlocking fetches an issue and its blocking dependency context.
func (c *Client) GetIssueWithBlocking(ctx context.Context, repo Repository, number int) (IssueWithBlocking, error) {
	issue, err := c.GetIssue(ctx, repo, number)
	if err != nil {
		return IssueWithBlocking{}, err
	}
	blockedBy, err := c.listIssueDependencies(ctx, repo, number, "blocked_by")
	if err != nil {
		return IssueWithBlocking{}, err
	}
	blocks, err := c.listIssueDependencies(ctx, repo, number, "blocking")
	if err != nil {
		return IssueWithBlocking{}, err
	}
	return IssueWithBlocking{
		Issue:     issue,
		BlockedBy: blockedBy,
		Blocks:    blocks,
	}, nil
}

// ListMilestones fetches repository milestones.
func (c *Client) ListMilestones(ctx context.Context, repo Repository, opts ...ListMilestonesOption) ([]Milestone, error) {
	if err := validateRepository(repo); err != nil {
		return nil, err
	}
	cfg := listMilestonesConfig{state: "open"}
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.state != "open" && cfg.state != "closed" && cfg.state != "all" {
		return nil, fmt.Errorf("github: milestone state must be open, closed, or all")
	}
	path := fmt.Sprintf("/repos/%s/%s/milestones", url.PathEscape(repo.Owner), url.PathEscape(repo.Name))
	values := url.Values{}
	values.Set("state", cfg.state)
	var response []githubMilestoneResponse
	if err := c.do(ctx, http.MethodGet, path+"?"+values.Encode(), nil, http.StatusOK, &response); err != nil {
		return nil, err
	}
	out := make([]Milestone, 0, len(response))
	for _, milestone := range response {
		out = append(out, milestone.milestone())
	}
	return out, nil
}

// CreateMilestone creates a repository milestone.
func (c *Client) CreateMilestone(ctx context.Context, repo Repository, title string, opts ...MilestoneOption) (Milestone, error) {
	if err := validateRepository(repo); err != nil {
		return Milestone{}, err
	}
	if title == "" {
		return Milestone{}, fmt.Errorf("github: milestone title is required")
	}
	cfg := milestoneConfig{title: &title}
	for _, opt := range opts {
		opt(&cfg)
	}
	path := fmt.Sprintf("/repos/%s/%s/milestones", url.PathEscape(repo.Owner), url.PathEscape(repo.Name))
	var response githubMilestoneResponse
	if err := c.do(ctx, http.MethodPost, path, milestoneRequest(cfg), http.StatusCreated, &response); err != nil {
		return Milestone{}, err
	}
	return response.milestone(), nil
}

// UpdateMilestone updates a repository milestone.
func (c *Client) UpdateMilestone(ctx context.Context, repo Repository, number int, opts ...MilestoneOption) (Milestone, error) {
	if err := validateRepository(repo); err != nil {
		return Milestone{}, err
	}
	if number <= 0 {
		return Milestone{}, fmt.Errorf("github: milestone number must be positive")
	}
	cfg := milestoneConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.title == nil && cfg.description == nil {
		return Milestone{}, fmt.Errorf("github: milestone update requires title or description")
	}
	path := fmt.Sprintf("/repos/%s/%s/milestones/%d", url.PathEscape(repo.Owner), url.PathEscape(repo.Name), number)
	var response githubMilestoneResponse
	if err := c.do(ctx, http.MethodPatch, path, milestoneRequest(cfg), http.StatusOK, &response); err != nil {
		return Milestone{}, err
	}
	return response.milestone(), nil
}

// AddBlockedBy creates a dependency where blockedIssueNumber is blocked by blockingIssueNumber.
func (c *Client) AddBlockedBy(ctx context.Context, repo Repository, blockingIssueNumber int, blockedIssueNumber int) (IssueWithBlocking, error) {
	if blockingIssueNumber <= 0 {
		return IssueWithBlocking{}, fmt.Errorf("github: blocking issue number must be positive")
	}
	if blockedIssueNumber <= 0 {
		return IssueWithBlocking{}, fmt.Errorf("github: blocked issue number must be positive")
	}
	blockingIssue, err := c.GetIssue(ctx, repo, blockingIssueNumber)
	if err != nil {
		return IssueWithBlocking{}, err
	}
	path := fmt.Sprintf("/repos/%s/%s/issues/%d/dependencies/blocked_by", url.PathEscape(repo.Owner), url.PathEscape(repo.Name), blockedIssueNumber)
	if err := c.do(ctx, http.MethodPost, path, issueDependencyRequest{IssueID: blockingIssue.ID}, http.StatusCreated, nil); err != nil {
		return IssueWithBlocking{}, err
	}
	return c.GetIssueWithBlocking(ctx, repo, blockedIssueNumber)
}

// RemoveBlockedBy removes a dependency where blockedIssueNumber is blocked by blockingIssueNumber.
func (c *Client) RemoveBlockedBy(ctx context.Context, repo Repository, blockingIssueNumber int, blockedIssueNumber int) (IssueWithBlocking, error) {
	if blockingIssueNumber <= 0 {
		return IssueWithBlocking{}, fmt.Errorf("github: blocking issue number must be positive")
	}
	if blockedIssueNumber <= 0 {
		return IssueWithBlocking{}, fmt.Errorf("github: blocked issue number must be positive")
	}
	blockingIssue, err := c.GetIssue(ctx, repo, blockingIssueNumber)
	if err != nil {
		return IssueWithBlocking{}, err
	}
	path := fmt.Sprintf("/repos/%s/%s/issues/%d/dependencies/blocked_by", url.PathEscape(repo.Owner), url.PathEscape(repo.Name), blockedIssueNumber)
	if err := c.do(ctx, http.MethodDelete, path, issueDependencyRequest{IssueID: blockingIssue.ID}, http.StatusNoContent, nil); err != nil {
		return IssueWithBlocking{}, err
	}
	return c.GetIssueWithBlocking(ctx, repo, blockedIssueNumber)
}

func (c *Client) listIssueDependencies(ctx context.Context, repo Repository, number int, kind string) ([]IssueSummary, error) {
	path := fmt.Sprintf("/repos/%s/%s/issues/%d/dependencies/%s", url.PathEscape(repo.Owner), url.PathEscape(repo.Name), number, kind)
	var response []githubIssueResponse
	if err := c.do(ctx, http.MethodGet, path, nil, http.StatusOK, &response); err != nil {
		return nil, err
	}
	out := make([]IssueSummary, 0, len(response))
	for _, issue := range response {
		out = append(out, issue.summary())
	}
	return out, nil
}

func (c *Client) do(ctx context.Context, method, path string, body any, wantStatus int, out any) error {
	token, err := c.tokens.Token(ctx)
	if err != nil {
		return err
	}
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("github: encode request: %w", err)
		}
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.apiURL+path, reader)
	if err != nil {
		return fmt.Errorf("github: create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+token.Value)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return doJSON(c.httpClient, req, wantStatus, out)
}

func validateRepository(repo Repository) error {
	if repo.Owner == "" {
		return fmt.Errorf("github: repository owner is required")
	}
	if repo.Name == "" {
		return fmt.Errorf("github: repository name is required")
	}
	return nil
}

func milestoneRequest(cfg milestoneConfig) map[string]string {
	request := map[string]string{}
	if cfg.title != nil {
		request["title"] = *cfg.title
	}
	if cfg.description != nil {
		request["description"] = *cfg.description
	}
	return request
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
	if out == nil {
		return nil
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

type issueDependencyRequest struct {
	IssueID int64 `json:"issue_id"`
}

type githubMilestoneResponse struct {
	ID           int64  `json:"id"`
	Number       int    `json:"number"`
	Title        string `json:"title"`
	State        string `json:"state"`
	Description  string `json:"description"`
	HTMLURL      string `json:"html_url"`
	OpenIssues   int    `json:"open_issues"`
	ClosedIssues int    `json:"closed_issues"`
}

func (r githubMilestoneResponse) milestone() Milestone {
	return Milestone{
		ID:           r.ID,
		Number:       r.Number,
		Title:        r.Title,
		State:        r.State,
		Description:  r.Description,
		URL:          r.HTMLURL,
		OpenIssues:   r.OpenIssues,
		ClosedIssues: r.ClosedIssues,
	}
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

func (r githubIssueResponse) summary() IssueSummary {
	return IssueSummary{
		ID:     r.ID,
		Number: r.Number,
		Title:  r.Title,
		State:  r.State,
		URL:    r.HTMLURL,
	}
}
