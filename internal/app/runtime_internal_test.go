package app

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/iamhectordev/hector/internal/tracing"
	"github.com/iamhectordev/hector/pkg/supervisor"
	"github.com/iamhectordev/hector/pkg/waffle"
	"github.com/stretchr/testify/require"
)

func TestRuntimeSupervisorOwnsTracingShutdownOnNormalStop(t *testing.T) {
	tracingRuntime, err := tracing.Setup(t.Context(), tracing.Config{
		Enabled:     true,
		ServiceName: "hector",
		SampleRatio: 1,
		Exporter: tracing.ExporterConfig{
			Type: tracing.ExporterJSONL,
			Path: filepath.Join(t.TempDir(), "traces.jsonl"),
		},
	})
	require.NoError(t, err)

	bus, err := waffle.NewEventBus()
	require.NoError(t, err)

	started := make(chan struct{}, 1)
	runtime := &Runtime{
		cfg:     &Config{},
		logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		tracing: tracingRuntime,
		bus:     bus,
	}
	require.NoError(t, runtime.initSupervisor(t.Context(), []supervisor.Module{
		blockingModule{started: started},
	}))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan supervisor.Report, 1)
	go func() {
		done <- runtime.sv.Run(ctx)
	}()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("module did not start")
	}

	cancel()
	var rep supervisor.Report
	select {
	case rep = <-done:
	case <-time.After(time.Second):
		t.Fatal("supervisor did not stop")
	}

	require.Equal(t, supervisor.ReasonContextCanceled, rep.Reason)
	require.Empty(t, rep.PostStopErrors)
	require.Nil(t, runtime.tracing)
	runtime.close(ctx)
}

type blockingModule struct {
	started chan<- struct{}
}

func (m blockingModule) Name() string { return "block" }

func (m blockingModule) Init(context.Context) error { return nil }

func (m blockingModule) Start(ctx context.Context) error {
	m.started <- struct{}{}
	<-ctx.Done()
	return nil
}

func (m blockingModule) Stop(context.Context) error { return nil }
