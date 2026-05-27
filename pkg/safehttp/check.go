package safehttp

import (
	"fmt"
	"net/netip"
)

func checkScheme(scheme string) error {
	switch scheme {
	case "http", "https":
		return nil
	default:
		return fmt.Errorf("%w: %s", ErrBlockedScheme, scheme)
	}
}

// checkIP returns ErrBlockedAddress for any address that must not be dialled.
// allowLoopback exempts loopback addresses for dev/test environments.
func checkIP(ip netip.Addr, allowLoopback bool) error {
	// Unmap IPv4-in-IPv6 (e.g. ::ffff:10.0.0.1 → 10.0.0.1) so the IPv4 rules apply.
	ip = ip.Unmap()

	if ip.IsLoopback() && !allowLoopback {
		return fmt.Errorf("%w: %s", ErrBlockedAddress, ip)
	}
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return fmt.Errorf("%w: %s", ErrBlockedAddress, ip)
	}
	// IsPrivate covers RFC1918 (10/8, 172.16/12, 192.168/16) and IPv6 ULA (fc00::/7).
	if ip.IsPrivate() {
		return fmt.Errorf("%w: %s", ErrBlockedAddress, ip)
	}
	if ip.IsMulticast() {
		return fmt.Errorf("%w: %s", ErrBlockedAddress, ip)
	}
	if ip.IsUnspecified() {
		return fmt.Errorf("%w: %s", ErrBlockedAddress, ip)
	}
	return nil
}
