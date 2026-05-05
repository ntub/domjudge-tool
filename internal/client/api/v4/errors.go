// Package v4 provides a typed Go client for the DOMjudge REST API v4.
// It mirrors the Python domjudge_tool_cli/services/api/v4 service structure.
package v4

import (
	"errors"
	"fmt"
	"net/http"
)

// Sentinel errors for callers using errors.Is / errors.As.
var (
	// ErrUnauthorized is returned when the server responds with 401.
	ErrUnauthorized = errors.New("api/v4: unauthorized")
	// ErrNotFound is returned when the server responds with 404.
	ErrNotFound = errors.New("api/v4: not found")
	// ErrServerError is returned when the server responds with 5xx.
	ErrServerError = errors.New("api/v4: server error")
	// ErrEmptyResponse is returned when the server returns an empty collection
	// where at least one element is expected (e.g. SourceCode result[0]).
	ErrEmptyResponse = errors.New("api/v4: empty response")
	// ErrInvalidArgument is returned when a required argument is missing or
	// invalid (e.g. empty dst in Submissions.Files).
	ErrInvalidArgument = errors.New("api/v4: invalid argument")
	// ErrImportRejected is returned when a bulk import endpoint rejects a
	// multipart payload with structured validation details.
	ErrImportRejected = errors.New("api/v4: import rejected")
)

// HTTPError wraps a non-2xx HTTP response. Its Is method allows
// errors.Is(err, ErrUnauthorized) / ErrNotFound / ErrServerError to work
// even when HTTPError is wrapped inside a fmt.Errorf chain.
type HTTPError struct {
	StatusCode int
	Body       []byte
	URL        string
}

// Error implements the error interface.
func (e *HTTPError) Error() string {
	return fmt.Sprintf("api/v4: HTTP %d from %s", e.StatusCode, e.URL)
}

// Is makes errors.Is work against the three HTTP sentinel errors.
func (e *HTTPError) Is(target error) bool {
	switch target { //nolint:errorlint // deliberate switch on sentinel identity
	case ErrUnauthorized:
		return e.StatusCode == http.StatusUnauthorized
	case ErrNotFound:
		return e.StatusCode == http.StatusNotFound
	case ErrServerError:
		return e.StatusCode >= 500
	}
	return false
}

// ImportRejectedError wraps a structured 4xx response from a DOMjudge bulk
// import endpoint.
type ImportRejectedError struct {
	StatusCode int
	Body       []byte
	URL        string
	Details    []string
}

// Error implements the error interface.
func (e *ImportRejectedError) Error() string {
	return fmt.Sprintf("api/v4: import rejected with HTTP %d from %s", e.StatusCode, e.URL)
}

// Is makes errors.Is(err, ErrImportRejected) work through wrapping.
func (e *ImportRejectedError) Is(target error) bool {
	return target == ErrImportRejected
}
