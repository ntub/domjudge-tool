package pathx_test

import (
	"strings"
	"testing"

	"github.com/ntub/domjudge-tool/internal/util/pathx"
)

func TestSafeFilename(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{name: "ascii passthrough", in: "hello.txt", want: "hello.txt"},
		{name: "spaces become dashes", in: "Hello World", want: "Hello-World"},
		{name: "unicode preserved", in: "題目-中文.txt", want: "題目-中文.txt"},
		{name: "trim trailing dot", in: "name.", want: "name"},
		{name: "trim leading dash", in: "-leading", want: "leading"},
		{name: "empty stays empty", in: "", want: ""},
		{name: "only reserved chars collapse", in: "/\\:*?\"<>|", want: ""},
		{name: "control char NUL", in: "a\x00b", want: "a-b"},
		{name: "DEL char", in: "a\x7fb", want: "a-b"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := pathx.SafeFilename(tc.in)
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestPathSafe_RejectsTraversalAndOSReservedChars is the named TDD
// anchor for the Python regression at
// `commands/problems/_problems.py:32`. The Python implementation only
// stripped spaces, leaving `../` traversal sequences and
// Windows-reserved characters intact in problem-derived filenames. The
// Go port must neutralise all such characters so that callers
// constructing `f"{folder}/{problem.id}_{problem.name}.zip"` cannot be
// induced to write outside `folder`.
func TestPathSafe_RejectsTraversalAndOSReservedChars(t *testing.T) {
	got := pathx.SafeFilename("../etc/passwd")
	if strings.Contains(got, "/") || strings.Contains(got, "..") {
		t.Fatalf("traversal sequence survived: %q", got)
	}
	for _, ch := range []string{"/", "\\", ":", "*", "?", `"`, "<", ">", "|"} {
		raw := "name" + ch + "x.zip"
		out := pathx.SafeFilename(raw)
		if strings.Contains(out, ch) {
			t.Errorf("char %q survived: input=%q output=%q", ch, raw, out)
		}
	}
}
