package supervisor

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
)

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
// It also returns a stop function that unregisters signal notifications and stops the internal listener.
// stop does not cancel the returned context.
func NotifyContext(parent context.Context, signals ...os.Signal) (context.Context, context.CancelFunc) {
	if len(signals) == 0 {
		signals = DefaultSignals()
	}

	ctx, cancelCause := context.WithCancelCause(parent)

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, signals...)
	stopCh := make(chan struct{})

	var once sync.Once
	stop := func() {
		once.Do(func() {
			signal.Stop(ch)
			close(stopCh)
		})
	}

	go func() {
		select {
		case <-stopCh:
			return
		case <-ctx.Done():
			return
		case sig := <-ch:
			cancelCause(SignalCause{Signal: sig})
			return
		}
	}()

	return ctx, stop
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
