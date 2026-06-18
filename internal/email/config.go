package email

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
)

var validate = validator.New(validator.WithRequiredStructEnabled())

type Provider string

const ProviderIMAPSMTP Provider = "imap_smtp"

// Config contains inbound email mailbox settings.
type Config struct {
	Enabled        bool               `yaml:"enabled" env:"EMAIL_ENABLED"`
	Provider       Provider           `yaml:"provider" env:"EMAIL_PROVIDER"`
	MailboxAddress string             `yaml:"mailbox_address" env:"EMAIL_MAILBOX_ADDRESS"`
	IMAP           IMAPConfig         `yaml:"imap"`
	InboundAllow   InboundAllowConfig `yaml:"inbound_allow"`
}

type IMAPConfig struct {
	Host         string       `yaml:"host" env:"EMAIL_IMAP_HOST"`
	Port         int          `yaml:"port" env:"EMAIL_IMAP_PORT"`
	Username     string       `yaml:"username" env:"EMAIL_IMAP_USERNAME"`
	SecretRef    string       `yaml:"secret_ref" env:"EMAIL_IMAP_SECRET_REF"`
	Folder       string       `yaml:"folder" env:"EMAIL_IMAP_FOLDER"`
	PollInterval PollInterval `yaml:"poll_interval" env:"EMAIL_IMAP_POLL_INTERVAL"`
}

type InboundAllowConfig struct {
	ExactAddresses []string `yaml:"exact_addresses"`
	Domains        []string `yaml:"domains"`
}

type PollInterval time.Duration

func (p *PollInterval) UnmarshalText(text []byte) error {
	d, err := time.ParseDuration(string(text))
	if err != nil {
		return err
	}
	*p = PollInterval(d)
	return nil
}

func (p PollInterval) Duration() time.Duration {
	return time.Duration(p)
}

func (p PollInterval) String() string {
	return time.Duration(p).String()
}

func ValidateConfig(cfg Config) error {
	var fields []string
	if cfg.Enabled {
		fields = append(fields, validateEnabledConfig(cfg)...)
		fields = append(fields, validateInboundAllow(cfg.InboundAllow)...)
	}
	if len(fields) > 0 {
		return fmt.Errorf("email: invalid config: %s", strings.Join(fields, ", "))
	}
	return nil
}

func validateEnabledConfig(cfg Config) []string {
	var fields []string
	if cfg.Provider != ProviderIMAPSMTP {
		fields = append(fields, "Provider")
	}
	if err := validate.Var(cfg.MailboxAddress, "required,email"); err != nil {
		fields = append(fields, "MailboxAddress")
	}
	if cfg.IMAP.Host == "" {
		fields = append(fields, "Host")
	}
	if cfg.IMAP.Port <= 0 {
		fields = append(fields, "Port")
	}
	if cfg.IMAP.Username == "" {
		fields = append(fields, "Username")
	}
	if cfg.IMAP.SecretRef == "" {
		fields = append(fields, "SecretRef")
	}
	if cfg.IMAP.Folder == "" {
		fields = append(fields, "Folder")
	}
	if cfg.IMAP.PollInterval.Duration() <= 0 {
		fields = append(fields, "PollInterval")
	}
	return fields
}

func validateInboundAllow(cfg InboundAllowConfig) []string {
	var fields []string
	for _, address := range cfg.ExactAddresses {
		if err := validate.Var(address, "required,email"); err != nil {
			fields = append(fields, "ExactAddresses")
			break
		}
	}
	for _, domain := range cfg.Domains {
		if strings.HasPrefix(domain, "@") || validate.Var(domain, "required,fqdn") != nil {
			fields = append(fields, "Domains")
			break
		}
	}
	return fields
}
