// Package progress provides an opt-in single-line progress bar for batch
// CLI loops.
//
// The package exposes a small interface, [Reporter], so library packages
// (`internal/email`, `internal/importer`, `internal/submissions`) can take
// progress feedback as an optional dependency. Library code never
// constructs a Reporter — only the CLI root PreRunE does, via [New], and
// passes it through context. Tests and non-TTY pipelines should use [Nop]
// to keep output silent and byte-identical to non-progress runs.
//
// The visual implementation wraps github.com/schollz/progressbar/v3.
// schollz handles carriage-return rewrites, throttling, and terminal
// width detection; this package adds a uniform TTY/environment gate so
// the bar disappears in pipes, dumb terminals, NO_COLOR sessions, and
// `--no-progress` runs without each call site re-implementing the rule.
package progress

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/schollz/progressbar/v3"
	"golang.org/x/term"
)

// Reporter is implemented by code that wants visual feedback for a
// known-length loop. Library packages accept Reporter as an optional
// dependency; [Nop] is silent and safe in tests and non-TTY pipelines.
//
// Calling order is Start → Tick* → Finish. Calling Tick or Finish
// before Start is a no-op (defensive, since Reporter is often passed
// through context with a nil-guard fallback).
type Reporter interface {
	// Start initialises a bar with the total expected ticks and a
	// short label printed before the bar.
	Start(total int, label string)
	// Tick advances the bar by one unit.
	Tick()
	// Finish completes the bar and writes a trailing newline so the
	// next stderr line starts cleanly.
	Finish()
}

// New returns a Reporter that writes a single-line bar to w when w is
// a TTY, NO_COLOR is unset, TERM != "dumb", and noProgress is false.
// Otherwise returns [Nop]. The conventional writer for CLI subcommands
// is `cmd.ErrOrStderr()` so that stdout-as-data pipes stay
// byte-identical to non-progress runs.
func New(w io.Writer, noProgress bool) Reporter {
	if !shouldEnable(w, noProgress, os.Getenv) {
		return Nop()
	}
	return &realReporter{w: w}
}

// Nop returns a no-op Reporter. All methods are safe to call in any
// order; nothing is written.
func Nop() Reporter {
	return nopReporter{}
}

// shouldEnable centralises the TTY / env / flag gate so that production
// New and tests share the same predicate. The lookup function is
// injected so tests can drive env variations without touching the real
// process environment.
func shouldEnable(w io.Writer, noProgress bool, lookup func(string) string) bool {
	if noProgress {
		return false
	}
	if lookup("NO_COLOR") != "" {
		return false
	}
	if lookup("TERM") == "dumb" {
		return false
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

// realReporter wraps *progressbar.ProgressBar. The bar is created
// lazily in Start because we don't know `total` at New time, and a
// fresh bar per Start avoids any ChangeMax / re-Start ambiguity.
type realReporter struct {
	w   io.Writer
	bar *progressbar.ProgressBar
}

func (r *realReporter) Start(total int, label string) {
	w := r.w
	r.bar = progressbar.NewOptions(total,
		progressbar.OptionSetWriter(w),
		progressbar.OptionSetDescription(label),
		progressbar.OptionThrottle(50*time.Millisecond),
		progressbar.OptionShowCount(),
		progressbar.OptionClearOnFinish(),
		// schollz omits the trailing newline by default; the Default*
		// factories use this same hook to keep the next stderr line
		// from clobbering the bar (progressbar.go:511-512).
		progressbar.OptionOnCompletion(func() {
			_, _ = fmt.Fprintln(w)
		}),
	)
}

func (r *realReporter) Tick() {
	if r.bar == nil {
		return
	}
	_ = r.bar.Add(1)
}

func (r *realReporter) Finish() {
	if r.bar == nil {
		return
	}
	_ = r.bar.Finish()
}

// newRealBarForTesting builds a Reporter that writes to w with
// deterministic options (no throttle, no clear-on-finish) so unit
// tests can assert against captured bytes without racing the throttle
// timer or fighting clear-screen sequences. It bypasses the TTY /
// env / flag gate entirely; production callers must continue to go
// through [New].
func newRealBarForTesting(w io.Writer) Reporter {
	return &testReporter{w: w}
}

type testReporter struct {
	w   io.Writer
	bar *progressbar.ProgressBar
}

func (r *testReporter) Start(total int, label string) {
	w := r.w
	r.bar = progressbar.NewOptions(total,
		progressbar.OptionSetWriter(w),
		progressbar.OptionSetDescription(label),
		progressbar.OptionShowCount(),
		progressbar.OptionOnCompletion(func() {
			_, _ = fmt.Fprintln(w)
		}),
	)
}

func (r *testReporter) Tick() {
	if r.bar == nil {
		return
	}
	_ = r.bar.Add(1)
}

func (r *testReporter) Finish() {
	if r.bar == nil {
		return
	}
	_ = r.bar.Finish()
}

// nopReporter is a zero-value-safe no-op implementation. All methods
// are pointer-free so callers can use the literal type or the value
// returned by [Nop] interchangeably.
type nopReporter struct{}

func (nopReporter) Start(int, string) {}
func (nopReporter) Tick()             {}
func (nopReporter) Finish()           {}
