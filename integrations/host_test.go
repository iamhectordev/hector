package integrations_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/iamhectordev/hector/integrations"
	"github.com/iamhectordev/hector/pkg/tools"
	"github.com/stretchr/testify/require"
)

// toolsOnly is an Integration that only provides tools.
type toolsOnly struct {
	name  string
	tools []tools.Tool
}

func (f *toolsOnly) Name() string              { return f.name }
func (f *toolsOnly) Tools() []tools.Tool        { return f.tools }

// withInit adds the Initializer facet.
type withInit struct {
	toolsOnly
	fn func(ctx context.Context) error
}

func (f *withInit) Init(ctx context.Context) error { return f.fn(ctx) }

// withRun adds the EventSource facet.
type withRun struct {
	toolsOnly
	fn func(ctx context.Context) error
}

func (f *withRun) Run(ctx context.Context) error { return f.fn(ctx) }

// withClose adds the io.Closer facet and tracks close count.
type withClose struct {
	toolsOnly
	fn    func() error
	count int
}

func (f *withClose) Close() error {
	f.count++
	if f.fn != nil {
		return f.fn()
	}
	return nil
}

func TestHostToolsOnly(t *testing.T) {
	tool := &stubTool{name: "test_tool"}
	fi := &toolsOnly{name: "stub", tools: []tools.Tool{tool}}

	h, err := integrations.NewHost(fi)
	require.NoError(t, err)
	require.Equal(t, "integration.stub", h.Name())

	require.NoError(t, h.Init(t.Context()))

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- h.Start(ctx) }()
	cancel()
	require.NoError(t, <-done)

	require.NoError(t, h.Stop(t.Context()))
}

func TestHostInitializerFacet(t *testing.T) {
	t.Run("delegates init", func(t *testing.T) {
		var called bool
		fi := &withInit{
			toolsOnly: toolsOnly{name: "authed"},
			fn:        func(_ context.Context) error { called = true; return nil },
		}
		h, err := integrations.NewHost(fi)
		require.NoError(t, err)
		require.NoError(t, h.Init(t.Context()))
		require.True(t, called)
	})

	t.Run("propagates init error", func(t *testing.T) {
		initErr := errors.New("auth failed")
		fi := &withInit{
			toolsOnly: toolsOnly{name: "bad-auth"},
			fn:        func(_ context.Context) error { return initErr },
		}
		h, err := integrations.NewHost(fi)
		require.NoError(t, err)
		require.ErrorIs(t, h.Init(t.Context()), initErr)
	})
}

func TestHostEventSource(t *testing.T) {
	t.Run("ctx cancel returns nil", func(t *testing.T) {
		fi := &withRun{
			toolsOnly: toolsOnly{name: "events"},
			fn:        func(ctx context.Context) error { return ctx.Err() },
		}
		h, err := integrations.NewHost(fi)
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan error, 1)
		go func() { done <- h.Start(ctx) }()
		cancel()
		require.NoError(t, <-done)
	})

	t.Run("non-context error propagates", func(t *testing.T) {
		fatal := errors.New("socket dead")
		fi := &withRun{
			toolsOnly: toolsOnly{name: "events"},
			fn:        func(_ context.Context) error { return fatal },
		}
		h, err := integrations.NewHost(fi)
		require.NoError(t, err)
		require.ErrorIs(t, h.Start(t.Context()), fatal)
	})
}

func TestHostCloser(t *testing.T) {
	t.Run("stop calls close once", func(t *testing.T) {
		fi := &withClose{toolsOnly: toolsOnly{name: "closer"}}
		h, err := integrations.NewHost(fi)
		require.NoError(t, err)
		require.NoError(t, h.Stop(t.Context()))
		require.Equal(t, 1, fi.count)
	})

	t.Run("close error propagates", func(t *testing.T) {
		closeErr := errors.New("drain failed")
		fi := &withClose{
			toolsOnly: toolsOnly{name: "closer"},
			fn:        func() error { return closeErr },
		}
		h, err := integrations.NewHost(fi)
		require.NoError(t, err)
		require.ErrorIs(t, h.Stop(t.Context()), closeErr)
	})
}

func TestNewHostValidation(t *testing.T) {
	t.Run("nil integration", func(t *testing.T) {
		_, err := integrations.NewHost(nil)
		require.Error(t, err)
	})

	t.Run("empty name", func(t *testing.T) {
		_, err := integrations.NewHost(&toolsOnly{name: ""})
		require.Error(t, err)
	})
}

type stubTool struct {
	name string
}

func (s *stubTool) Definition() tools.Definition {
	return tools.Definition{
		Name:        s.name,
		Description: "stub tool",
		Parameters:  json.RawMessage(`{}`),
	}
}

func (s *stubTool) Run(_ context.Context, _ json.RawMessage) (string, error) {
	return "", nil
}
