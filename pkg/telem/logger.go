package telem

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/trace"
)

type loggerKey struct{}

// WithLogger stores logger in ctx for [Logger].
func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	if logger == nil {
		return ctx
	}
	return context.WithValue(ctx, loggerKey{}, logger)
}

// Logger returns a context-aware logger that accepts telem fields.
func Logger(ctx context.Context) ContextLogger {
	logger, ok := ctx.Value(loggerKey{}).(*slog.Logger)
	if !ok || logger == nil {
		logger = slog.Default()
	}
	return ContextLogger{logger: logger.With(contextAttrs(ctx)...)}
}

// ContextLogger wraps slog with telem fields.
type ContextLogger struct {
	logger *slog.Logger
}

// InfoContext logs at info level.
func (l ContextLogger) InfoContext(ctx context.Context, msg string, fields ...Field) {
	l.logger.LogAttrs(ctx, slog.LevelInfo, msg, slogAttrs(fields)...)
}

// DebugContext logs at debug level.
func (l ContextLogger) DebugContext(ctx context.Context, msg string, fields ...Field) {
	l.logger.LogAttrs(ctx, slog.LevelDebug, msg, slogAttrs(fields)...)
}

// WarnContext logs at warn level.
func (l ContextLogger) WarnContext(ctx context.Context, msg string, fields ...Field) {
	l.logger.LogAttrs(ctx, slog.LevelWarn, msg, slogAttrs(fields)...)
}

// ErrorContext logs at error level.
func (l ContextLogger) ErrorContext(ctx context.Context, msg string, fields ...Field) {
	l.logger.LogAttrs(ctx, slog.LevelError, msg, slogAttrs(fields)...)
}

func contextAttrs(ctx context.Context) []any {
	spanCtx := trace.SpanContextFromContext(ctx)
	fields := make([]any, 0, 2+len(baggageFromContext(ctx).Members()))
	if spanCtx.IsValid() {
		fields = append(fields,
			"trace_id", spanCtx.TraceID().String(),
			"span_id", spanCtx.SpanID().String(),
		)
	}
	for _, member := range baggageFromContext(ctx).Members() {
		fields = append(fields, member.Key(), member.Value())
	}
	return fields
}
