package app

import (
	"context"
	"testing"
	"time"

	configemail "github.com/iamhectordev/hector/internal/email"
	"github.com/iamhectordev/hector/pkg/supervisor"
	"github.com/stretchr/testify/require"
)

func TestRuntimeEmailModuleDisabledKeepsModulesUnchanged(t *testing.T) {
	t.Parallel()

	runtime := &Runtime{cfg: &Config{}}
	modules := []supervisor.Module{namedModule{name: "agent"}}

	modules, err := runtime.appendEmailModule(modules)

	require.NoError(t, err)
	require.Len(t, modules, 1)
	require.Equal(t, "agent", modules[0].Name())
}

func TestRuntimeEmailModuleEnabledAppendsEmailModule(t *testing.T) {
	t.Parallel()

	runtime := &Runtime{cfg: &Config{Email: validEmailConfig(t)}}
	modules := []supervisor.Module{namedModule{name: "agent"}}

	modules, err := runtime.appendEmailModule(modules)

	require.NoError(t, err)
	require.Len(t, modules, 2)
	require.Equal(t, "agent", modules[0].Name())
	require.Equal(t, "email", modules[1].Name())
}

type namedModule struct {
	name string
}

func (m namedModule) Name() string { return m.name }

func (m namedModule) Init(context.Context) error { return nil }

func (m namedModule) Start(context.Context) error { return nil }

func (m namedModule) Stop(context.Context) error { return nil }

func validEmailConfig(t *testing.T) configemail.Config {
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
