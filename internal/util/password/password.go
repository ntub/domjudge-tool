// Package password generates random passwords for newly-created users.
//
// It mirrors the Python helper in `domjudge_tool_cli.utils.password`:
// the default length is 10 characters and the default charset is the
// concatenation of ASCII letters and digits, equivalent to
// `string.ascii_letters + string.digits`.
package password

import (
	"crypto/rand"
	"errors"
	"math/big"
)

// DefaultLength is the password length used when length <= 0 is passed.
const DefaultLength = 10

// DefaultCharset is the printable charset used when an empty charset is
// passed. It matches Python's `string.ascii_letters + string.digits`.
const DefaultCharset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// ErrInvalidLength signals that a non-positive length was requested
// after the default-substitution rule had a chance to apply.
var ErrInvalidLength = errors.New("password length must be positive")

// ErrEmptyCharset signals that the caller passed an empty charset and
// disabled the default-substitution rule.
var ErrEmptyCharset = errors.New("password charset must be non-empty")

// Generate returns a cryptographically random password of the given
// length, drawing characters uniformly at random from charset.
//
// A length of 0 substitutes [DefaultLength] (10); a length less than 0
// returns [ErrInvalidLength]. An empty charset returns
// [ErrEmptyCharset] — unlike the Python reference, which silently fell
// back to the default. Callers that want the default charset should
// pass [DefaultCharset] explicitly.
//
// The Python reference uses `random.choices`, which is not
// cryptographically secure. The Go port uses crypto/rand because the
// generated value lands in user-facing credential exports and there is
// no behavioural reason to keep the PRNG path.
func Generate(length int, charset string) (string, error) {
	if length < 0 {
		return "", ErrInvalidLength
	}
	if length == 0 {
		length = DefaultLength
	}
	if charset == "" {
		return "", ErrEmptyCharset
	}
	runes := []rune(charset)

	bound := big.NewInt(int64(len(runes)))
	out := make([]rune, length)
	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, bound)
		if err != nil {
			return "", err
		}
		out[i] = runes[n.Int64()]
	}
	return string(out), nil
}
