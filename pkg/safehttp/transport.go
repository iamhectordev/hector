package safehttp

import (
	"errors"
	"io"
	"net/http"
)

// safeTransport wraps an inner RoundTripper to enforce scheme checks and cap
// response body size.
type safeTransport struct {
	inner        http.RoundTripper
	maxBodyBytes int64
}

func (t *safeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := checkScheme(req.URL.Scheme); err != nil {
		return nil, err
	}
	resp, err := t.inner.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errors.New("safehttp: nil response from inner transport")
	}
	if resp.Body == nil {
		resp.Body = http.NoBody
	}
	resp.Body = &limitedBody{r: resp.Body, limit: t.maxBodyBytes}
	return resp, nil
}

// limitedBody wraps a ReadCloser and returns [ErrOversize] once more than limit
// bytes have been read.
type limitedBody struct {
	r     io.ReadCloser
	n     int64
	limit int64
}

func (b *limitedBody) Read(p []byte) (int, error) {
	n, err := b.r.Read(p)
	b.n += int64(n)
	if b.n > b.limit {
		return n, ErrOversize
	}
	return n, err
}

func (b *limitedBody) Close() error { return b.r.Close() }
