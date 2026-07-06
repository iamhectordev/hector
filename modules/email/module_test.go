package email_test

import (
	"context"
	"testing"
	"time"

	configemail "github.com/iamhectordev/hector/internal/email"
	"github.com/iamhectordev/hector/modules/email"
	"github.com/iamhectordev/hector/modules/email/emailtest"
	"github.com/stretchr/testify/require"
)

func TestModuleLifecycleUsesMailbox(t *testing.T) {
	t.Parallel()

	mailbox := emailtest.NewMailbox(email.Email{
		ID:         email.MessageID("msg-1"),
		From:       "alice@example.com",
		Subject:    "hello",
		TextBody:   "hi",
		ReceivedAt: time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC),
	})
	module, err := email.NewModule(validConfig(t), mailbox)
	require.NoError(t, err)
	require.Equal(t, "email", module.Name())

	require.NoError(t, module.Init(t.Context()))

	messages, err := mailbox.FetchUnread(t.Context(), 10)
	require.NoError(t, err)
	require.Len(t, messages, 1)

	require.NoError(t, mailbox.MarkRead(t.Context(), []email.MessageID{email.MessageID("msg-1")}))
	require.True(t, mailbox.IsRead(email.MessageID("msg-1")))

	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan error, 1)
	go func() {
		started <- module.Start(ctx)
	}()
	cancel()
	require.NoError(t, <-started)

	require.NoError(t, module.Stop(t.Context()))
	require.True(t, mailbox.Closed())
}

func TestNewModuleRejectsInvalidConfig(t *testing.T) {
	t.Parallel()

	_, err := email.NewModule(configemail.Config{Enabled: true}, emailtest.NewMailbox())

	require.Error(t, err)
	require.ErrorContains(t, err, "email: invalid config")
}

func TestNewModuleRejectsNilMailbox(t *testing.T) {
	t.Parallel()

	_, err := email.NewModule(validConfig(t), nil)

	require.Error(t, err)
	require.ErrorContains(t, err, "email: mailbox is required")
}

func validConfig(t *testing.T) configemail.Config {
	t.Helper()

	d, err := time.ParseDuration("1m")
	require.NoError(t, err)
	return configemail.Config{
		Enabled:        true,
		Provider:       configemail.ProviderIMAPSMTP,
		MailboxAddress: "hector@example.com",
		IMAP: configemail.IMAPConfig{
			Host:         "imap.example.com",
			Port:         993,
			Username:     "hector@example.com",
			SecretRef:    "secret://email/imap",
			Folder:       "INBOX",
			PollInterval: configemail.PollInterval(d),
		},
	}
}
