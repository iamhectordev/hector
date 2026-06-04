package memory

import "context"

// Object is a unit of stored knowledge.
type Object struct {
	ID      string
	Content string
}

// Store persists and retrieves memory objects.
type Store interface {
	Put(ctx context.Context, obj Object) error
	Search(ctx context.Context, query string, limit int) ([]Object, error)
}
