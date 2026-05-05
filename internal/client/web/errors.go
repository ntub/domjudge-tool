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
)
