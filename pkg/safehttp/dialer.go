package safehttp

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"time"
)

// defaultResolver satisfies Resolver using the system DNS.
var defaultResolver Resolver = net.DefaultResolver

func newInnerDialer() *net.Dialer {
	return &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}
}

// safeDialer wraps a net.Dialer to enforce DNS pinning and IP allowlist checks.
// It resolves the hostname once, validates every returned address, and dials
// the first allowed IP directly — preventing DNS-rebinding TOCTOU attacks.
type safeDialer struct {
	inner         *net.Dialer
	resolver      Resolver
	allowLoopback bool
}

func (d *safeDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("safehttp: split host/port %q: %w", addr, err)
	}

	// Resolve once. The checked IP is pinned — we never re-resolve before dialling.
	resolved, err := d.resolver.LookupHost(ctx, host)
	if err != nil {
		return nil, err
	}
	if len(resolved) == 0 {
		return nil, fmt.Errorf("%w: no addresses for %s", ErrBlockedAddress, host)
	}

	// Validate every resolved address; use the first one that passes.
	var firstErr error
	for _, a := range resolved {
		ip, parseErr := netip.ParseAddr(a)
		if parseErr != nil {
			continue
		}
		if checkErr := checkIP(ip, d.allowLoopback); checkErr != nil {
			if firstErr == nil {
				firstErr = checkErr
			}
			continue
		}
		// Dial the validated IP directly (DNS pin). net.Dialer preserves the
		// original Host header / TLS SNI because those are set at the http layer.
		pinnedAddr := net.JoinHostPort(ip.String(), port)
		return d.inner.DialContext(ctx, network, pinnedAddr)
	}

	if firstErr != nil {
		return nil, firstErr
	}
	return nil, fmt.Errorf("%w: no valid address resolved for %s", ErrBlockedAddress, host)
}
