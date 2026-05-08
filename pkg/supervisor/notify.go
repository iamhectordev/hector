package supervisor

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
)

type notifyContextKey struct{}

// SignalCause is used as a context cancel cause when shutdown was triggered by an OS signal.
type SignalCause struct {
	Signal os.Signal
}

func (c SignalCause) Error() string {
	if c.Signal == nil {
		return "shutdown signal received"
	}
	return fmt.Sprintf("shutdown signal received: %s", c.Signal)
}

// DefaultSignals returns platform-appropriate shutdown signals.
func DefaultSignals() []os.Signal {
	return append([]os.Signal(nil), defaultSignals...)
}

// NotifyContext returns a child context canceled with [SignalCause] when a matching OS signal arrives.
// It also returns a stop function that unregisters signal notifications and cancels the child context.
func NotifyContext(parent context.Context, signals ...os.Signal) (context.Context, context.CancelFunc) {
	if len(signals) == 0 {
		signals = DefaultSignals()
	}

	base := context.WithValue(parent, notifyContextKey{}, true)
	ctx, cancelCause := context.WithCancelCause(base)

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, signals...)

	var once sync.Once
	stop := func() {
		once.Do(func() {
			signal.Stop(ch)
			cancelCause(nil)
		})
	}

	go func() {
		select {
		case <-ctx.Done():
			stop()
		case sig := <-ch:
			cancelCause(SignalCause{Signal: sig})
			stop()
		}
	}()

	return ctx, stop
}

func hasNotifyContext(ctx context.Context) bool {
	v, _ := ctx.Value(notifyContextKey{}).(bool)
	return v
}

func signalFromCause(err error) (os.Signal, bool) {
	if err == nil {
		return nil, false
	}
	if sc, ok := err.(SignalCause); ok {
		return sc.Signal, true
	}
	if sc, ok := err.(*SignalCause); ok {
		return sc.Signal, true
	}
	return nil, false
}
