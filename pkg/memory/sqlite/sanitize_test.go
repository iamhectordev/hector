package sqlite

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSanitizeFTSQuery_QuotesTokensWithPrefix(t *testing.T) {
	require.Equal(t, `"hello"* OR "world"*`, sanitizeFTSQuery("hello world"))
}

func TestSanitizeFTSQuery_TreatsKeywordsAsLiterals(t *testing.T) {
	require.Equal(t, `"AND"* OR "OR"* OR "NOT"*`, sanitizeFTSQuery("AND OR NOT"))
}

func TestSanitizeFTSQuery_StripsColumnFilterSyntax(t *testing.T) {
	require.Equal(t, `"content"* OR "attack"*`, sanitizeFTSQuery("content:attack"))
}

func TestSanitizeFTSQuery_EmptyInputReturnsEmpty(t *testing.T) {
	require.Equal(t, "", sanitizeFTSQuery(""))
}

func TestSanitizeFTSQuery_SingleToken(t *testing.T) {
	require.Equal(t, `"deploy"*`, sanitizeFTSQuery("deploy"))
}
