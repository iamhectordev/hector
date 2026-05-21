package agent

import (
	"context"

	"github.com/iamhectordev/hector/modules/slack"
	"github.com/iamhectordev/hector/modules/tui"
	"github.com/iamhectordev/hector/pkg/session"
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
	SenderID   string            `xml:"sender_id,attr,omitempty"`
	SenderName string            `xml:"sender_name,attr,omitempty"`
	Text       string            `xml:"text"`
	Reactions  *MessageReactions `xml:"reactions,omitempty"`
}

type MessageReactions struct {
	Status string            `xml:"status,attr,omitempty"`
	Reason string            `xml:"reason,attr,omitempty"`
	Items  []MessageReaction `xml:"r,omitempty"`
}

type MessageReaction struct {
	Emoji string `xml:"emoji,attr"`
	Count int    `xml:"count,attr"`
	You   *bool  `xml:"you,attr,omitempty"`
}

func (m *Module) onTUIMessage(ctx context.Context, e waffle.Event[tui.MessageReceivedData]) error {
	sourceURI := tui.NewOriginURI()
	ctx = session.With(ctx, session.Session{SourceURI: sourceURI})
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
	ctx = session.With(ctx, session.Session{SourceURI: sourceURI})
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
		Reactions:  slackReactionsXML(data.Reactions),
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

func slackReactionsXML(reactions slack.Reactions) *MessageReactions {
	if reactions.Unavailable != nil {
		return &MessageReactions{
			Status: "unavailable",
			Reason: reactions.Unavailable.Reason,
		}
	}
	if len(reactions.Items) == 0 {
		return nil
	}
	items := make([]MessageReaction, 0, len(reactions.Items))
	for _, reaction := range reactions.Items {
		var you *bool
		if reaction.You {
			value := true
			you = &value
		}
		items = append(items, MessageReaction{
			Emoji: reaction.Emoji,
			Count: reaction.Count,
			You:   you,
		})
	}
	return &MessageReactions{Items: items}
}
