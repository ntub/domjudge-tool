// Package emails implements the "Email Commands:" subtree of the
// domjudge-tool CLI: send-user-accounts.
//
// The subtree depends on internal/email for both template loading
// and SMTP delivery, and on internal/importer for CSV parsing.
package emails

import (
	"context"

	"github.com/ntub/domjudge-tool/internal/email"
)

// groupID matches the "emails" group registered in cli.NewRootCmd.
const groupID = "emails"

// DialFn matches email.Dial. Tests inject a stub returning a stub
// Sender.
type DialFn = func(ctx context.Context, opts email.DialOptions) (email.Sender, error)

// Deps groups the factories the emails-subtree needs.
type Deps struct {
	Dial DialFn
}

// DefaultDeps wires Deps with the production email.Dial.
func DefaultDeps() Deps {
	return Deps{Dial: email.Dial}
}

// DialedDeps is exported so tests can capture the DialOptions the
// CLI passes through to Dial. Production code never reads from this
// struct.
type DialedDeps struct {
	Opts email.DialOptions
}
