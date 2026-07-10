package mock

import (
	"net/url"
	"testing"
	"time"
)

type Expectation struct {
	ch chan url.Values
}

func (e *Expectation) Require(t *testing.T, ctx interface{ Done() <-chan struct{} }) url.Values {
	t.Helper()
	select {
	case v := <-e.ch:
		return v
	case <-ctx.Done():
		t.Fatal("slackmock: timed out waiting for expected API call")
		return nil
	}
}

func (e *Expectation) AssertNotCalled(t *testing.T, d time.Duration) {
	t.Helper()
	select {
	case v := <-e.ch:
		t.Fatalf("slackmock: unexpected API call with params: %v", v)
	case <-time.After(d):
	}
}
