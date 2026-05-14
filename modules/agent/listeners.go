package agent

import (
	"context"

	"github.com/iamhectordev/hector/modules/slack"
	"github.com/iamhectordev/hector/modules/tui"
	"github.com/iamhectordev/hector/pkg/session"
	"github.com/iamhectordev/hector/pkg/waffle"
)

func (m *Module) onTUIMessage(ctx context.Context, e waffle.Event[tui.MessageReceivedData]) error {
	ctx = session.With(ctx, session.Session{SourceURI: tui.NewOriginURI()})
	text := e.Data().Text
	if err := m.handle(ctx, text); err != nil {
		m.log(ctx).ErrorContext(ctx, "agent failed to process tui message",
			"event_id", e.ID(), "event_type", e.Type(), "text_len", len(text), "err", err)
		return err
	}
	return nil
}

func (m *Module) onSlackMessage(ctx context.Context, e waffle.Event[slack.MessageReceivedData]) error {
	data := e.Data()
	ctx = session.With(ctx, session.Session{SourceURI: slack.NewOriginURI(data.Origin.ChannelID, data.Origin.ThreadTS)})
	text := data.Text
	if err := m.handle(ctx, text); err != nil {
		m.log(ctx).ErrorContext(ctx, "agent failed to process slack message",
			"event_id", e.ID(), "event_type", e.Type(), "text_len", len(text), "err", err)
		return err
	}
	return nil
}
