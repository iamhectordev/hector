package github

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// NewTokenProvider returns the token provider selected by config.
func NewTokenProvider(cfg Config) (TokenProvider, error) {
	switch cfg.AuthType {
	case "", AuthTypeAppInstallation:
	default:
		return nil, fmt.Errorf("github: unsupported auth type %q", cfg.AuthType)
	}
	return NewAppInstallationTokenProvider(cfg)
}

// AppInstallationTokenProvider mints and caches GitHub App installation tokens.
type AppInstallationTokenProvider struct {
	appID          int64
	installationID int64
	privateKey     *rsa.PrivateKey
	apiURL         string
	httpClient     *http.Client
	now            func() time.Time

	mu    sync.Mutex
	token AccessToken
}

// NewAppInstallationTokenProvider validates config and returns an app-installation provider.
func NewAppInstallationTokenProvider(cfg Config) (*AppInstallationTokenProvider, error) {
	if err := validate.Struct(cfg); err != nil {
		return nil, fmt.Errorf("github: invalid config: %w", err)
	}
	apiURL, err := apiURL(cfg.APIURL)
	if err != nil {
		return nil, fmt.Errorf("github: invalid config: %w", err)
	}
	privateKey, err := loadPrivateKey(cfg.PrivateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("github: load private key: %w", err)
	}
	return &AppInstallationTokenProvider{
		appID:          cfg.AppID,
		installationID: cfg.InstallationID,
		privateKey:     privateKey,
		apiURL:         strings.TrimRight(apiURL, "/"),
		httpClient:     http.DefaultClient,
		now:            time.Now,
	}, nil
}

// Token returns a cached installation token, minting one when missing or near expiry.
func (p *AppInstallationTokenProvider) Token(ctx context.Context) (AccessToken, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.token.Value != "" && p.now().Add(5*time.Minute).Before(p.token.ExpiresAt) {
		return p.token, nil
	}
	jwt, err := p.signJWT()
	if err != nil {
		return AccessToken{}, err
	}
	path := "/app/installations/" + strconv.FormatInt(p.installationID, 10) + "/access_tokens"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.apiURL+path, bytes.NewReader(nil))
	if err != nil {
		return AccessToken{}, fmt.Errorf("github: create installation token request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	var response installationTokenResponse
	if err := doJSON(p.httpClient, req, http.StatusCreated, &response); err != nil {
		return AccessToken{}, err
	}
	if response.Token == "" {
		return AccessToken{}, fmt.Errorf("github: installation token response missing token")
	}
	p.token = AccessToken{Value: response.Token, ExpiresAt: response.ExpiresAt}
	return p.token, nil
}

func (p *AppInstallationTokenProvider) signJWT() (string, error) {
	now := p.now()
	header, err := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT"})
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(map[string]any{
		"iat": now.Add(-1 * time.Minute).Unix(),
		"exp": now.Add(9 * time.Minute).Unix(),
		"iss": p.appID,
	})
	if err != nil {
		return "", err
	}
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	sum := sha256.Sum256([]byte(unsigned))
	sig, err := rsa.SignPKCS1v15(rand.Reader, p.privateKey, crypto.SHA256, sum[:])
	if err != nil {
		return "", fmt.Errorf("github: sign app jwt: %w", err)
	}
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

func loadPrivateKey(path string) (*rsa.PrivateKey, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(contents)
	if block == nil {
		return nil, fmt.Errorf("invalid pem")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("private key is not RSA")
	}
	return key, nil
}

type installationTokenResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}
