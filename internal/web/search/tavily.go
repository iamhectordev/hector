package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-playground/validator/v10"
)

const defaultTavilyAPIURL = "https://api.tavily.com"

var validate = validator.New(validator.WithRequiredStructEnabled())

type TavilyConfig struct {
	APIKey string `yaml:"api_key" env:"TAVILY_API_KEY" validate:"required"`
	APIURL string `yaml:"api_url" env:"TAVILY_API_URL" validate:"omitempty,url"`
}

type Tavily struct {
	apiURL     string
	apiKey     string
	httpClient *http.Client
}

type TavilyOption func(*Tavily)

type tavilySearchRequest struct {
	Query       string `json:"query"`
	SearchDepth string `json:"search_depth"`
	MaxResults  int    `json:"max_results"`
}

type tavilySearchResponse struct {
	Results []tavilySearchResult `json:"results"`
}

type tavilySearchResult struct {
	Title   string   `json:"title"`
	URL     string   `json:"url"`
	Content string   `json:"content"`
	Score   *float64 `json:"score"`
}

func WithTavilyHTTPClient(httpClient *http.Client) TavilyOption {
	return func(t *Tavily) {
		t.httpClient = httpClient
	}
}

func NewTavily(cfg TavilyConfig, opts ...TavilyOption) (*Tavily, error) {
	if err := validate.Struct(cfg); err != nil {
		return nil, fmt.Errorf("tavily: invalid config: %w", err)
	}
	apiURL, err := tavilyAPIURL(cfg.APIURL)
	if err != nil {
		return nil, fmt.Errorf("tavily: invalid config: %w", err)
	}
	t := &Tavily{
		apiURL:     apiURL,
		apiKey:     cfg.APIKey,
		httpClient: http.DefaultClient,
	}
	for _, opt := range opts {
		opt(t)
	}
	if t.httpClient == nil {
		return nil, fmt.Errorf("tavily: http client is required")
	}
	return t, nil
}

func (t *Tavily) Verify(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.apiURL+"/usage", nil)
	if err != nil {
		return fmt.Errorf("tavily: verify: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+t.apiKey)

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("tavily: verify: %w", err)
	}
	if resp == nil {
		return fmt.Errorf("tavily: verify: no response")
	}
	defer func() { _ = resp.Body.Close() }()

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return nil
	case resp.StatusCode == http.StatusUnauthorized:
		return fmt.Errorf("tavily: verify: unauthorized")
	case resp.StatusCode == http.StatusTooManyRequests:
		return fmt.Errorf("tavily: verify: rate limited")
	default:
		return fmt.Errorf("tavily: verify: unexpected status %d", resp.StatusCode)
	}
}

func (t *Tavily) Search(ctx context.Context, query string) ([]Result, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("tavily: search: query is required")
	}

	body, err := json.Marshal(tavilySearchRequest{
		Query:       query,
		SearchDepth: "basic",
		MaxResults:  5,
	})
	if err != nil {
		return nil, fmt.Errorf("tavily: search: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.apiURL+"/search", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("tavily: search: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+t.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tavily: search: %w", err)
	}
	if resp == nil {
		return nil, fmt.Errorf("tavily: search: no response")
	}
	defer func() { _ = resp.Body.Close() }()

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
	case resp.StatusCode == http.StatusUnauthorized:
		return nil, fmt.Errorf("tavily: search: unauthorized")
	case resp.StatusCode == http.StatusTooManyRequests:
		return nil, fmt.Errorf("tavily: search: rate limited")
	default:
		return nil, fmt.Errorf("tavily: search: unexpected status %d", resp.StatusCode)
	}

	var out tavilySearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("tavily: search: decode response: %w", err)
	}

	results := make([]Result, 0, len(out.Results))
	for _, result := range out.Results {
		results = append(results, Result{
			Provider: ProviderTavily,
			URL:      result.URL,
			Title:    result.Title,
			Snippet:  result.Content,
			Score:    result.Score,
		})
	}
	return results, nil
}

func tavilyAPIURL(value string) (string, error) {
	if value == "" {
		return defaultTavilyAPIURL, nil
	}
	u, err := url.ParseRequestURI(value)
	if err != nil {
		return "", fmt.Errorf("invalid api_url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("invalid api_url scheme %q", u.Scheme)
	}
	return strings.TrimRight(value, "/"), nil
}
