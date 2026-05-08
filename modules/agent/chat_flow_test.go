package agent_test

import (
	"context"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/iamhectordev/hector/modules/agent"
	"github.com/iamhectordev/hector/modules/agent/internal/processor"
	"github.com/iamhectordev/hector/modules/tui"
	"github.com/iamhectordev/hector/pkg/supervisor"
	"github.com/iamhectordev/hector/pkg/waffle"
	"github.com/stretchr/testify/require"
)

func TestChatEcho_LineFromTUI_PrintsViaAgent(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	buf := newSafeBuffer()
	bus, err := waffle.NewEventBus(waffle.WithWorkers(2))
	require.NoError(t, err)

	sv, err := supervisor.New([]supervisor.Module{
		agent.NewModule(bus, processor.New(buf)),
		tui.NewModule(bus, strings.NewReader("hello\n")),
	})
	require.NoError(t, err)

	done := make(chan supervisor.Report, 1)
	go func() {
		done <- sv.Run(ctx)
	}()

	require.NoError(t, waitForWrite(t.Context(), buf, 2*time.Second))

	cancel()

	rep := <-done
	require.ErrorIs(t, rep.Err(), context.Canceled)
	require.NoError(t, bus.Shutdown(context.Background()))
	require.Equal(t, "hello\n", buf.String())
}

type safeBuffer struct {
	mu      sync.Mutex
	b       strings.Builder
	written chan struct{}
}

func newSafeBuffer() *safeBuffer {
	return &safeBuffer{written: make(chan struct{}, 1)}
}

var _ io.Writer = (*safeBuffer)(nil)

func (s *safeBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n, err := s.b.Write(p)
	if n > 0 {
		select {
		case s.written <- struct{}{}:
		default:
		}
	}
	return n, err
}

func (s *safeBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

func waitForWrite(ctx context.Context, buf *safeBuffer, timeout time.Duration) error {
	select {
	case <-buf.written:
		return nil
	case <-time.After(timeout):
		return context.DeadlineExceeded
	case <-ctx.Done():
		return ctx.Err()
	}
}
