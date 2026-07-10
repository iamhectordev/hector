package slack

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExpandPath(t *testing.T) {
	t.Parallel()

	home, err := os.UserHomeDir()
	require.NoError(t, err)

	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "expands tilde prefix", path: "~/foo/bar", want: filepath.Join(home, "foo/bar")},
		{name: "passes through absolute path", path: "/var/log/hector/events.jsonl", want: "/var/log/hector/events.jsonl"},
		{name: "passes through relative path", path: "rel/path.jsonl", want: "rel/path.jsonl"},
		{name: "empty string", path: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := expandPath(tt.path)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestNewFileEventLogger_CreatesParentDirs(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "a", "b", "c", "events.jsonl")

	logger, err := NewFileEventLogger(EventLogConfig{Enabled: true, Path: path})
	require.NoError(t, err)
	t.Cleanup(func() { _ = logger.Close() })

	_, err = os.Stat(path)
	require.NoError(t, err, "file should exist at nested path")
}

func TestNewFileEventLogger_ExpandsTilde(t *testing.T) {
	t.Parallel()

	home, err := os.UserHomeDir()
	require.NoError(t, err)

	path := "~/slack-events-test-" + strings.ReplaceAll(t.Name(), "/", "_") + ".jsonl"

	logger, err := NewFileEventLogger(EventLogConfig{Enabled: true, Path: path})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = logger.Close()
		_ = os.Remove(filepath.Join(home, filepath.Base(path)))
	})

	full := filepath.Join(home, filepath.Base(path))
	_, err = os.Stat(full)
	require.NoError(t, err, "file should exist at expanded path")

	data, err := os.ReadFile(full)
	require.NoError(t, err)
	require.Empty(t, data, "new file should be empty")
}
