package agent

import (
	"context"

	"github.com/iamhectordev/hector/modules/slack"
	"github.com/iamhectordev/hector/modules/tui"
	"github.com/iamhectordev/hector/pkg/waffle"
)

type SlackContext struct {
	Platform    string `xml:"platform,attr"`
	ChannelType string `xml:"channel_type,attr,omitempty"`
	ChannelID   string `xml:"channel_id,attr,omitempty"`
	ChannelName string `xml:"channel_name,attr,omitempty"`
	ThreadTS    string `xml:"thread_ts,attr,omitempty"`
}

type TUIContext struct {
	Platform string `xml:"platform,attr"`
}

type UserMessage struct {
	SenderID   string `xml:"sender_id,attr,omitempty"`
	SenderName string `xml:"sender_name,attr,omitempty"`
	Text       string `xml:",chardata"`
}

func (m *Module) onTUIMessage(ctx context.Context, e waffle.Event[tui.MessageReceivedData]) error {
	sourceURI := tui.NewOriginURI()
	text := e.Data().Text

	tuiCtx := TUIContext{Platform: "tui"}
	system, err := NewPrompt(
		TextPart(m.baseSystem),
		NewXMLPart("conversation", tuiCtx),
	).Render()
	if err != nil {
		return err
	}

	msgCtx := UserMessage{Text: text}
	content, err := NewPrompt(
		NewXMLPart("msg", msgCtx),
	).Render()
	if err != nil {
		return err
	}

	agentCtx, err := m.newAgentContext(sourceURI)
	if err != nil {
		return err
	}

	if err := m.handle(ctx, agentCtx, system, content); err != nil {
		m.log(ctx).ErrorContext(ctx, "agent failed to process tui message",
			"event_id", e.ID(), "event_type", e.Type(), "text_len", len(text), "err", err)
		return err
	}
	return nil
}

func (m *Module) onSlackMessage(ctx context.Context, e waffle.Event[slack.MessageReceivedData]) error {
	data := e.Data()
	sourceURI := slack.NewOriginURI(data.Channel.ID, data.ThreadTS)
	text := data.Text

	slackCtx := SlackContext{
		Platform:    "slack",
		ChannelType: string(data.Channel.Type),
		ChannelID:   data.Channel.ID,
		ChannelName: data.Channel.Name,
		ThreadTS:    data.ThreadTS,
	}

	system, err := NewPrompt(
		TextPart(m.baseSystem),
		NewXMLPart("conversation", slackCtx),
	).Render()
	if err != nil {
		return err
	}

	msgCtx := UserMessage{
		SenderID:   data.Sender.ID,
		SenderName: data.Sender.Name,
		Text:       data.Text,
	}
	content, err := NewPrompt(
		NewXMLPart("msg", msgCtx),
	).Render()
	if err != nil {
		return err
	}

	agentCtx, err := m.newAgentContext(sourceURI)
	if err != nil {
		return err
	}

	if err := m.handle(ctx, agentCtx, system, content); err != nil {
		m.log(ctx).ErrorContext(ctx, "agent failed to process slack message",
			"event_id", e.ID(), "event_type", e.Type(), "text_len", len(text), "err", err)
		return err
	}
	return nil
}
