package session

import "context"

// Session carries per-request attributes through the call chain.
type Session struct {
	ID        string
	SourceURI string
}

type contextKey struct{}

// With returns a copy of ctx carrying s.
func With(ctx context.Context, s Session) context.Context {
	return context.WithValue(ctx, contextKey{}, s)
}

// From returns the Session stored in ctx and whether one was present.
func From(ctx context.Context) (Session, bool) {
	s, ok := ctx.Value(contextKey{}).(Session)
	return s, ok
}
