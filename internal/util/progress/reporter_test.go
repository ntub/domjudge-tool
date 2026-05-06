package progress

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

// fakeFile is an io.Writer that wraps a buffer but is not *os.File, so
// production New() always treats it as non-TTY. Used to assert that
// gating short-circuits to Nop without exercising the schollz writer.
type fakeFile struct {
	buf bytes.Buffer
}

func (f *fakeFile) Write(p []byte) (int, error) {
	return f.buf.Write(p)
}

// envFor returns a deterministic env lookup so shouldEnable can be
// driven without leaking through to the real process environment.
func envFor(env map[string]string) func(string) string {
	return func(k string) string {
		return env[k]
	}
}

func TestReporter_NonTTYWriter_NoOutput(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, false)
	r.Start(3, "import")
	r.Tick()
	r.Tick()
	r.Tick()
	r.Finish()

	if got := buf.Len(); got != 0 {
		t.Fatalf("expected no output for non-TTY writer, got %d bytes: %q",
			got, buf.String())
	}
}

func TestReporter_NoProgressFlag_NoOutput(t *testing.T) {
	// Even if the writer were TTY-like, the explicit flag must win.
	// Here we use a real *os.File (the env's stderr is not necessarily
	// a TTY in CI, but the gate must not even reach the IsTerminal
	// check when noProgress=true).
	r := New(os.Stderr, true)
	if _, ok := r.(nopReporter); !ok {
		t.Fatalf("expected Nop reporter when noProgress=true, got %T", r)
	}
	r.Start(2, "x")
	r.Tick()
	r.Finish()
}

func TestReporter_NO_COLOR_NoOutput(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	t.Setenv("TERM", "xterm-256color")

	// Even passing a real *os.File must yield Nop because of NO_COLOR.
	if got := shouldEnable(os.Stderr, false, os.Getenv); got {
		t.Fatalf("shouldEnable returned true with NO_COLOR=1; want false")
	}
	r := New(os.Stderr, false)
	if _, ok := r.(nopReporter); !ok {
		t.Fatalf("expected Nop reporter with NO_COLOR=1, got %T", r)
	}
}

func TestReporter_TERMDumb_NoOutput(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("TERM", "dumb")

	if got := shouldEnable(os.Stderr, false, os.Getenv); got {
		t.Fatalf("shouldEnable returned true with TERM=dumb; want false")
	}
	r := New(os.Stderr, false)
	if _, ok := r.(nopReporter); !ok {
		t.Fatalf("expected Nop reporter with TERM=dumb, got %T", r)
	}
}

func TestReporter_TickIncrements_FinishWritesNewline(t *testing.T) {
	// Use the in-package test helper to bypass TTY gating; we need the
	// schollz writer to actually receive bytes so we can assert that
	// Start emitted the label and Tick advanced the bar.
	var buf bytes.Buffer
	r := newRealBarForTesting(&buf)

	r.Start(3, "import-loop")
	r.Tick()
	r.Tick()
	r.Tick()
	r.Finish()

	out := buf.String()
	if out == "" {
		t.Fatalf("expected schollz to write progress bytes; got empty buffer")
	}
	if !strings.Contains(out, "import-loop") {
		t.Fatalf("expected description %q in output, got %q",
			"import-loop", out)
	}
	// schollz emits a "3/3" count when OptionShowCount is set; check
	// the loosest form so we don't snapshot byte-identical schollz output.
	if !strings.Contains(out, "3/3") {
		t.Fatalf("expected count %q after three Ticks + Finish, got %q",
			"3/3", out)
	}
	// Finish appends a newline (via OptionOnCompletion) so the next
	// stderr line starts cleanly.
	if !strings.HasSuffix(out, "\n") {
		t.Fatalf("expected output to end with newline after Finish, got %q",
			out)
	}
}

func TestReporter_NopFromTopFunc_IsSafeToCall(t *testing.T) {
	// Public API: Nop() must be safe to call in any order without
	// panicking. Library code that takes Reporter via context will
	// fall through to Nop when no CLI was wired in.
	r := Nop()
	r.Tick()   // before Start
	r.Finish() // before Start
	r.Start(0, "")
	r.Start(5, "label")
	r.Tick()
	r.Tick()
	r.Finish()
	r.Finish() // double Finish must also be safe
}

// TestShouldEnable_Matrix exercises the gate predicate exhaustively
// without going through New. Useful because unit tests can't reliably
// produce a TTY-backed *os.File on every CI runner.
func TestShouldEnable_Matrix(t *testing.T) {
	type tc struct {
		name       string
		w          io.Writer
		noProgress bool
		env        map[string]string
		want       bool
	}
	cases := []tc{
		{
			name:       "non-tty buffer is gated off",
			w:          &bytes.Buffer{},
			noProgress: false,
			env:        map[string]string{"TERM": "xterm-256color"},
			want:       false,
		},
		{
			name:       "fake file (not *os.File) is gated off",
			w:          &fakeFile{},
			noProgress: false,
			env:        map[string]string{"TERM": "xterm-256color"},
			want:       false,
		},
		{
			name:       "noProgress overrides everything",
			w:          os.Stderr,
			noProgress: true,
			env:        map[string]string{"TERM": "xterm-256color"},
			want:       false,
		},
		{
			name:       "NO_COLOR=1 disables",
			w:          os.Stderr,
			noProgress: false,
			env:        map[string]string{"NO_COLOR": "1", "TERM": "xterm"},
			want:       false,
		},
		{
			name:       "TERM=dumb disables",
			w:          os.Stderr,
			noProgress: false,
			env:        map[string]string{"NO_COLOR": "", "TERM": "dumb"},
			want:       false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := shouldEnable(c.w, c.noProgress, envFor(c.env))
			if got != c.want {
				t.Fatalf("shouldEnable=%v, want %v", got, c.want)
			}
		})
	}
}
