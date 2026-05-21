package agent

import (
	"context"
	"encoding/xml"

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
	Files      []MessageFile     `xml:"file,omitempty"`
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

type MessageFile struct {
	ID      string `xml:"id,attr"`
	Name    string `xml:"name,attr,omitempty"`
	Type    string `xml:"type,attr,omitempty"`
	Status  string `xml:"status,attr,omitempty"`
	Reason  string `xml:"reason,attr,omitempty"`
	Content string `xml:"-"`
}

func (f MessageFile) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "id"}, Value: f.ID})
	if f.Name != "" {
		start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "name"}, Value: f.Name})
	}
	if f.Type != "" {
		start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "type"}, Value: f.Type})
	}
	if f.Status != "" {
		start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "status"}, Value: f.Status})
	}
	if f.Reason != "" {
		start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "reason"}, Value: f.Reason})
	}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	if f.Content != "" {
		if err := e.EncodeToken(xml.CharData(f.Content)); err != nil {
			return err
		}
	}
	return e.EncodeToken(start.End())
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
		Files:      slackFilesXML(data.Files),
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

func slackFilesXML(files []slack.FileAttachment) []MessageFile {
	if len(files) == 0 {
		return nil
	}
	out := make([]MessageFile, 0, len(files))
	for _, file := range files {
		msgFile := MessageFile{
			ID:      file.ID,
			Name:    file.Name,
			Type:    file.ContentType,
			Content: file.Content,
			Status:  string(file.Status),
			Reason:  file.Reason,
		}
		out = append(out, msgFile)
	}
	return out
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
