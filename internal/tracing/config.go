package tracing

// ExporterType selects the tracing exporter implementation.
type ExporterType string

const (
	ExporterNone  ExporterType = "none"
	ExporterJSONL ExporterType = "jsonl"
)

// Config controls OpenTelemetry tracing setup.
type Config struct {
	Enabled     bool           `yaml:"enabled" env:"TRACING_ENABLED" default:"false"`
	ServiceName string         `yaml:"service_name" env:"TRACING_SERVICE_NAME" default:"hector"`
	SampleRatio float64        `yaml:"sample_ratio" env:"TRACING_SAMPLE_RATIO" default:"1" validate:"gte=0,lte=1"`
	Exporter    ExporterConfig `yaml:"exporter"`
}

// ExporterConfig controls where traces are exported.
type ExporterConfig struct {
	Type ExporterType `yaml:"type" env:"TRACING_EXPORTER_TYPE" default:"none" validate:"omitempty,oneof=none jsonl"`
	Path string       `yaml:"path" env:"TRACING_EXPORTER_PATH"`
}
