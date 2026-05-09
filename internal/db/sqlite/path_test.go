package sqlite

import (
	"path/filepath"
	"testing"

	"github.com/doron-cohen/klee/xdg"
	"github.com/stretchr/testify/require"
)

func TestResolvePath_DefaultsToXDGDataHome(t *testing.T) {
	got := resolvePath(Config{})
	want := filepath.Join(xdg.New(appName).DataHome(), defaultDBName)
	require.Equal(t, want, got)
}

func TestResolvePath_UsesExplicitPath(t *testing.T) {
	require.Equal(t, "custom.db", resolvePath(Config{Path: "custom.db"}))
}
