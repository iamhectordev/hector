package tracing

import (
	"path/filepath"
	"testing"

	"github.com/doron-cohen/klee/xdg"
	"github.com/stretchr/testify/require"
)

func TestNormalizeConfigDefaultsJSONLPathToXDGDataHome(t *testing.T) {
	cfg := normalizeConfig(Config{
		Exporter: ExporterConfig{
			Type: ExporterJSONL,
		},
	})

	want := filepath.Join(xdg.New(appName).DataHome(), defaultJSONLTracesName)
	require.Equal(t, want, cfg.Exporter.Path)
}

func TestNormalizeConfigKeepsExplicitJSONLPath(t *testing.T) {
	cfg := normalizeConfig(Config{
		Exporter: ExporterConfig{
			Type: ExporterJSONL,
			Path: " custom-traces.jsonl ",
		},
	})

	require.Equal(t, "custom-traces.jsonl", cfg.Exporter.Path)
}
