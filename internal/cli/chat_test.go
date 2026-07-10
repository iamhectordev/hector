package cli_test

import (
	"context"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/iamhectordev/hector/modules/agent"
	"github.com/iamhectordev/hector/modules/tui"
	"github.com/iamhectordev/hector/pkg/tools"
	"github.com/iamhectordev/hector/pkg/comms"
	"github.com/iamhectordev/hector/pkg/llm/schema"
	llmtest "github.com/iamhectordev/hector/pkg/llm/testing"
	"github.com/iamhectordev/hector/pkg/session"
	"github.com/iamhectordev/hector/pkg/supervisor"
	"github.com/iamhectordev/hector/pkg/waffle"
	"github.com/stretchr/testify/require"
)

func TestChat_LineFromTUI_RepliesViaReplyTool(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	buf := newSafeBuffer()
	bus, err := waffle.NewEventBus(waffle.WithWorkers(2))
	require.NoError(t, err)

	registry := newTUIReplyRegistry(t, buf)
	sv, err := supervisor.New([]supervisor.Module{
		agent.NewModule(bus, agent.NewLoop(
			llmtest.NewCompleter(t,
				llmtest.ToolCalls(llmtest.Call("c1", "reply", `{"text":"hello back"}`)),
				llmtest.Stop(""),
			),
			agent.WithTools(registry),
		),
			agent.WithSessionStore(noopSessionStore{}),
		),
		tui.NewModule(bus, tui.WithReader(strings.NewReader("hello\n"))),
	}, supervisor.WithPostInitHook("bus.start", bus.Start))
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
	require.Equal(t, "hello back\n", buf.String())
}

func TestChat_LineFromTUI_DoesNotPrintPlainAssistantStop(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	buf := newSafeBuffer()
	bus, err := waffle.NewEventBus(waffle.WithWorkers(2))
	require.NoError(t, err)

	registry := newTUIReplyRegistry(t, buf)
	completer := &plainStopCompleter{done: make(chan struct{})}
	sv, err := supervisor.New([]supervisor.Module{
		agent.NewModule(bus, agent.NewLoop(
			completer,
			agent.WithTools(registry),
		),
			agent.WithSessionStore(noopSessionStore{}),
		),
		tui.NewModule(bus, tui.WithReader(strings.NewReader("hello\n"))),
	}, supervisor.WithPostInitHook("bus.start", bus.Start))
	require.NoError(t, err)

	done := make(chan supervisor.Report, 1)
	go func() {
		done <- sv.Run(ctx)
	}()

	require.NoError(t, waitForSignal(t.Context(), completer.done, 2*time.Second))
	require.Never(t, func() bool {
		return waitForWrite(t.Context(), buf, 10*time.Millisecond) == nil
	}, 100*time.Millisecond, 10*time.Millisecond)

	cancel()

	rep := <-done
	require.ErrorIs(t, rep.Err(), context.Canceled)
	require.NoError(t, bus.Shutdown(context.Background()))
	require.Empty(t, buf.String())
}

func newTUIReplyRegistry(t *testing.T, out io.Writer) *tools.Registry {
	t.Helper()

	replyRouter, err := comms.NewReplyRouter(tui.NewReplyHandler(out))
	require.NoError(t, err)
	registry, err := tools.NewRegistry(replyRouter)
	require.NoError(t, err)
	return registry
}

type plainStopCompleter struct {
	done chan struct{}
}

func (c *plainStopCompleter) Complete(context.Context, schema.CompletionRequest) (*schema.Message, error) {
	reply := schema.AssistantMessage("plain assistant text")
	reply.FinishReason = schema.FinishReasonStop
	close(c.done)
	return reply, nil
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
	return waitForSignal(ctx, buf.written, timeout)
}

func waitForSignal(ctx context.Context, ch <-chan struct{}, timeout time.Duration) error {
	select {
	case <-ch:
		return nil
	case <-time.After(timeout):
		return context.DeadlineExceeded
	case <-ctx.Done():
		return ctx.Err()
	}
}

type noopSessionStore struct{}

func (noopSessionStore) GetOrCreate(_ context.Context, sourceURI string) (session.StoredSession, error) {
	return session.StoredSession{SourceURI: sourceURI}, nil
}

func (noopSessionStore) Messages(context.Context, string) ([]*schema.Message, error) {
	return nil, nil
}

func (noopSessionStore) Record(context.Context, string, []*schema.Message) error {
	return nil
}
