package safehttp_test

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/iamhectordev/hector/pkg/safehttp"
	"github.com/stretchr/testify/require"
)

// fakeResolver returns IPs in round-robin order across successive LookupHost calls.
type fakeResolver struct {
	calls atomic.Int32
	ips   []string // returned in rotation
}

func (r *fakeResolver) LookupHost(_ context.Context, _ string) ([]string, error) {
	idx := int(r.calls.Add(1)) - 1
	return []string{r.ips[idx%len(r.ips)]}, nil
}

func okServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func get(t *testing.T, client *http.Client, url string) (*http.Response, error) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	require.NoError(t, err)
	return client.Do(req)
}

// TestClient_WithDialContext verifies that passing a base transport with DialContext
// already set returns an error — we must own the dialer to enforce SSRF protections.
func TestClient_WithDialContext(t *testing.T) {
	t.Parallel()
	base := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return nil, nil
		},
	}
	_, err := safehttp.Client(safehttp.WithBaseTransport(base))
	require.Error(t, err)
	require.Contains(t, err.Error(), "DialContext")
}

func TestCheckIP_Blocked(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		ip   string
	}{
		{"loopback_v4", "127.0.0.1"},
		{"loopback_v6", "::1"},
		{"link_local_v4", "169.254.169.254"},
		{"link_local_v4_other", "169.254.0.1"},
		{"link_local_v6", "fe80::1"},
		{"rfc1918_10", "10.0.0.1"},
		{"rfc1918_172", "172.16.0.1"},
		{"rfc1918_192", "192.168.1.1"},
		{"multicast_v4", "224.0.0.1"},
		{"multicast_v6", "ff02::1"},
		{"unspecified_v4", "0.0.0.0"},
		{"unspecified_v6", "::"},
		{"ipv6_ula", "fc00::1"},
		{"ipv4_mapped_private", "::ffff:10.0.0.1"},
		{"ipv4_mapped_loopback", "::ffff:127.0.0.1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := &fakeResolver{ips: []string{tc.ip}}
			client, err := safehttp.Client(safehttp.WithResolver(r))
			require.NoError(t, err)

			// Use a placeholder URL — the dialer fires before TCP connection.
			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://target.internal/", nil)
			require.NoError(t, err)
			_, err = client.Do(req)
			require.ErrorIs(t, err, safehttp.ErrBlockedAddress, "expected blocked address for IP %s", tc.ip)
		})
	}
}

func TestCheckIP_PublicAllowed(t *testing.T) {
	t.Parallel()
	srv := okServer(t)
	r := &fakeResolver{ips: []string{"1.2.3.4"}}
	// Short timeout: the dial to 1.2.3.4 won't reach the test server, but we only
	// care that the address is not blocked — not that the connection succeeds.
	client, err := safehttp.Client(safehttp.WithResolver(r), safehttp.WithTimeout(200*time.Millisecond))
	require.NoError(t, err)

	_, err = get(t, client, srv.URL)
	require.False(t, errors.Is(err, safehttp.ErrBlockedAddress), "public IP should not be blocked")
}

func TestClient_AllowLoopback(t *testing.T) {
	t.Parallel()
	srv := okServer(t)
	client, err := safehttp.Client(safehttp.WithAllowLoopback())
	require.NoError(t, err)

	resp, err := get(t, client, srv.URL)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestClient_BlockedScheme(t *testing.T) {
	t.Parallel()
	client, err := safehttp.Client(safehttp.WithAllowLoopback())
	require.NoError(t, err)

	_, err = get(t, client, "ftp://example.com/resource")
	require.ErrorIs(t, err, safehttp.ErrBlockedScheme)
}

func TestClient_BlockedAddress_ViaResolver(t *testing.T) {
	t.Parallel()
	r := &fakeResolver{ips: []string{"10.0.0.1"}}
	client, err := safehttp.Client(safehttp.WithResolver(r))
	require.NoError(t, err)

	_, err = get(t, client, "http://internal.corp/")
	require.ErrorIs(t, err, safehttp.ErrBlockedAddress)
}

func TestClient_Redirect_BlockedAddress(t *testing.T) {
	t.Parallel()
	// Server redirects to a URL that resolves to a private IP.
	redirected := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !redirected {
			redirected = true
			http.Redirect(w, r, "http://internal.corp/secret", http.StatusFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	r := &fakeResolver{ips: []string{"127.0.0.1", "10.0.0.1"}}
	// call 1 → srv IP (127.0.0.1, allowed via AllowLoopback)
	// call 2 → 10.0.0.1 (private, blocked)
	client, err := safehttp.Client(safehttp.WithAllowLoopback(), safehttp.WithResolver(r))
	require.NoError(t, err)

	_, err = get(t, client, srv.URL)
	require.ErrorIs(t, err, safehttp.ErrBlockedAddress)
}

// TestClient_TOCTOU verifies that the dialer calls LookupHost exactly once per
// connection (DNS pinning). If it were to re-resolve, the second call would return
// a private IP and the connection would be blocked.
func TestClient_TOCTOU(t *testing.T) {
	t.Parallel()
	srv := okServer(t)

	// Call 1: safe (127.0.0.1). Call 2+: private (10.0.0.1).
	// A single request must only trigger one LookupHost call.
	r := &fakeResolver{ips: []string{"127.0.0.1", "10.0.0.1"}}
	client, err := safehttp.Client(safehttp.WithAllowLoopback(), safehttp.WithResolver(r))
	require.NoError(t, err)

	resp, err := get(t, client, srv.URL)
	require.NoError(t, err, "request should succeed: resolver was called only once, pinned to safe IP")
	require.NotNil(t, resp)
	require.NoError(t, resp.Body.Close())
	require.EqualValues(t, 1, r.calls.Load(), "LookupHost must be called exactly once per connection")
}

func TestClient_Oversize(t *testing.T) {
	t.Parallel()
	const limit = 1024
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		// Write more than the limit.
		_, _ = io.Copy(w, io.LimitReader(strings.NewReader(strings.Repeat("x", limit+1)), limit+1))
	}))
	t.Cleanup(srv.Close)

	client, err := safehttp.Client(safehttp.WithAllowLoopback(), safehttp.WithMaxBodyBytes(limit))
	require.NoError(t, err)

	resp, err := get(t, client, srv.URL)
	require.NoError(t, err) // Do itself succeeds; oversize detected on body read
	require.NotNil(t, resp)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	_, readErr := io.ReadAll(resp.Body)
	require.ErrorIs(t, readErr, safehttp.ErrOversize)
}

func TestClient_Timeout(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
	}))
	t.Cleanup(srv.Close)

	client, err := safehttp.Client(safehttp.WithAllowLoopback(), safehttp.WithTimeout(10*time.Millisecond))
	require.NoError(t, err)

	_, err = get(t, client, srv.URL)
	require.Error(t, err)
	var netErr interface{ Timeout() bool }
	require.True(t, errors.As(err, &netErr), "expected timeout error, got: %v", err)
	require.True(t, netErr.Timeout(), "expected Timeout()=true, got: %v", err)
}
