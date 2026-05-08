package processor_test

import (
	"bytes"
	"testing"

	"github.com/iamhectordev/hector/modules/agent/internal/processor"
	"github.com/stretchr/testify/require"
)

func TestProcessor_Handle_WritesLine(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	p := processor.New(&buf)

	require.NoError(t, p.Handle(t.Context(), "hello"))

	require.Equal(t, "hello\n", buf.String())
}
