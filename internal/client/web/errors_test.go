package web

import (
	"errors"
	"fmt"
	"testing"
)

// TestErrTeamIDMissing_IsComparable locks ErrTeamIDMissing as an
// errors.Is-comparable sentinel and pins the wire-format string the
// importer wraps with row context. Drift in the sentinel string
// breaks operator-facing error messages and breaks downstream callers
// that errors.Is on the sentinel.
func TestErrTeamIDMissing_IsComparable(t *testing.T) {
	if ErrTeamIDMissing == nil {
		t.Fatal("ErrTeamIDMissing must not be nil")
	}
	wrapped := fmt.Errorf("importer: update team alice: %w", ErrTeamIDMissing)
	if !errors.Is(wrapped, ErrTeamIDMissing) {
		t.Errorf("errors.Is(wrapped, ErrTeamIDMissing) = false, want true")
	}
	if got := ErrTeamIDMissing.Error(); got != "web: team_id missing on snapshot" {
		t.Errorf("ErrTeamIDMissing.Error() = %q, want %q", got, "web: team_id missing on snapshot")
	}
}

func TestErrUnsupportedOnV8_IsComparable(t *testing.T) {
	if ErrUnsupportedOnV8 == nil {
		t.Fatal("ErrUnsupportedOnV8 must not be nil")
	}
	wrapped := fmt.Errorf("v8: update team: %w", ErrUnsupportedOnV8)
	if !errors.Is(wrapped, ErrUnsupportedOnV8) {
		t.Errorf("errors.Is(wrapped, ErrUnsupportedOnV8) = false, want true")
	}
	if got := ErrUnsupportedOnV8.Error(); got != "web: unsupported on v8 scraper" {
		t.Errorf("ErrUnsupportedOnV8.Error() = %q, want %q", got, "web: unsupported on v8 scraper")
	}
}
