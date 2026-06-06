package telem_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"

	"github.com/iamhectordev/hector/pkg/telem"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestTraceAddsFieldsBaggageAndErrorStatus(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		require.NoError(t, provider.Shutdown(context.Background()))
	})

	ctx := telem.WithBaggage(t.Context(), telem.String("session.id", "sess_123"))
	_, span := telem.Trace(ctx, "agent.turn.run", telem.Int("agent.message_count", 2))
	err := errors.New("boom")
	span.End(&err)

	spans := recorder.Ended()
	require.Len(t, spans, 1)
	require.Equal(t, "agent.turn.run", spans[0].Name())
	require.Equal(t, "sess_123", spans[0].Attributes()[0].Value.AsString())
	require.Equal(t, int64(2), spans[0].Attributes()[1].Value.AsInt64())
	require.Equal(t, "Error", spans[0].Status().Code.String())
}

func TestLoggerAddsTraceIDsBaggageAndFields(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		require.NoError(t, provider.Shutdown(context.Background()))
	})

	ctx := telem.WithLogger(t.Context(), logger)
	ctx = telem.WithBaggage(ctx, telem.String("session.id", "sess_123"))
	ctx, span := telem.Trace(ctx, "slack.message.receive")
	defer span.End(nil)

	telem.Logger(ctx).InfoContext(ctx, "message received", telem.String("surface.name", "slack"))

	var got map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
	require.Equal(t, "message received", got["msg"])
	require.Equal(t, "sess_123", got["session.id"])
	require.Equal(t, "slack", got["surface.name"])
	require.NotEmpty(t, got["trace_id"])
	require.NotEmpty(t, got["span_id"])
}

func TestEventAddsSpanEvent(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		require.NoError(t, provider.Shutdown(context.Background()))
	})

	ctx, span := telem.Trace(t.Context(), "tool.call")
	telem.Event(ctx, "retry", telem.Int("attempt", 2))
	span.End(nil)

	spans := recorder.Ended()
	require.Len(t, spans, 1)
	require.Len(t, spans[0].Events(), 1)
	require.Equal(t, "retry", spans[0].Events()[0].Name)
	require.Equal(t, int64(2), spans[0].Events()[0].Attributes[0].Value.AsInt64())
}
