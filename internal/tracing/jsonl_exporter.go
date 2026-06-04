package tracing

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// JSONLExporter writes one ended span per JSON line.
type JSONLExporter struct {
	mu   sync.Mutex
	file *os.File
	enc  *json.Encoder
}

type spanRecord struct {
	SchemaVersion     int            `json:"schema_version"`
	TraceID           string         `json:"trace_id"`
	SpanID            string         `json:"span_id"`
	ParentSpanID      string         `json:"parent_span_id,omitempty"`
	Name              string         `json:"name"`
	Kind              string         `json:"kind"`
	StartUnixNano     int64          `json:"start_unix_nano"`
	EndUnixNano       int64          `json:"end_unix_nano"`
	DurationNano      int64          `json:"duration_nano"`
	StatusCode        string         `json:"status_code"`
	StatusDescription string         `json:"status_description,omitempty"`
	ServiceName       string         `json:"service_name,omitempty"`
	Attributes        map[string]any `json:"attributes,omitempty"`
	Events            []eventRecord  `json:"events,omitempty"`
}

type eventRecord struct {
	Name         string         `json:"name"`
	TimeUnixNano int64          `json:"time_unix_nano"`
	Attributes   map[string]any `json:"attributes,omitempty"`
}

// NewJSONLExporter opens path for append-only JSONL span export.
func NewJSONLExporter(path string) (*JSONLExporter, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("tracing: create jsonl exporter parent directory: %w", err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, fmt.Errorf("tracing: open jsonl exporter: %w", err)
	}
	return &JSONLExporter{
		file: file,
		enc:  json.NewEncoder(file),
	}, nil
}

// ExportSpans exports ended spans as flattened JSONL records.
func (e *JSONLExporter) ExportSpans(_ context.Context, spans []sdktrace.ReadOnlySpan) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, span := range spans {
		if err := e.enc.Encode(newSpanRecord(span)); err != nil {
			return fmt.Errorf("tracing: write jsonl span: %w", err)
		}
	}
	return nil
}

// Shutdown flushes and closes the exporter file.
func (e *JSONLExporter) Shutdown(_ context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.file == nil {
		return nil
	}
	if err := e.file.Sync(); err != nil {
		return fmt.Errorf("tracing: sync jsonl exporter: %w", err)
	}
	if err := e.file.Close(); err != nil {
		return fmt.Errorf("tracing: close jsonl exporter: %w", err)
	}
	e.file = nil
	return nil
}

func newSpanRecord(span sdktrace.ReadOnlySpan) spanRecord {
	parentSpanID := ""
	if span.Parent().SpanID().IsValid() {
		parentSpanID = span.Parent().SpanID().String()
	}
	start := span.StartTime()
	end := span.EndTime()
	status := span.Status()
	return spanRecord{
		SchemaVersion:     1,
		TraceID:           span.SpanContext().TraceID().String(),
		SpanID:            span.SpanContext().SpanID().String(),
		ParentSpanID:      parentSpanID,
		Name:              span.Name(),
		Kind:              spanKind(span.SpanKind()),
		StartUnixNano:     unixNano(start),
		EndUnixNano:       unixNano(end),
		DurationNano:      end.Sub(start).Nanoseconds(),
		StatusCode:        statusCode(status.Code),
		StatusDescription: status.Description,
		ServiceName:       serviceName(span),
		Attributes:        attributes(span.Attributes()),
		Events:            events(span.Events()),
	}
}

func unixNano(t time.Time) int64 {
	return t.UTC().UnixNano()
}

func spanKind(kind trace.SpanKind) string {
	return strings.ToLower(strings.TrimPrefix(kind.String(), "SpanKind"))
}

func statusCode(code codes.Code) string {
	switch code {
	case codes.Unset:
		return "unset"
	case codes.Ok:
		return "ok"
	case codes.Error:
		return "error"
	default:
		return strings.ToLower(code.String())
	}
}

func serviceName(span sdktrace.ReadOnlySpan) string {
	for _, attr := range span.Resource().Attributes() {
		if string(attr.Key) == "service.name" {
			return attr.Value.AsString()
		}
	}
	return ""
}

func events(spanEvents []sdktrace.Event) []eventRecord {
	if len(spanEvents) == 0 {
		return nil
	}
	records := make([]eventRecord, 0, len(spanEvents))
	for _, event := range spanEvents {
		records = append(records, eventRecord{
			Name:         event.Name,
			TimeUnixNano: unixNano(event.Time),
			Attributes:   attributes(event.Attributes),
		})
	}
	return records
}

func attributes(attrs []attribute.KeyValue) map[string]any {
	if len(attrs) == 0 {
		return nil
	}
	out := make(map[string]any, len(attrs))
	for _, attr := range attrs {
		key := string(attr.Key)
		if isRawContentAttribute(key) {
			continue
		}
		out[key] = attr.Value.AsInterface()
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func isRawContentAttribute(key string) bool {
	switch strings.ToLower(key) {
	case "prompt",
		"response",
		"completion",
		"llm.prompt",
		"llm.response",
		"llm.completion",
		"gen_ai.prompt",
		"gen_ai.completion",
		"tool.arguments",
		"tool.output",
		"tool.input",
		"tool.result":
		return true
	default:
		return false
	}
}
