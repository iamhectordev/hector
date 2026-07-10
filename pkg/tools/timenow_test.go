package tools_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	pkgtools "github.com/iamhectordev/hector/pkg/tools"
)

func TestTimeNowDefinition(t *testing.T) {
	tool, err := pkgtools.NewTimeNow()
	require.NoError(t, err)

	def := tool.Definition()
	require.Equal(t, "time_now", def.Name)
	require.NotEmpty(t, def.Description)
	require.NotEmpty(t, def.Parameters)
}

func TestTimeNowRun(t *testing.T) {
	tool, err := pkgtools.NewTimeNow()
	require.NoError(t, err)

	output, err := tool.Run(t.Context(), nil)
	require.NoError(t, err)

	var env map[string]any
	require.NoError(t, json.Unmarshal([]byte(output), &env))
	require.Equal(t, "ok", env["status"])
	result, ok := env["result"].(string)
	require.True(t, ok)
	require.Contains(t, result, "UTC")
}
