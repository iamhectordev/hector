package email

import (
	"context"
	"fmt"
	"time"

	configemail "github.com/iamhectordev/hector/internal/email"
	"github.com/iamhectordev/hector/pkg/telem"
)

// MessageID identifies a message in an IMAP mailbox.
type MessageID string

// Email is the normalized message shape consumed by the email module.
type Email struct {
	ID         MessageID
	From       string
	To         []string
	Cc         []string
	Subject    string
	TextBody   string
	HTMLBody   string
	ReceivedAt time.Time
}

// IMAPMailbox is the application-facing mailbox API used by the email module.
type IMAPMailbox interface {
	FetchUnread(ctx context.Context, limit int) ([]Email, error)
	MarkRead(ctx context.Context, ids []MessageID) error
	Close(ctx context.Context) error
}

// Module owns inbound email lifecycle.
type Module struct {
	cfg     configemail.Config
	mailbox IMAPMailbox
}

// NewModule creates an email module.
func NewModule(cfg configemail.Config, mailbox IMAPMailbox) (*Module, error) {
	if err := configemail.ValidateConfig(cfg); err != nil {
		return nil, err
	}
	if mailbox == nil {
		return nil, fmt.Errorf("email: mailbox is required")
	}
	return &Module{
		cfg:     cfg,
		mailbox: mailbox,
	}, nil
}

func (m *Module) Name() string { return "email" }

func (m *Module) Init(ctx context.Context) error {
	m.log(ctx).InfoContext(ctx, "email module initialized")
	return nil
}

func (m *Module) Start(ctx context.Context) (err error) {
	ctx, span := telem.Trace(ctx, spanModuleStart, moduleFields(m.cfg)...)
	defer span.End(&err)

	m.log(ctx).InfoContext(ctx, "email module running")
	<-ctx.Done()
	m.log(ctx).InfoContext(ctx, "email module stopping", telem.Any("cause", context.Cause(ctx)))
	return nil
}

func (m *Module) Stop(ctx context.Context) error {
	return m.mailbox.Close(ctx)
}

func (m *Module) log(ctx context.Context) telem.ContextLogger {
	return telem.Logger(ctx).With(
		telem.String("component", "module"),
		telem.String("module", m.Name()),
	)
}
