package general

import (
	"strings"
	"testing"
)

func TestVersionLine_LdflagInjectionWins(t *testing.T) {
	saved := Version
	t.Cleanup(func() { Version = saved })
	Version = "v1.0.0-rc1"
	if got := versionLine(); !strings.Contains(got, "v1.0.0-rc1") {
		t.Errorf("versionLine() = %q, want substring %q", got, "v1.0.0-rc1")
	}
}

func TestVersionLine_FallsBackWhenLdflagEmpty(t *testing.T) {
	saved := Version
	t.Cleanup(func() { Version = saved })
	Version = ""
	if got := versionLine(); !strings.HasPrefix(got, "domjudge-tool ") {
		t.Errorf("versionLine() = %q, want prefix %q", got, "domjudge-tool ")
	}
}
