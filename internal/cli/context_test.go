package cli_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/ntub/domjudge-tool/internal/cli"
	"github.com/ntub/domjudge-tool/internal/config"
	"github.com/ntub/domjudge-tool/internal/util/progress"
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

// recordingReporter is a sentinel Reporter used to assert
// pointer-equality round-trips through WithReporter /
// ReporterFromContext. It is not exposed by the production package
// because library code never constructs Reporter directly.
type recordingReporter struct{ ticks int }

func (r *recordingReporter) Start(int, string) {}
func (r *recordingReporter) Tick()             { r.ticks++ }
func (r *recordingReporter) Finish()           {}

func TestReporterFromContext_DefaultsToNop(t *testing.T) {
	t.Parallel()

	got := cli.ReporterFromContext(context.Background())
	if got == nil {
		t.Fatalf("ReporterFromContext(empty): got nil, want progress.Nop()")
	}
	// progress.Nop() returns a zero-sized struct value, so two
	// independent calls compare equal as interface values. This
	// locks the contract that ReporterFromContext returns the
	// canonical Nop, not just any silent Reporter.
	if got != progress.Nop() {
		t.Errorf("ReporterFromContext(empty) = %#v, want progress.Nop()", got)
	}
	// The whole point of the Nop fallback is that lib callers can
	// invoke methods unconditionally; assert no panics.
	got.Start(3, "probe")
	got.Tick()
	got.Tick()
	got.Finish()
}

func TestWithReporter_RoundTrip(t *testing.T) {
	t.Parallel()

	rep := &recordingReporter{}
	ctx := cli.WithReporter(context.Background(), rep)
	got := cli.ReporterFromContext(ctx)
	if got != progress.Reporter(rep) {
		t.Fatalf("ReporterFromContext: got %#v, want %#v", got, rep)
	}

	// Drive the recovered Reporter and confirm we mutated the
	// same underlying value — guards against accidental copies in
	// the helper or wrapper indirection.
	got.Tick()
	got.Tick()
	if rep.ticks != 2 {
		t.Errorf("recovered reporter: ticks = %d, want 2", rep.ticks)
	}
}

func TestWithReporter_NilFallsBackToNop(t *testing.T) {
	t.Parallel()

	// A nil Reporter stashed on ctx must fall back to Nop so lib
	// code never has to nil-check what it pulls out.
	ctx := cli.WithReporter(context.Background(), nil)
	got := cli.ReporterFromContext(ctx)
	if got == nil {
		t.Fatalf("ReporterFromContext(nil): got nil, want progress.Nop()")
	}
	if got != progress.Nop() {
		t.Errorf("ReporterFromContext(nil) = %#v, want progress.Nop()", got)
	}
}
