package llm_test

import (
	"errors"
	"testing"

	"github.com/iamhectordev/hector/pkg/llm"
	"github.com/stretchr/testify/require"
)

func TestIsRetryable(t *testing.T) {
	t.Parallel()

	err := &llm.Error{
		Provider: string(llm.BackendOpenAI),
		Kind:     llm.ErrorRateLimited,
		Retry:    true,
	}

	require.True(t, llm.IsRetryable(err))
	require.True(t, llm.IsRetryable(errors.Join(errors.New("wrapped"), err)))
	require.False(t, llm.IsRetryable(errors.New("plain")))
}
