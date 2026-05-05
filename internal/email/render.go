package email

import (
	"github.com/ntub/domjudge-tool/internal/config"
	"github.com/ntub/domjudge-tool/internal/model"
)

// BuildEML assembles a complete RFC 5322 message for user, using tmpl
// and cfg. Returns (rfc5322Bytes, recipient, err). Reuses templateVars,
// Render, recipientFor, and buildMessage from send.go so SendUserAccounts
// and the offline render command share identical byte output.
func BuildEML(
	tmpl *Template,
	user model.CreateUser,
	cfg *config.DomServerClient,
	opts SendOptions,
) (msg []byte, recipient string, err error) {
	domain := domainFromEmail(opts.FromEmail)
	vars := templateVars(user, cfg)
	subject, body := tmpl.Render(vars)
	recipient = recipientFor(user, domain)
	msg, err = buildMessage(opts.FromEmail, recipient, subject, body, tmpl.BodyHTML, domain)
	if err != nil {
		return nil, "", err
	}
	return msg, recipient, nil
}
