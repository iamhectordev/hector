package tracing

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.38.0"
)

// Runtime owns process tracing lifecycle.
type Runtime struct {
	enabled  bool
	provider *sdktrace.TracerProvider
}

// Setup configures process tracing.
func Setup(_ context.Context, cfg Config) (*Runtime, error) {
	cfg = normalizeConfig(cfg)
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}

	otel.SetTextMapPropagator(textMapPropagator())
	if !cfg.Enabled {
		return &Runtime{}, nil
	}

	provider := sdktrace.NewTracerProvider(
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(cfg.ServiceName),
		)),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.SampleRatio))),
	)
	otel.SetTracerProvider(provider)
	return &Runtime{enabled: true, provider: provider}, nil
}

// Enabled reports whether tracing is active.
func (r *Runtime) Enabled() bool {
	if r == nil {
		return false
	}
	return r.enabled
}

// Shutdown flushes and releases tracing resources.
func (r *Runtime) Shutdown(ctx context.Context) error {
	if r == nil || r.provider == nil {
		return nil
	}
	return r.provider.Shutdown(ctx)
}

func textMapPropagator() propagation.TextMapPropagator {
	return propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
}

func normalizeConfig(cfg Config) Config {
	if cfg.ServiceName == "" {
		cfg.ServiceName = "hector"
	}
	if cfg.Exporter.Type == "" {
		cfg.Exporter.Type = ExporterNone
	}
	return cfg
}

func validateConfig(cfg Config) error {
	if cfg.SampleRatio < 0 || cfg.SampleRatio > 1 {
		return fmt.Errorf("tracing: sample_ratio must be between 0 and 1")
	}
	switch cfg.Exporter.Type {
	case ExporterNone:
		return nil
	case ExporterJSONL:
		return fmt.Errorf("tracing: jsonl exporter is not implemented")
	default:
		return fmt.Errorf("tracing: unsupported exporter type %q", cfg.Exporter.Type)
	}
}
