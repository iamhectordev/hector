package slack

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"

	"github.com/iamhectordev/hector/pkg/comms"
	"github.com/iamhectordev/hector/pkg/telem"
	"github.com/iamhectordev/hector/pkg/waffle"
	slackgo "github.com/slack-go/slack"
	"github.com/slack-go/slack/socketmode"
)

type Option func(*Integration)

func WithEventLogger(logger EventLogger) Option {
	return func(m *Integration) {
		m.eventLogger = logger
	}
}

type Integration struct {
	bus         *waffle.EventBus
	appToken    string
	botToken    string
	apiURL      string
	api         *slackgo.Client
	client      *socketmode.Client
	botUserID   string
	eventLogger EventLogger
	onMessage   messageHandler
}

func New(bus *waffle.EventBus, cfg *Config, opts ...Option) (*Integration, error) {
	if cfg == nil {
		return nil, fmt.Errorf("slack: config is required")
	}
	if err := validate.Struct(cfg); err != nil {
		return nil, fmt.Errorf("slack: invalid config: %w", err)
	}
	appToken, err := cfg.AppToken.Value()
	if err != nil {
		return nil, fmt.Errorf("slack: app_token: %w", err)
	}
	botToken, err := cfg.BotToken.Value()
	if err != nil {
		return nil, fmt.Errorf("slack: bot_token: %w", err)
	}
	eventLogger := NewDiscardEventLogger()
	if cfg.EventLog.Enabled {
		eventLogger, err = NewFileEventLogger(cfg.EventLog)
		if err != nil {
			return nil, err
		}
	}
	m := &Integration{
		bus:         bus,
		appToken:    appToken,
		botToken:    botToken,
		apiURL:      cfg.APIURL,
		eventLogger: eventLogger,
	}
	m.onMessage = m.handleMessage
	if len(cfg.AllowUsers) > 0 {
		slog.Default().Info("slack allowlist active", "users", cfg.AllowUsers)
		m.onMessage = allowUsers(cfg.AllowUsers, m.handleMessage)
	}
	for _, opt := range opts {
		opt(m)
	}
	return m, nil
}

func (m *Integration) Name() string {
	return "slack"
}

func (m *Integration) Init(ctx context.Context) error {
	options := []slackgo.Option{
		slackgo.OptionAppLevelToken(m.appToken),
	}
	if m.apiURL != "" {
		options = append(options, slackgo.OptionAPIURL(m.apiURL))
	}
	api := slackgo.New(m.botToken, options...)

	auth, err := api.AuthTestContext(ctx)
	if err != nil {
		return fmt.Errorf("slack: auth test: %w", err)
	}
	m.log(ctx).InfoContext(ctx, "slack auth verified",
		telem.String("team_id", auth.TeamID),
		telem.String("user_id", auth.UserID),
		telem.String("bot_id", auth.BotID),
	)
	m.api = api
	m.botUserID = auth.UserID
	m.client = socketmode.New(api)
	return nil
}

func (m *Integration) Run(ctx context.Context) error {
	client := m.client
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	errCh := make(chan error, 2)
	go func() {
		errCh <- client.RunContext(runCtx)
	}()
	go func() {
		errCh <- m.eventLoop(runCtx, client)
	}()

	select {
	case <-ctx.Done():
		cancel()
		err := <-errCh
		if err != nil && !errors.Is(err, context.Canceled) {
			return fmt.Errorf("slack: stop after run error: %w", err)
		}
		m.log(ctx).InfoContext(ctx, "slack integration stopping", telem.Any("cause", context.Cause(ctx)))
		return nil
	case err := <-errCh:
		cancel()
		if errors.Is(err, context.Canceled) && ctx.Err() != nil {
			return nil
		}
		if err != nil {
			return err
		}
		return fmt.Errorf("slack: socket event loop stopped unexpectedly")
	}
}

func (m *Integration) Close() error {
	return m.eventLogger.Close()
}

func (m *Integration) ReplyHandler() comms.ReplyHandler {
	return NewReplyHandler(func() Replier { return m.api })
}

func (m *Integration) eventLoop(ctx context.Context, client *socketmode.Client) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case evt, ok := <-client.Events:
			if !ok {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				return fmt.Errorf("slack: events channel closed")
			}
			if err := m.eventLogger.Log(ctx, evt); err != nil {
				m.log(ctx).WarnContext(ctx, "slack event log failed", telem.Any("err", err))
			}
			if err := m.handleSocketEvent(ctx, client, evt); err != nil {
				return err
			}
		}
	}
}

func (m *Integration) log(ctx context.Context) telem.ContextLogger {
	return telem.Logger(ctx).With(
		telem.String("component", "module"),
		telem.String("module", "slack"),
	)
}

var _ io.Closer = (*Integration)(nil)
