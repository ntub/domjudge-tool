package v8

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/ntub/domjudge-tool/internal/client/web"
)

// HTTPError wraps a non-2xx response from a /jury fetch. Its Is()
// method matches the gateway-level sentinels in web/errors.go.
type HTTPError struct {
	Status int
	URL    string
	Body   []byte
}

// Error implements the error interface.
func (e *HTTPError) Error() string {
	return fmt.Sprintf("v8: http %d at %s", e.Status, e.URL)
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
