package v4

import (
	"errors"
	"fmt"
	"testing"
)

func TestHTTPError_Is(t *testing.T) {
	tests := []struct {
		name   string
		code   int
		target error
		want   bool
	}{
		{name: "401 is ErrUnauthorized", code: 401, target: ErrUnauthorized, want: true},
		{name: "403 is not ErrUnauthorized", code: 403, target: ErrUnauthorized, want: false},
		{name: "404 is ErrNotFound", code: 404, target: ErrNotFound, want: true},
		{name: "403 is not ErrNotFound", code: 403, target: ErrNotFound, want: false},
		{name: "500 is ErrServerError", code: 500, target: ErrServerError, want: true},
		{name: "503 is ErrServerError", code: 503, target: ErrServerError, want: true},
		{name: "404 is not ErrServerError", code: 404, target: ErrServerError, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			httpErr := &HTTPError{StatusCode: tc.code}
			got := errors.Is(httpErr, tc.target)
			if got != tc.want {
				t.Errorf("errors.Is(&HTTPError{StatusCode: %d}, %v) = %v, want %v",
					tc.code, tc.target, got, tc.want)
			}
		})
	}
}

func TestHTTPError_WrappedIsWorks(t *testing.T) {
	httpErr := &HTTPError{StatusCode: 401}
	wrapped := fmt.Errorf("doing X: %w", httpErr)

	if !errors.Is(wrapped, ErrUnauthorized) {
		t.Errorf("errors.Is(wrapped, ErrUnauthorized) = false, want true; err = %v", wrapped)
	}
}
