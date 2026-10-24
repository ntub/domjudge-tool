// Package config loads and saves the `domserver.json` configuration
// file consumed by the CLI.
//
// Backward compatibility: the JSON shape (field names, types, and
// defaults) must stay byte-identical to what the Python reference
// (`domjudge_tool_cli.models.domserver.DomServerClient`) produces. JSON
// tags use snake_case and Optional Pydantic fields are pointer types.
// Python-compatible Optional fields omit `omitempty` so that `null`
// values survive round-trip rather than being silently dropped; newer
// Go-only fields may opt into `omitempty` to preserve old canonical
// goldens.
//
// The Python reference defaults `affiliation_country` to `"TWN"`,
// `version` to `"7.3.2"` and `api_version` to `"v4"`. [Load] applies
// those defaults when the input file omits the corresponding key.
package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"slices"
)

// DomServerClient mirrors `domjudge_tool_cli.models.domserver.DomServerClient`.
//
// The pointer types for Optional Pydantic fields are required so that
// JSON round-trip preserves the distinction between "absent" (zero
// value) and "explicitly null" (nil pointer with the key still present
// because there's no `omitempty`).
type DomServerClient struct {
	Host                    string   `json:"host"`
	Username                string   `json:"username"`
	Password                string   `json:"password"`
	DisableSSL              bool     `json:"disable_ssl"`
	Timeout                 *float64 `json:"timeout"`
	MaxConnections          *int     `json:"max_connections"`
	MaxKeepaliveConnections *int     `json:"max_keepalive_connections"`
	CategoryID              *int     `json:"category_id"`
	AffiliationID           *int     `json:"affiliation_id"`
	AffiliationCountry      *string  `json:"affiliation_country"`
	UserRoles               []int    `json:"user_roles"`
	Version                 string   `json:"version"`
	APIVersion              string   `json:"api_version"`
	PreferScraper           *bool    `json:"prefer_scraper,omitempty"`
	// ContestID supplies a contest hint to contest-scoped workflows.
	// Empty uses the workflow's active-contest resolution. Omitting the
	// field preserves compatibility with legacy configuration files.
	ContestID *string `json:"contest_id,omitempty"`
}

// knownFields lists every JSON key recognised by [DomServerClient].
// [Load] uses it to log a warning when an input file contains an
// unknown field; unknown fields are tolerated to keep older releases
// forward-compatible with newer config schemas.
var knownFields = []string{
	"host", "username", "password",
	"disable_ssl", "timeout",
	"max_connections", "max_keepalive_connections",
	"category_id", "affiliation_id", "affiliation_country",
	"user_roles", "version", "api_version", "prefer_scraper",
	"contest_id",
}

// Default returns a DomServerClient pre-populated with the same
// defaults the Python reference applies when fields are missing.
//
// Required fields (host, username, password) are left at their zero
// values; callers must populate them before saving.
func Default() *DomServerClient {
	twn := "TWN"
	return &DomServerClient{
		DisableSSL:         false,
		AffiliationCountry: &twn,
		Version:            "7.3.2",
		APIVersion:         "v4",
	}
}

// ErrMissingRequiredField wraps a [DomServerClient] validation failure
// triggered by a missing required field (host, username, or password).
var ErrMissingRequiredField = errors.New("config: missing required field")

// Load reads the JSON file at path and decodes it into a populated
// [DomServerClient] with defaults applied for any omitted Optional
// fields. Unknown fields are accepted but logged at warn level via the
// default [slog] logger.
//
// Errors from the underlying [os.ReadFile] are returned wrapped so
// callers can distinguish missing files via [errors.Is] +
// [os.ErrNotExist].
func Load(path string) (*DomServerClient, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: read %s: %w", path, err)
	}
	cfg := Default()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("config: parse %s: %w", path, err)
	}

	// Walk the file's top-level keys to surface unknown fields.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err == nil {
		for key := range raw {
			if !slices.Contains(knownFields, key) {
				slog.Warn("unknown field in domserver.json (ignored)", "path", path, "field", key)
			}
		}
	}
	return cfg, nil
}

// Save writes cfg to path as canonical JSON: 4-space indent, no
// trailing newline, fields in [DomServerClient] declaration order. The
// output is round-trip stable: `Load(p)` followed by `Save(c, p)`
// produces a file byte-identical to the input as long as the input was
// itself canonical and contained no unknown fields.
func Save(cfg *DomServerClient, path string) error {
	if cfg == nil {
		return fmt.Errorf("config: cannot save nil DomServerClient")
	}
	data, err := MarshalCanonical(cfg)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("config: write %s: %w", path, err)
	}
	return nil
}

// MarshalCanonical encodes cfg as canonical JSON (see [Save]). It is
// exported so tests can assert byte-identical round-trip without
// touching the filesystem.
func MarshalCanonical(cfg *DomServerClient) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "    ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(cfg); err != nil {
		return nil, fmt.Errorf("config: marshal: %w", err)
	}
	// json.Encoder always appends a newline; strip it so the output
	// matches Go's json.MarshalIndent and the canonical golden fixture.
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// Validate checks that the required fields (host, username, password)
// are populated. It is the caller's responsibility to invoke this
// before using a freshly-decoded config to make HTTP calls; [Load]
// does not call it because some commands legitimately operate on a
// partially-filled config (e.g. `general config` interactive setup).
func (c *DomServerClient) Validate() error {
	switch {
	case c.Host == "":
		return fmt.Errorf("%w: host", ErrMissingRequiredField)
	case c.Username == "":
		return fmt.Errorf("%w: username", ErrMissingRequiredField)
	case c.Password == "":
		return fmt.Errorf("%w: password", ErrMissingRequiredField)
	}
	return nil
}
