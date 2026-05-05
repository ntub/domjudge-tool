package password_test

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/ntub/domjudge-tool/internal/util/password"
)

func TestGenerate_DefaultLength10AsciiLettersDigits(t *testing.T) {
	got, err := password.Generate(0, password.DefaultCharset)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if utf8.RuneCountInString(got) != password.DefaultLength {
		t.Fatalf("default length = %d, want %d (got %q)", utf8.RuneCountInString(got), password.DefaultLength, got)
	}
	for _, r := range got {
		if !strings.ContainsRune(password.DefaultCharset, r) {
			t.Fatalf("character %q outside default charset", r)
		}
	}
}

func TestGenerate_RejectsEmptyCharset(t *testing.T) {
	_, err := password.Generate(10, "")
	if !errors.Is(err, password.ErrEmptyCharset) {
		t.Fatalf("err = %v, want ErrEmptyCharset", err)
	}
}

func TestGenerate_HonorsCustomLength(t *testing.T) {
	for _, n := range []int{1, 5, 32, 128} {
		got, err := password.Generate(n, "abc")
		if err != nil {
			t.Fatalf("len=%d: %v", n, err)
		}
		if utf8.RuneCountInString(got) != n {
			t.Fatalf("len=%d, got %d (%q)", n, utf8.RuneCountInString(got), got)
		}
	}
}

func TestGenerate_HonorsCustomCharset(t *testing.T) {
	got, err := password.Generate(20, "AB")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, r := range got {
		if r != 'A' && r != 'B' {
			t.Fatalf("unexpected rune %q in %q", r, got)
		}
	}
}

func TestGenerate_RejectsNegativeLength(t *testing.T) {
	_, err := password.Generate(-1, "abc")
	if !errors.Is(err, password.ErrInvalidLength) {
		t.Fatalf("err = %v, want ErrInvalidLength", err)
	}
}

func TestGenerate_ZeroLengthSubstitutesDefault(t *testing.T) {
	got, err := password.Generate(0, "abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if utf8.RuneCountInString(got) != password.DefaultLength {
		t.Fatalf("expected DefaultLength on length=0, got %d", utf8.RuneCountInString(got))
	}
}

// TestGenerate_NegativeLengthBeforeEmptyCharset confirms the negative-
// length check happens before the charset check. Callers that pass both
// invalid arguments get the more semantically pointed error first.
func TestGenerate_NegativeLengthBeforeEmptyCharset(t *testing.T) {
	_, err := password.Generate(-1, "")
	if !errors.Is(err, password.ErrInvalidLength) {
		t.Fatalf("err = %v, want ErrInvalidLength", err)
	}
}

// TestGenerate_HandlesUTF8Charset confirms a multi-byte rune charset
// generates strings whose rune count (not byte count) matches length.
// The Python reference operates on Unicode code points; the Go port
// matches by working on []rune.
func TestGenerate_HandlesUTF8Charset(t *testing.T) {
	got, err := password.Generate(8, "αβγ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if utf8.RuneCountInString(got) != 8 {
		t.Fatalf("rune count = %d, want 8", utf8.RuneCountInString(got))
	}
}
