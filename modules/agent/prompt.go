package agent

import (
	"encoding/xml"
	"strings"
)

// SystemPrompt is the base instruction set for every agent turn.
const SystemPrompt = `You are Hector, an AI software engineer colleague.

You MUST always use the 'reply' tool to respond to the user. Never output text directly — it will not reach them. The only way your words arrive is through the 'reply' tool.`

// PromptPart is a composable section of the system prompt.
type PromptPart interface {
	Render() (string, error)
}

// TextPart is a plain string prompt section.
type TextPart string

func (t TextPart) Render() (string, error) {
	return string(t), nil
}

// Prompt aggregates strongly-typed parts and renders them into the final system prompt.
type Prompt []PromptPart

func NewPrompt(parts ...PromptPart) Prompt {
	return parts
}

func (p Prompt) Render() (string, error) {
	var out []string
	for _, part := range p {
		s, err := part.Render()
		if err != nil {
			return "", err
		}
		if s != "" {
			out = append(out, s)
		}
	}
	return strings.Join(out, "\n\n"), nil
}

// XMLPart is a generic adapter that takes any struct implementing xml.Marshaler
// (or structurally valid for xml.Marshal) and wraps it in a root tag, satisfying PromptPart.
type XMLPart struct {
	RootName string
	Data     any
}

func NewXMLPart(rootName string, data any) XMLPart {
	return XMLPart{RootName: rootName, Data: data}
}

func (p XMLPart) Render() (string, error) {
	var buf strings.Builder
	enc := xml.NewEncoder(&buf)
	enc.Indent("", "  ")
	
	start := xml.StartElement{Name: xml.Name{Local: p.RootName}}
	if err := enc.EncodeElement(p.Data, start); err != nil {
		return "", err
	}
	if err := enc.Flush(); err != nil {
		return "", err
	}
	return buf.String(), nil
}
