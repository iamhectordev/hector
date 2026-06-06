package telem

import (
	"fmt"
	"log/slog"

	"go.opentelemetry.io/otel/attribute"
)

// Field is a typed observability field that can be used for traces, logs, and baggage.
type Field struct {
	key   string
	value any
	attr  attribute.KeyValue
}

// String returns a string field.
func String(key, value string) Field {
	return Field{key: key, value: value, attr: attribute.String(key, value)}
}

// Int returns an integer field.
func Int(key string, value int) Field {
	return Field{key: key, value: value, attr: attribute.Int(key, value)}
}

// Bool returns a boolean field.
func Bool(key string, value bool) Field {
	return Field{key: key, value: value, attr: attribute.Bool(key, value)}
}

func attrs(fields []Field) []attribute.KeyValue {
	out := make([]attribute.KeyValue, 0, len(fields))
	for _, field := range fields {
		out = append(out, field.attr)
	}
	return out
}

func slogAttrs(fields []Field) []slog.Attr {
	out := make([]slog.Attr, 0, len(fields))
	for _, field := range fields {
		out = append(out, slog.Any(field.key, field.value))
	}
	return out
}

func fieldString(field Field) string {
	if value, ok := field.value.(string); ok {
		return value
	}
	return fmt.Sprint(field.value)
}
