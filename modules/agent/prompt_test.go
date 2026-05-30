package agent_test

import (
	"testing"

	"github.com/iamhectordev/hector/modules/agent"
	"github.com/stretchr/testify/require"
)

func TestXMLPart_RenderForward(t *testing.T) {
	t.Parallel()

	fwd := agent.MessageForward{
		Message: agent.UserMessage{
			SenderID:   "U789",
			SenderName: "Bob",
			TS:         "1747123400.000050",
			Text:       "Original message",
		},
	}
	got, err := agent.NewXMLPart("fwd", fwd).Render()
	require.NoError(t, err)

	want := `<fwd>
  <msg sender_id="U789" sender_name="Bob" ts="1747123400.000050">
    <text>Original message</text>
  </msg>
</fwd>`
	require.Equal(t, want, got)
}

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

func TestXMLPart_RenderMessageFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		file agent.MessageFile
		want string
	}{
		{
			name: "text content stays readable",
			file: agent.MessageFile{
				ID:      "F456",
				Name:    "config.md",
				Type:    "text/markdown",
				Content: "# Config\n\nport: 8080\n",
			},
			want: `<file id="F456" name="config.md" type="text/markdown"># Config

port: 8080
</file>`,
		},
		{
			name: "unsupported file has status and reason",
			file: agent.MessageFile{
				ID:     "F789",
				Name:   "photo.png",
				Type:   "image/png",
				Status: "unsupported",
				Reason: "non-textual file",
			},
			want: `<file id="F789" name="photo.png" type="image/png" status="unsupported" reason="non-textual file"></file>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := agent.NewXMLPart("file", tt.file).Render()
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
