package agent_test

import (
	"testing"

	"github.com/iamhectordev/hector/modules/agent"
	"github.com/stretchr/testify/require"
)

func TestPrompt_Render(t *testing.T) {
	t.Parallel()

	p := agent.NewPrompt(
		agent.TextPart("Hello world."),
		agent.TextPart("Second line."),
	)
	got, err := p.Render()
	require.NoError(t, err)
	require.Equal(t, "Hello world.\n\nSecond line.", got)
}

func TestXMLPart_Render(t *testing.T) {
	t.Parallel()

	type mockData struct {
		ID   string `xml:"id,attr"`
		Name string `xml:"name"`
	}

	part := agent.NewXMLPart("test_root", mockData{ID: "123", Name: "Alice"})
	got, err := part.Render()
	require.NoError(t, err)

	want := `<test_root id="123">
  <name>Alice</name>
</test_root>`
	require.Equal(t, want, got)
}

func TestPrompt_WithXMLPart(t *testing.T) {
	t.Parallel()

	type ctxData struct {
		Platform string `xml:"platform,attr"`
	}

	p := agent.NewPrompt(
		agent.TextPart("System msg."),
		agent.NewXMLPart("conversation", ctxData{Platform: "slack"}),
	)
	got, err := p.Render()
	require.NoError(t, err)

	want := `System msg.

<conversation platform="slack"></conversation>`
	require.Equal(t, want, got)
}
