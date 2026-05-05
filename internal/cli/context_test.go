package cli_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/ntub/domjudge-tool/internal/cli"
	"github.com/ntub/domjudge-tool/internal/config"
)

func TestContextHelpers_RoundTrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		fn   func(t *testing.T)
	}{
		{
			name: "config round-trip",
			fn: func(t *testing.T) {
				cfg := &config.DomServerClient{Host: "https://example.com"}
				ctx := cli.WithConfig(context.Background(), cfg)
				got, ok := cli.ConfigFromContext(ctx)
				if !ok {
					t.Fatalf("ConfigFromContext: ok=false, want true")
				}
				if got != cfg {
					t.Fatalf("ConfigFromContext: pointer differs from input")
				}
			},
		},
		{
			name: "logger round-trip",
			fn: func(t *testing.T) {
				logger := slog.New(slog.NewTextHandler(nopWriter{}, nil))
				ctx := cli.WithLogger(context.Background(), logger)
				got, ok := cli.LoggerFromContext(ctx)
				if !ok || got != logger {
					t.Fatalf("LoggerFromContext: ok=%v got=%p want=%p", ok, got, logger)
				}
			},
		},
		{
			name: "missing config returns ok=false",
			fn: func(t *testing.T) {
				if _, ok := cli.ConfigFromContext(context.Background()); ok {
					t.Fatalf("ConfigFromContext on empty ctx: ok=true, want false")
				}
			},
		},
		{
			name: "missing logger returns ok=false",
			fn: func(t *testing.T) {
				if _, ok := cli.LoggerFromContext(context.Background()); ok {
					t.Fatalf("LoggerFromContext on empty ctx: ok=true, want false")
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, tc.fn)
	}
}

type nopWriter struct{}

func (nopWriter) Write(p []byte) (int, error) { return len(p), nil }
