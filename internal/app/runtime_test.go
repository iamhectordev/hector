package app_test

import (
	"io"
	"log/slog"
	"testing"

	"github.com/iamhectordev/hector/internal/app"
	"github.com/iamhectordev/hector/internal/tracing"
	"github.com/stretchr/testify/require"
)

func TestRuntimeStartsTracingBeforeAppInit(t *testing.T) {
	t.Parallel()

	runtime, err := app.NewRuntime(app.Config{
		Tracing: tracing.Config{
			Enabled:     true,
			ServiceName: "hector",
			SampleRatio: -1,
		},
	}, app.WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))))
	require.NoError(t, err)

	err = runtime.Start(t.Context())
	require.Error(t, err)
	require.ErrorContains(t, err, "sample_ratio")
}
