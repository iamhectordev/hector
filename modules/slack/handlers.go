package slack

import (
	"context"
	"fmt"
	"time"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"

	"github.com/iamhectordev/hector/pkg/session"
)

func (m *Module) handleSocketEvent(ctx context.Context, client *socketmode.Client, evt socketmode.Event) error {
	m.log(ctx).DebugContext(ctx, "slack socket event", "type", evt.Type)
	switch evt.Type {
	case socketmode.EventTypeConnecting:
		m.log(ctx).DebugContext(ctx, "slack socket connecting")
		return nil
	case socketmode.EventTypeConnected:
		m.log(ctx).DebugContext(ctx, "slack socket connected")
		return nil
	case socketmode.EventTypeConnectionError:
		return m.handleConnectionError(ctx, evt)
	case socketmode.EventTypeInvalidAuth:
		return fmt.Errorf("slack: socket mode invalid auth")
	case socketmode.EventTypeIncomingError:
		m.log(ctx).WarnContext(ctx, "slack socket incoming error", "err", incomingError(evt))
		return nil
	case socketmode.EventTypeErrorBadMessage:
		return badMessageError(evt)
	case socketmode.EventTypeErrorWriteFailed:
		return writeFailedError(evt)
	case socketmode.EventTypeEventsAPI:
		return m.handleEventsAPI(ctx, client, evt)
	default:
		m.log(ctx).DebugContext(ctx, "slack socket event ignored", "event_type", evt.Type)
		return nil
	}
}

func (m *Module) handleEventsAPI(ctx context.Context, client *socketmode.Client, evt socketmode.Event) error {
	apiEvent, ok := evt.Data.(slackevents.EventsAPIEvent)
	if !ok {
		return fmt.Errorf("slack: expected EventsAPIEvent, got %T", evt.Data)
	}

	m.log(ctx).DebugContext(ctx, "slack api event", "type", apiEvent.Type, "inner_type", apiEvent.InnerEvent.Type)

	if apiEvent.Type != slackevents.CallbackEvent {
		return ackSocketEvent(ctx, client, evt)
	}

	switch inner := apiEvent.InnerEvent.Data.(type) {
	case *slackevents.MessageEvent:
		if err := m.handleMessage(ctx, inner); err != nil {
			return err
		}
		return ackSocketEvent(ctx, client, evt)
	default:
		return ackSocketEvent(ctx, client, evt)
	}
}

func (m *Module) handleMessage(ctx context.Context, e *slackevents.MessageEvent) error {
	m.log(ctx).DebugContext(ctx, "slack message event", "user", e.User, "channel", e.Channel, "subtype", e.SubType, "thread_ts", e.ThreadTimeStamp, "ts", e.TimeStamp)
	if e.User == m.botUserID {
		m.log(ctx).DebugContext(ctx, "slack message ignored: bot self-message", "user", e.User)
		return nil
	}
	data, ok, err := messageReceivedData(time.Now().UTC(), e)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	m.enricher().Enrich(ctx, &data, e)

	data.Channel.Type = channelTypeFromSlack(e.ChannelType)

	ctx = session.With(ctx, session.Session{SourceURI: NewOriginURI(data.Channel.ID, data.ThreadTS)})
	// Record before ack so local persistence errors are not hidden behind a successful Slack ack.
	if err := m.bus.Record(ctx, MessageReceived.New(data)); err != nil {
		return fmt.Errorf("failed to record message received: %w", err)
	}
	return nil
}

func (m *Module) enricher() messageEnricher {
	return messageEnricher{
		api:       m.api,
		botUserID: m.botUserID,
	}
}

func channelTypeFromSlack(channelType string) ChannelType {
	switch channelType {
	case "im":
		return ChannelTypeDM
	case "mpim":
		return ChannelTypeGroupDM
	case "channel":
		return ChannelTypeChannel
	case "group":
		return ChannelTypePrivate
	default:
		return ChannelType(channelType)
	}
}

func (m *Module) handleConnectionError(ctx context.Context, evt socketmode.Event) error {
	e, ok := evt.Data.(*slack.ConnectionErrorEvent)
	if !ok {
		return fmt.Errorf("slack: expected ConnectionErrorEvent, got %T", evt.Data)
	}
	if e.ErrorObj == nil {
		return fmt.Errorf("slack: connection error event missing cause")
	}
	m.log(ctx).WarnContext(ctx, "slack socket connection error", "attempt", e.Attempt, "backoff", e.Backoff, "err", e.ErrorObj)
	return nil
}

func ackSocketEvent(ctx context.Context, client *socketmode.Client, evt socketmode.Event) error {
	if evt.Request == nil {
		return fmt.Errorf("slack: socket event %q missing request", evt.Type)
	}
	if evt.Request.EnvelopeID == "" {
		return fmt.Errorf("slack: socket event %q missing envelope id", evt.Type)
	}
	if err := client.AckCtx(ctx, evt.Request.EnvelopeID, nil); err != nil {
		return fmt.Errorf("slack: ack socket event %q: %w", evt.Type, err)
	}
	return nil
}

func incomingError(evt socketmode.Event) error {
	e, ok := evt.Data.(*slack.IncomingEventError)
	if !ok {
		return fmt.Errorf("slack: expected IncomingEventError, got %T", evt.Data)
	}
	if e.ErrorObj == nil {
		return fmt.Errorf("slack: incoming event error missing cause")
	}
	return fmt.Errorf("slack: incoming event error: %w", e.ErrorObj)
}

func badMessageError(evt socketmode.Event) error {
	e, ok := evt.Data.(*socketmode.ErrorBadMessage)
	if !ok {
		return fmt.Errorf("slack: expected ErrorBadMessage, got %T", evt.Data)
	}
	if e.Cause == nil {
		return fmt.Errorf("slack: bad message error missing cause")
	}
	return fmt.Errorf("slack: bad message: %w", e.Cause)
}

func writeFailedError(evt socketmode.Event) error {
	e, ok := evt.Data.(*socketmode.ErrorWriteFailed)
	if !ok {
		return fmt.Errorf("slack: expected ErrorWriteFailed, got %T", evt.Data)
	}
	if e.Cause == nil {
		return fmt.Errorf("slack: write failed error missing cause")
	}
	return fmt.Errorf("slack: write failed: %w", e.Cause)
}
