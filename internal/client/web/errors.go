package web

import "errors"

// Sentinel errors returned by every Gateway implementation. Callers
// match with errors.Is, never type-assert to a version-specific package.
var (
	// ErrLoginFailed indicates the authenticated session was not
	// established (login form rejected creds, server stayed on /login).
	ErrLoginFailed = errors.New("web: login failed")

	// ErrCreateFailed indicates a write op (create team, affiliation, …)
	// did not redirect away from its /add path.
	ErrCreateFailed = errors.New("web: create failed")

	// ErrUpdateFailed indicates an edit op did not redirect away from
	// its /edit path.
	ErrUpdateFailed = errors.New("web: update failed")

	// ErrMalformedRow indicates a table row missed an <a> the parser
	// expected. Returned by every list parser instead of panicking.
	ErrMalformedRow = errors.New("web: malformed table row")

	// ErrUnauthorized maps to HTTP 401 from any /jury fetch.
	ErrUnauthorized = errors.New("web: unauthorized")

	// ErrServerError maps to HTTP 5xx from any /jury fetch.
	ErrServerError = errors.New("web: server error")

	// ErrUnsupportedOnV8 marks Gateway methods that the v8 scraper
	// intentionally does not implement because the v4 API owns those
	// operations in Phase 2.
	ErrUnsupportedOnV8 = errors.New("web: unsupported on v8 scraper")

	// ErrTeamIDMissing is returned by importer paths when a user exists
	// on the server but the API snapshot's team_id is nil and the
	// fallback lookup (singular /users/{id} + team-list-by-name)
	// failed to recover one. Wrap to surface the username at the
	// importer layer.
	ErrTeamIDMissing = errors.New("web: team_id missing on snapshot")
)
