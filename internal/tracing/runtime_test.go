package tracing_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/iamhectordev/hector/internal/tracing"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
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

func TestSetupExportsJSONLSpanRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "traces.jsonl")
	runtime, err := tracing.Setup(t.Context(), tracing.Config{
		Enabled:     true,
		ServiceName: "hector",
		SampleRatio: 1,
		Exporter: tracing.ExporterConfig{
			Type: tracing.ExporterJSONL,
			Path: path,
		},
	})
	require.NoError(t, err)

	ctx, span := otel.Tracer("test").Start(t.Context(), "operation",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(attribute.String("module", "test")),
	)
	span.AddEvent("retry", trace.WithAttributes(attribute.Int("attempt", 2)))
	span.SetStatus(codes.Error, "failed")
	span.End()

	require.NoError(t, runtime.Shutdown(ctx))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	lines := requireJSONLLines(t, data)
	require.Len(t, lines, 1)

	require.Equal(t, 1, int(lines[0]["schema_version"].(float64)))
	require.NotEmpty(t, lines[0]["trace_id"])
	require.NotEmpty(t, lines[0]["span_id"])
	require.Equal(t, "operation", lines[0]["name"])
	require.Equal(t, "client", lines[0]["kind"])
	require.NotZero(t, lines[0]["start_unix_nano"])
	require.NotZero(t, lines[0]["end_unix_nano"])
	require.NotZero(t, lines[0]["duration_nano"])
	require.Equal(t, "error", lines[0]["status_code"])
	require.Equal(t, "failed", lines[0]["status_description"])
	require.Equal(t, "hector", lines[0]["service_name"])
	require.Equal(t, "test", lines[0]["attributes"].(map[string]any)["module"])
	require.Equal(t, "retry", lines[0]["events"].([]any)[0].(map[string]any)["name"])
}

func TestSetupExportsTraceTreeIDs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "traces.jsonl")
	runtime, err := tracing.Setup(t.Context(), tracing.Config{
		Enabled:     true,
		ServiceName: "hector",
		SampleRatio: 1,
		Exporter: tracing.ExporterConfig{
			Type: tracing.ExporterJSONL,
			Path: path,
		},
	})
	require.NoError(t, err)

	ctx, parent := otel.Tracer("test").Start(t.Context(), "parent")
	_, child := otel.Tracer("test").Start(ctx, "child")
	child.End()
	parent.End()
	require.NoError(t, runtime.Shutdown(t.Context()))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	lines := requireJSONLLines(t, data)
	require.Len(t, lines, 2)

	byName := map[string]map[string]any{}
	for _, line := range lines {
		byName[line["name"].(string)] = line
	}
	parentRecord := byName["parent"]
	childRecord := byName["child"]
	require.NotEmpty(t, parentRecord["trace_id"])
	require.Equal(t, parentRecord["trace_id"], childRecord["trace_id"])
	require.Equal(t, parentRecord["span_id"], childRecord["parent_span_id"])
	require.NotContains(t, childRecord, "event_names")
}

func TestSetupJSONLExporterDropsRawContentAttributesByDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "traces.jsonl")
	runtime, err := tracing.Setup(t.Context(), tracing.Config{
		Enabled:     true,
		ServiceName: "hector",
		SampleRatio: 1,
		Exporter: tracing.ExporterConfig{
			Type: tracing.ExporterJSONL,
			Path: path,
		},
	})
	require.NoError(t, err)

	_, span := otel.Tracer("test").Start(t.Context(), "llm.complete",
		trace.WithAttributes(
			attribute.String("llm.prompt", "raw user prompt"),
			attribute.String("llm.response", "raw model response"),
			attribute.String("tool.arguments", `{"secret":"raw"}`),
			attribute.String("tool.output", "raw tool output"),
			attribute.String("llm.model", "test-model"),
		),
	)
	span.AddEvent("tool.done",
		trace.WithAttributes(
			attribute.String("tool.output", "raw event output"),
			attribute.Int("result.count", 1),
		),
	)
	span.End()
	require.NoError(t, runtime.Shutdown(t.Context()))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	lines := requireJSONLLines(t, data)
	require.Len(t, lines, 1)

	attrs := lines[0]["attributes"].(map[string]any)
	require.NotContains(t, attrs, "llm.prompt")
	require.NotContains(t, attrs, "llm.response")
	require.NotContains(t, attrs, "tool.arguments")
	require.NotContains(t, attrs, "tool.output")
	require.Equal(t, "test-model", attrs["llm.model"])

	eventAttrs := lines[0]["events"].([]any)[0].(map[string]any)["attributes"].(map[string]any)
	require.NotContains(t, eventAttrs, "tool.output")
	require.Equal(t, float64(1), eventAttrs["result.count"])
}

func requireJSONLLines(t *testing.T, data []byte) []map[string]any {
	t.Helper()
	rawLines := bytes.Split(bytes.TrimSpace(data), []byte("\n"))
	records := make([]map[string]any, 0, len(rawLines))
	for _, raw := range rawLines {
		var record map[string]any
		require.NoError(t, json.Unmarshal(raw, &record))
		records = append(records, record)
	}
	return records
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
