package mock

import (
	"net/url"
	"testing"
	"time"
)

// Expectation captures a single incoming API call registered via Server.Expect.
type Expectation struct {
	ch chan url.Values
}

// Require blocks until the expected API call arrives or ctx is done, failing the
// test on timeout.
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

// AssertNotCalled asserts that no call arrives within d, failing the test if one does.
func (e *Expectation) AssertNotCalled(t *testing.T, d time.Duration) {
	t.Helper()
	select {
	case v := <-e.ch:
		t.Fatalf("slackmock: unexpected API call with params: %v", v)
	case <-time.After(d):
	}
}
