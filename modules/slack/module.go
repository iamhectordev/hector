package slack

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/iamhectordev/hector/pkg/waffle"
	slackgo "github.com/slack-go/slack"
	"github.com/slack-go/slack/socketmode"
)

// Module connects to Slack Socket Mode and publishes direct messages on the bus.
type Module struct {
	bus      *waffle.EventBus
	appToken string
	botToken string
	logger   *slog.Logger
}

func NewModule(bus *waffle.EventBus, appToken, botToken string) *Module {
	return &Module{
		bus:      bus,
		appToken: appToken,
		botToken: botToken,
		logger:   slog.Default().With("component", "module", "module", "slack"),
	}
}

func (m *Module) Name() string {
	return "slack"
}

func (m *Module) Start(ctx context.Context) error {
	api := slackgo.New(
		m.botToken,
		slackgo.OptionAppLevelToken(m.appToken),
	)

	auth, err := api.AuthTestContext(ctx)
	if err != nil {
		return fmt.Errorf("slack: auth test: %w", err)
	}
	m.log(ctx).InfoContext(ctx, "slack auth verified", "team_id", auth.TeamID, "user_id", auth.UserID)

	return m.run(ctx, socketmode.New(api))
}

func (m *Module) Stop(context.Context) error {
	return nil
}

func (m *Module) run(ctx context.Context, client *socketmode.Client) error {
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
		m.log(ctx).InfoContext(ctx, "slack module stopping", "cause", context.Cause(ctx))
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

func (m *Module) eventLoop(ctx context.Context, client *socketmode.Client) error {
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
			if err := m.handleSocketEvent(ctx, client, evt); err != nil {
				return err
			}
		}
	}
}

func (m *Module) log(context.Context) *slog.Logger {
	return m.logger
}
