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
	require.NotNil(t, def.InputSchema)
	require.Equal(t, "object", def.InputSchema.Type)
	require.Empty(t, def.InputSchema.Properties)
	require.NotNil(t, def.InputSchema.AdditionalProperties)
}

func TestTimeNowRun(t *testing.T) {
	output, err := tools.TimeNow{}.Run(t.Context(), nil)
	require.NoError(t, err)

	require.Contains(t, output, "UTC")
	require.Len(t, strings.SplitN(output, ",", 2), 2)
}
