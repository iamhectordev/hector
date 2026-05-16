package agent

import (
	"context"

	"github.com/iamhectordev/hector/modules/slack"
	"github.com/iamhectordev/hector/modules/tui"
	"github.com/iamhectordev/hector/pkg/session"
	"github.com/iamhectordev/hector/pkg/waffle"
)

type ParticipantContext struct {
	ID   string `xml:"id,attr"`
	Name string `xml:"name,attr,omitempty"`
}

type SlackContext struct {
	Platform     string               `xml:"platform,attr"`
	ChannelType  string               `xml:"channel_type,attr,omitempty"`
	ChannelID    string               `xml:"channel_id,attr,omitempty"`
	ChannelName  string               `xml:"channel_name,attr,omitempty"`
	ThreadTS     string               `xml:"thread_ts,attr,omitempty"`
	Participants []ParticipantContext `xml:"participants>participant,omitempty"`
}

type TUIContext struct {
	Platform string `xml:"platform,attr"`
}

func (m *Module) onTUIMessage(ctx context.Context, e waffle.Event[tui.MessageReceivedData]) error {
	ctx = session.With(ctx, session.Session{SourceURI: tui.NewOriginURI()})
	text := e.Data().Text

	tuiCtx := TUIContext{Platform: "tui"}
	system, err := NewPrompt(
		TextPart(m.baseSystem),
		NewXMLPart("conversation", tuiCtx),
	).Render()
	if err != nil {
		return err
	}

	if err := m.handle(ctx, system, text); err != nil {
		m.log(ctx).ErrorContext(ctx, "agent failed to process tui message",
			"event_id", e.ID(), "event_type", e.Type(), "text_len", len(text), "err", err)
		return err
	}
	return nil
}

func (m *Module) onSlackMessage(ctx context.Context, e waffle.Event[slack.MessageReceivedData]) error {
	data := e.Data()
	ctx = session.With(ctx, session.Session{SourceURI: slack.NewOriginURI(data.Channel.ID, data.ThreadTS)})
	text := data.Text

	slackCtx := SlackContext{
		Platform:    "slack",
		ChannelType: string(data.Channel.Type),
		ChannelID:   data.Channel.ID,
		ChannelName: data.Channel.Name,
		ThreadTS:    data.ThreadTS,
	}
	if data.Sender.ID != "" || data.Sender.Name != "" {
		slackCtx.Participants = []ParticipantContext{{
			ID:   data.Sender.ID,
			Name: data.Sender.Name,
		}}
	}

	system, err := NewPrompt(
		TextPart(m.baseSystem),
		NewXMLPart("conversation", slackCtx),
	).Render()
	if err != nil {
		return err
	}

	if err := m.handle(ctx, system, text); err != nil {
		m.log(ctx).ErrorContext(ctx, "agent failed to process slack message",
			"event_id", e.ID(), "event_type", e.Type(), "text_len", len(text), "err", err)
		return err
	}
	return nil
}
