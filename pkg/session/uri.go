package session

import (
	"fmt"
	"net/url"
)

// NewSourceURI constructs a source URI from its parts.
// Example: NewSourceURI("slack", "channels", "D1234ABC") → "slack://channels/D1234ABC"
func NewSourceURI(scheme, host, path string) string {
	return (&url.URL{Scheme: scheme, Host: host, Path: "/" + path}).String()
}

// ParseSourceURI parses a source URI, requiring a non-empty scheme.
func ParseSourceURI(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	if u.Scheme == "" {
		return nil, fmt.Errorf("session: source URI %q has no scheme", raw)
	}
	return u, nil
}
