// Package safehttp provides a hardened *http.Client that blocks SSRF via scheme
// enforcement, DNS resolution checks, DNS pinning, redirect re-validation,
// and response body size capping.
package safehttp

import (
	"context"
	"errors"
	"net/http"
	"time"
)

const (
	defaultTimeout      = 15 * time.Second
	defaultMaxBodyBytes = 5 * 1024 * 1024
)

// ErrBlockedScheme is returned when a request targets a non-HTTP/HTTPS scheme.
var ErrBlockedScheme = errors.New("safehttp: blocked scheme")

// ErrBlockedAddress is returned when DNS resolution yields a private, loopback,
// link-local, or otherwise disallowed IP address.
var ErrBlockedAddress = errors.New("safehttp: blocked address")

// ErrOversize is returned when a response body exceeds the configured size limit.
var ErrOversize = errors.New("safehttp: response body exceeds size limit")

// Resolver resolves hostnames to IP address strings.
type Resolver interface {
	LookupHost(ctx context.Context, host string) (addrs []string, err error)
}

type config struct {
	timeout       time.Duration
	maxBodyBytes  int64
	allowLoopback bool
	resolver      Resolver
	baseTransport *http.Transport
}

// Option configures [Client].
type Option func(*config)

// WithTimeout sets the end-to-end request timeout. Default: 15s.
func WithTimeout(d time.Duration) Option {
	return func(c *config) { c.timeout = d }
}

// WithMaxBodyBytes caps the response body size. Default: 5 MiB.
// Reads past the cap return [ErrOversize].
func WithMaxBodyBytes(n int64) Option {
	return func(c *config) { c.maxBodyBytes = n }
}

// WithAllowLoopback permits connections to loopback addresses (127.x.x.x, ::1).
// Intended for test and local-dev use only.
func WithAllowLoopback() Option {
	return func(c *config) { c.allowLoopback = true }
}

// WithResolver injects a custom [Resolver]. Default: net.DefaultResolver.
// Primarily used in tests to control DNS responses and verify DNS pinning.
func WithResolver(r Resolver) Option {
	return func(c *config) { c.resolver = r }
}

// WithBaseTransport sets the underlying *http.Transport that Client wraps.
// The provided transport's DialContext field must be nil — Client installs its
// own safe dialer and returns an error if DialContext is already set. The
// transport is cloned before modification; the caller's value is not mutated.
func WithBaseTransport(t *http.Transport) Option {
	return func(c *config) { c.baseTransport = t }
}

// Client returns a hardened *http.Client configured with SSRF protections:
//
//   - Only http and https schemes are allowed ([ErrBlockedScheme] otherwise).
//   - Each hostname is resolved via DNS; any address in a blocked range yields
//     [ErrBlockedAddress] (loopback, link-local, RFC1918, multicast, unspecified,
//     IPv6 ULA fc00::/7, IPv4-mapped IPv6).
//   - DNS pinning: the checked IP is dialled directly, preventing DNS-rebinding
//     TOCTOU attacks. The original Host header and TLS SNI are preserved.
//   - Each redirect hop is re-validated for scheme; IP checks fire via the dialer.
//   - Response bodies are capped at [WithMaxBodyBytes]; reads past the cap return
//     [ErrOversize].
//   - A configurable request timeout is applied to the whole round-trip.
func Client(opts ...Option) (*http.Client, error) {
	cfg := &config{
		timeout:      defaultTimeout,
		maxBodyBytes: defaultMaxBodyBytes,
		resolver:     defaultResolver,
	}
	for _, o := range opts {
		o(cfg)
	}

	var transport *http.Transport
	if cfg.baseTransport == nil {
		transport = &http.Transport{}
	} else {
		if cfg.baseTransport.DialContext != nil {
			return nil, errors.New("safehttp: base transport must not have DialContext set; " +
				"Client installs its own safe dialer to enforce SSRF protections")
		}
		transport = cfg.baseTransport.Clone()
	}

	d := &safeDialer{
		inner:         newInnerDialer(),
		resolver:      cfg.resolver,
		allowLoopback: cfg.allowLoopback,
	}
	transport.DialContext = d.DialContext

	return &http.Client{
		Transport: &safeTransport{inner: transport, maxBodyBytes: cfg.maxBodyBytes},
		Timeout:   cfg.timeout,
		CheckRedirect: func(req *http.Request, _ []*http.Request) error {
			// IP re-validation fires automatically via the dialer on the next dial.
			// We only need to re-check the scheme here.
			return checkScheme(req.URL.Scheme)
		},
	}, nil
}
