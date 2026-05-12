package tools_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iamhectordev/hector/modules/tools"
)

func TestTimeNowDefinition(t *testing.T) {
	def := tools.TimeNow{}.Definition()

	require.Equal(t, "time.now", def.Name)
	require.NotEmpty(t, def.Description)
	require.NotEmpty(t, def.Parameters)
}

func TestTimeNowRun(t *testing.T) {
	output, err := tools.TimeNow{}.Run(t.Context(), nil)
	require.NoError(t, err)

	require.Contains(t, output, "UTC")
	require.Len(t, strings.SplitN(output, ",", 2), 2)
}
