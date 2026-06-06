package telem

import (
	"context"
	"reflect"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const tracerName = "github.com/iamhectordev/hector"

// Span wraps an OpenTelemetry span with Hector's end/error policy.
type Span struct {
	inner trace.Span
}

// Trace starts a span with baggage and operation fields attached.
func Trace(ctx context.Context, name string, fields ...Field) (context.Context, Span) {
	spanAttrs := append(baggageAttrs(ctx), attrs(fields)...)
	ctx, span := otel.Tracer(tracerName).Start(ctx, name, trace.WithAttributes(spanAttrs...))
	return ctx, Span{inner: span}
}

// End records err when present and ends the span.
func (s Span) End(err *error) {
	if err != nil && *err != nil {
		s.inner.RecordError(*err)
		s.inner.SetStatus(codes.Error, errorType(*err))
	}
	s.inner.End()
}

// AddFields adds attributes to an existing span.
func (s Span) AddFields(fields ...Field) {
	s.inner.SetAttributes(attrs(fields)...)
}

// SpanContext returns the wrapped span context.
func (s Span) SpanContext() trace.SpanContext {
	return s.inner.SpanContext()
}

// Event adds a named event to the current span.
func Event(ctx context.Context, name string, fields ...Field) {
	trace.SpanFromContext(ctx).AddEvent(name, trace.WithAttributes(attrs(fields)...))
}

func baggageAttrs(ctx context.Context) []attribute.KeyValue {
	bag := baggageFromContext(ctx)
	members := bag.Members()
	out := make([]attribute.KeyValue, 0, len(members))
	for _, member := range members {
		out = append(out, attribute.String(member.Key(), member.Value()))
	}
	return out
}

func errorType(err error) string {
	if err == nil {
		return ""
	}
	return reflect.TypeOf(err).String()
}
