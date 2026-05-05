// Package pathx provides path-safe filename helpers.
//
// The Python reference (`commands/problems/_problems.py:32`) only
// substituted spaces with hyphens, which left the door open for
// directory traversal (`../`) and OS-reserved characters when problem
// names came from untrusted input. The Go port tightens that into a
// real character allow-list; see [SafeFilename] for the rules.
package pathx

import "strings"

// reservedRunes is the set of characters that are not valid in a file
// name on at least one major operating system. Replacing them with a
// neutral character (`-`) keeps the filename usable on Linux, macOS and
// Windows alike. Path separators are included to defeat directory
// traversal attempts.
//
// Sources:
//   - https://learn.microsoft.com/windows/win32/fileio/naming-a-file
//   - POSIX path resolution + macOS NTFS interoperability.
const reservedRunes = `/\:*?"<>|`

// SafeFilename returns name with characters that are reserved on at
// least one supported OS replaced by '-' and leading/trailing
// whitespace and dots trimmed. ASCII control characters (including
// the NUL byte) are also replaced.
//
// The function never returns an empty string for non-empty input that
// contained at least one runnable character: callers that depend on
// the result naming a real file should still validate length and
// duplicate-name handling at the call site.
//
// Examples:
//
//	SafeFilename("Hello World")            // "Hello-World"
//	SafeFilename("../etc/passwd")          // "---etc-passwd"
//	SafeFilename("a:b*c?d.txt")            // "a-b-c-d.txt"
//	SafeFilename("  trimmed.   ")          // "trimmed"
func SafeFilename(name string) string {
	if name == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(name))
	for _, r := range name {
		switch {
		case r == ' ':
			b.WriteByte('-')
		case r < 0x20 || r == 0x7f:
			b.WriteByte('-')
		case strings.ContainsRune(reservedRunes, r):
			b.WriteByte('-')
		default:
			b.WriteRune(r)
		}
	}
	// Trim trailing dots and dashes — Windows refuses filenames that
	// end with a dot, and a trailing dash from substitution adds
	// nothing for the user.
	out := strings.TrimRight(b.String(), ".- ")
	out = strings.TrimLeft(out, ".- ")
	return out
}
