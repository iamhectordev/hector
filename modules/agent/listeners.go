package agent

import (
	"context"
	"encoding/xml"
	"time"

	"github.com/iamhectordev/hector/modules/slack"
	"github.com/iamhectordev/hector/modules/tui"
	"github.com/iamhectordev/hector/pkg/llm/schema"
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
	TS         string            `xml:"ts,attr,omitempty"`
	UpdatedAt  string            `xml:"updated_at,attr,omitempty"`
	Text       string            `xml:"text"`
	Reactions  *MessageReactions `xml:"reactions,omitempty"`
	Files      []MessageFile     `xml:"file,omitempty"`
	Images     []MessageImage    `xml:"img,omitempty"`
	Forwards   []MessageForward  `xml:"fwd,omitempty"`
}

type MessageForward struct {
	Message UserMessage `xml:"msg"`
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

type MessageImage struct {
	ID     string `xml:"id,attr"`
	Name   string `xml:"name,attr,omitempty"`
	Type   string `xml:"type,attr,omitempty"`
	Status string `xml:"status,attr,omitempty"`
	Reason string `xml:"reason,attr,omitempty"`
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

	if err := m.handle(ctx, agentCtx, system, []*schema.Message{schema.UserMessage(content)}); err != nil {
		m.log(ctx).ErrorContext(ctx, "agent failed to process tui message",
			"event_id", e.ID(), "event_type", e.Type(), "text_len", len(text), "err", err)
		return err
	}
	return nil
}

func (m *Module) onSlackMessage(ctx context.Context, e waffle.Event[slack.MessageReceivedData]) error {
	data := e.Data()
	msgCtx := newUserMessage(data.Sender, data.Text, data.Reactions, data.Files, data.Images, data.Forwards)
	return m.processSlackMessage(ctx, e.ID(), e.Type(), "slack message", data.Channel, data.ThreadTS, msgCtx, data.Text, data.Images)
}

func (m *Module) onSlackMessageUpdated(ctx context.Context, e waffle.Event[slack.MessageUpdatedData]) error {
	data := e.Data()
	msgCtx := newUserMessage(data.Sender, data.Text, data.Reactions, data.Files, data.Images, data.Forwards)
	msgCtx.UpdatedAt = data.UpdatedAt.Format(time.RFC3339)
	return m.processSlackMessage(ctx, e.ID(), e.Type(), "slack message update", data.Channel, data.ThreadTS, msgCtx, data.Text, data.Images)
}

func (m *Module) processSlackMessage(ctx context.Context, eventID, eventType, logLabel string, channel slack.Channel, threadTS string, msgCtx UserMessage, text string, images []slack.ImageAttachment) error {
	sourceURI := slack.NewOriginURI(channel.ID, threadTS)
	ctx = session.With(ctx, session.Session{SourceURI: sourceURI})

	slackCtx := SlackContext{
		Platform:    "slack",
		ChannelType: string(channel.Type),
		ChannelID:   channel.ID,
		ChannelName: channel.Name,
		ThreadTS:    threadTS,
	}

	system, err := NewPrompt(
		TextPart(m.baseSystem),
		NewXMLPart("conversation", slackCtx),
	).Render()
	if err != nil {
		return err
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

	if err := m.handle(ctx, agentCtx, system, slackUserMessages(content, images)); err != nil {
		m.log(ctx).ErrorContext(ctx, "agent failed to process "+logLabel,
			"event_id", eventID, "event_type", eventType, "text_len", len(text), "err", err)
		return err
	}
	return nil
}

func slackUserMessages(content string, images []slack.ImageAttachment) []*schema.Message {
	parts := slackImageParts(content, images)
	if len(parts) == 0 {
		return []*schema.Message{schema.UserMessage(content)}
	}
	return []*schema.Message{schema.UserMessageWithParts(content, parts)}
}

func newUserMessage(sender slack.Sender, text string, reactions slack.Reactions, files []slack.FileAttachment, images []slack.ImageAttachment, forwards []slack.MessageReceivedData) UserMessage {
	fwd := make([]MessageForward, 0, len(forwards))
	for _, f := range forwards {
		fwd = append(fwd, MessageForward{
			Message: UserMessage{
				SenderID:   f.Sender.ID,
				SenderName: f.Sender.Name,
				TS:         f.TS,
				Text:       f.Text,
				Reactions:  slackReactionsXML(f.Reactions),
				Files:      slackFilesXML(f.Files),
				Images:     slackImagesXML(f.Images),
			},
		})
	}
	return UserMessage{
		SenderID:   sender.ID,
		SenderName: sender.Name,
		Text:       text,
		Reactions:  slackReactionsXML(reactions),
		Files:      slackFilesXML(files),
		Images:     slackImagesXML(images),
		Forwards:   fwd,
	}
}
func slackImageParts(content string, images []slack.ImageAttachment) []schema.MessagePart {
	var parts []schema.MessagePart
	for _, image := range images {
		if image.Base64Data == "" {
			continue
		}
		if len(parts) == 0 {
			parts = append(parts, schema.TextPart(content))
		}
		parts = append(parts,
			schema.TextPart(`<image_data id="`+image.ID+`"/>`),
			schema.NewImagePart(image.ID, image.Base64Data, image.ContentType),
		)
	}
	return parts
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

func slackImagesXML(images []slack.ImageAttachment) []MessageImage {
	if len(images) == 0 {
		return nil
	}
	out := make([]MessageImage, 0, len(images))
	for _, image := range images {
		out = append(out, MessageImage{
			ID:     image.ID,
			Name:   image.Name,
			Type:   image.ContentType,
			Status: string(image.Status),
			Reason: image.Reason,
		})
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
