package v7

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/ntub/domjudge-tool/internal/client/web"
)

// HTTPError wraps a non-2xx response from a /jury fetch. Its Is() method
// matches the gateway-level sentinels in web/errors.go, so callers can
// write errors.Is(err, web.ErrUnauthorized) without depending on this
// package. Mirrors api/v4.HTTPError in shape.
type HTTPError struct {
	Status int
	URL    string
	Body   []byte
}

// Error implements the error interface.
func (e *HTTPError) Error() string {
	return fmt.Sprintf("v7: http %d at %s", e.Status, e.URL)
}

// Is matches against the package-level sentinels in web/.
func (e *HTTPError) Is(target error) bool {
	if e.Status == http.StatusUnauthorized {
		return errors.Is(target, web.ErrUnauthorized)
	}
	if e.Status >= 500 {
		return errors.Is(target, web.ErrServerError)
	}
	return false
}
