// Package cli builds the Cobra command tree consumed by
// cmd/domjudge-tool. Subcommands live under internal/cli/{group}/.
//
// All printing in this package and its subpackages goes through
// cmd.OutOrStdout / cmd.ErrOrStderr — never fmt.Print* — so tests can
// hijack the streams via cmd.SetOut / cmd.SetErr.
package cli

import (
	"context"
	"log/slog"

	"github.com/ntub/domjudge-tool/internal/config"
)

// ctxKey is an unexported key type so these context values cannot
// collide with values stashed by callers outside this package.
type ctxKey int

const (
	ctxConfigKey ctxKey = iota
	ctxLoggerKey
)

// WithConfig stashes cfg on ctx so a subcommand's RunE can fetch it
// via ConfigFromContext. The PreRunE attached by NewRootCmd is the
// canonical writer; subcommands MUST NOT re-load config inside RunE.
func WithConfig(ctx context.Context, cfg *config.DomServerClient) context.Context {
	return context.WithValue(ctx, ctxConfigKey, cfg)
}

// ConfigFromContext returns the config installed by WithConfig, or
// (nil, false) if none was installed.
func ConfigFromContext(ctx context.Context) (*config.DomServerClient, bool) {
	cfg, ok := ctx.Value(ctxConfigKey).(*config.DomServerClient)
	return cfg, ok
}

// WithLogger stashes a *slog.Logger on ctx for downstream RunEs.
func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxLoggerKey, logger)
}

// LoggerFromContext returns the logger installed by WithLogger, or
// (nil, false) if none was installed.
func LoggerFromContext(ctx context.Context) (*slog.Logger, bool) {
	l, ok := ctx.Value(ctxLoggerKey).(*slog.Logger)
	return l, ok
}
