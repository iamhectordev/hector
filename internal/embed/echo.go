package embed

import (
	"context"
	"crypto/sha256"
)

// EchoEmbedder returns a deterministic 3-dimensional vector derived from the
// text's SHA-256 hash. It makes no network calls and is intended for tests.
type EchoEmbedder struct{}

func (e *EchoEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	h := sha256.Sum256([]byte(text))
	return []float32{
		float32(h[0]) / 255.0,
		float32(h[1]) / 255.0,
		float32(h[2]) / 255.0,
	}, nil
}
