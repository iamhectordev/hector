package email_test

import (
	"testing"
	"time"

	"github.com/iamhectordev/hector/internal/email"
	"github.com/stretchr/testify/require"
)

func TestValidateConfigAcceptsDisabledEmptyConfig(t *testing.T) {
	t.Parallel()

	require.NoError(t, email.ValidateConfig(email.Config{}))
}

func TestValidateConfigSkipsEmailValidationWhenDisabled(t *testing.T) {
	t.Parallel()

	cfg := email.Config{
		MailboxAddress: "not-an-email",
		InboundAllow: email.InboundAllowConfig{
			ExactAddresses: []string{"not-an-email"},
			Domains:        []string{"@example.com"},
		},
	}

	require.NoError(t, email.ValidateConfig(cfg))
}

func TestValidateConfigAcceptsEnabledIMAPSMTPConfig(t *testing.T) {
	t.Parallel()

	cfg := validConfig(t)

	require.NoError(t, email.ValidateConfig(cfg))
	require.Equal(t, "1m30s", cfg.IMAP.PollInterval.String())
}

func TestValidateConfigRequiresEnabledFields(t *testing.T) {
	t.Parallel()

	err := email.ValidateConfig(email.Config{Enabled: true})

	require.Error(t, err)
	require.Contains(t, err.Error(), "email: invalid config")
	require.Contains(t, err.Error(), "Provider")
	require.Contains(t, err.Error(), "MailboxAddress")
	require.Contains(t, err.Error(), "Host")
	require.Contains(t, err.Error(), "Port")
	require.Contains(t, err.Error(), "Username")
	require.Contains(t, err.Error(), "SecretRef")
	require.Contains(t, err.Error(), "Folder")
	require.Contains(t, err.Error(), "PollInterval")
}

func TestValidateConfigRejectsInvalidInboundAllowlist(t *testing.T) {
	t.Parallel()

	cfg := validConfig(t)
	cfg.InboundAllow = email.InboundAllowConfig{
		ExactAddresses: []string{"not-an-email"},
		Domains:        []string{"@example.com", "not a domain"},
	}

	err := email.ValidateConfig(cfg)

	require.Error(t, err)
	require.Contains(t, err.Error(), "ExactAddresses")
	require.Contains(t, err.Error(), "Domains")
}

func TestValidateConfigDoesNotLeakSecretRef(t *testing.T) {
	t.Parallel()

	cfg := validConfig(t)
	cfg.IMAP.SecretRef = "prod/email/super-secret-password"
	cfg.IMAP.Port = 0

	err := email.ValidateConfig(cfg)

	require.Error(t, err)
	require.NotContains(t, err.Error(), cfg.IMAP.SecretRef)
}

func TestPollIntervalUnmarshalsDurationText(t *testing.T) {
	t.Parallel()

	var interval email.PollInterval
	require.NoError(t, interval.UnmarshalText([]byte("45s")))
	require.Equal(t, "45s", interval.String())
}

func validConfig(t *testing.T) email.Config {
	t.Helper()

	return email.Config{
		Enabled:        true,
		Provider:       email.ProviderIMAPSMTP,
		MailboxAddress: "hector@example.com",
		IMAP: email.IMAPConfig{
			Host:         "imap.example.com",
			Port:         993,
			Username:     "hector@example.com",
			SecretRef:    "secret://email/imap",
			Folder:       "INBOX",
			PollInterval: pollInterval(t, "1m30s"),
		},
		InboundAllow: email.InboundAllowConfig{
			ExactAddresses: []string{"alice@example.com"},
			Domains:        []string{"example.org"},
		},
	}
}

func pollInterval(t *testing.T, value string) email.PollInterval {
	t.Helper()

	d, err := time.ParseDuration(value)
	require.NoError(t, err)
	return email.PollInterval(d)
}
