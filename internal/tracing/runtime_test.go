package tracing_test

import (
	"testing"

	"github.com/iamhectordev/hector/internal/tracing"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/propagation"
)

func TestSetupDisabled(t *testing.T) {
	runtime, err := tracing.Setup(t.Context(), tracing.Config{})
	require.NoError(t, err)
	require.False(t, runtime.Enabled())
	require.NoError(t, runtime.Shutdown(t.Context()))
}

func TestSetupRejectsInvalidSampleRatio(t *testing.T) {
	for _, sampleRatio := range []float64{-0.1, 1.1} {
		_, err := tracing.Setup(t.Context(), tracing.Config{
			Enabled:     true,
			ServiceName: "hector",
			SampleRatio: sampleRatio,
		})
		require.Error(t, err)
		require.ErrorContains(t, err, "sample_ratio")
	}
}

func TestSetupRejectsJSONLUntilExporterExists(t *testing.T) {
	_, err := tracing.Setup(t.Context(), tracing.Config{
		Enabled:     true,
		ServiceName: "hector",
		SampleRatio: 1,
		Exporter: tracing.ExporterConfig{
			Type: tracing.ExporterJSONL,
			Path: "traces.jsonl",
		},
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "jsonl exporter")
}

func TestSetupEnabledInstallsTraceContextAndBaggagePropagation(t *testing.T) {
	runtime, err := tracing.Setup(t.Context(), tracing.Config{
		Enabled:     true,
		ServiceName: "hector",
		SampleRatio: 1,
	})
	require.NoError(t, err)
	defer func() {
		require.NoError(t, runtime.Shutdown(t.Context()))
	}()
	require.True(t, runtime.Enabled())

	ctx, span := otel.Tracer("test").Start(t.Context(), "operation")
	defer span.End()

	bag, err := baggage.Parse("session.id=sess_123")
	require.NoError(t, err)
	ctx = baggage.ContextWithBaggage(ctx, bag)

	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)

	require.NotEmpty(t, carrier.Get("traceparent"))
	require.Contains(t, carrier.Get("baggage"), "session.id=sess_123")
}

func TestSetupEnabledAppliesSampleRatio(t *testing.T) {
	runtime, err := tracing.Setup(t.Context(), tracing.Config{
		Enabled:     true,
		ServiceName: "hector",
		SampleRatio: 0,
	})
	require.NoError(t, err)
	defer func() {
		require.NoError(t, runtime.Shutdown(t.Context()))
	}()

	_, span := otel.Tracer("test").Start(t.Context(), "operation")
	defer span.End()

	require.False(t, span.SpanContext().IsSampled())
}
