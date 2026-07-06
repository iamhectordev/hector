package email

import (
	configemail "github.com/iamhectordev/hector/internal/email"
	"github.com/iamhectordev/hector/pkg/telem"
)

const spanModuleStart = "email.module.start"

func moduleFields(cfg configemail.Config) []telem.Field {
	return []telem.Field{
		telem.String("surface.name", "email"),
		telem.String("email.provider", string(cfg.Provider)),
		telem.String("email.mailbox_address", cfg.MailboxAddress),
		telem.String("email.folder", cfg.IMAP.Folder),
	}
}
