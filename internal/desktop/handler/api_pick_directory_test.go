package handler_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/ntub/domjudge-tool/internal/desktop/handler"
)

func TestPickDirectory_Success(t *testing.T) {
	picker := func(ctx context.Context) (string, error) {
		return "/path/to/picked/dir", nil
	}

	mux := http.NewServeMux()
	handler.RegisterPickDirectory(mux, picker)

	req := httptest.NewRequest(http.MethodGet, "/api/pick-directory?signal=customSignal", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	// Check if it's SSE and contains the customSignal with the selected path
	if !strings.Contains(body, "event: datastar-patch-signals") {
		t.Errorf("expected SSE event datastar-patch-signals, got %q", body)
	}
	if !strings.Contains(body, `customSignal`) {
		t.Errorf("expected customSignal in body, got %q", body)
	}
	if !strings.Contains(body, `/path/to/picked/dir`) {
		t.Errorf("expected path in body, got %q", body)
	}
}

func TestPickDirectory_DefaultSignal(t *testing.T) {
	picker := func(ctx context.Context) (string, error) {
		return "/default/path", nil
	}

	mux := http.NewServeMux()
	handler.RegisterPickDirectory(mux, picker)

	req := httptest.NewRequest(http.MethodGet, "/api/pick-directory", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, `outDir`) {
		t.Errorf("expected default signal outDir in body, got %q", body)
	}
	if !strings.Contains(body, `/default/path`) {
		t.Errorf("expected default path in body, got %q", body)
	}
}

func TestPickDirectory_Error(t *testing.T) {
	picker := func(ctx context.Context) (string, error) {
		return "", errors.New("failed to pick")
	}

	mux := http.NewServeMux()
	handler.RegisterPickDirectory(mux, picker)

	req := httptest.NewRequest(http.MethodGet, "/api/pick-directory", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", rec.Code)
	}
}

func TestPickDirectory_Cancel(t *testing.T) {
	picker := func(ctx context.Context) (string, error) {
		return "", nil
	}

	mux := http.NewServeMux()
	handler.RegisterPickDirectory(mux, picker)

	req := httptest.NewRequest(http.MethodGet, "/api/pick-directory", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", rec.Code)
	}
}

func TestPickDirectory_InvalidSignal_FallsBackToOutDir(t *testing.T) {
	picker := func(ctx context.Context) (string, error) {
		return "/some/path", nil
	}

	mux := http.NewServeMux()
	handler.RegisterPickDirectory(mux, picker)

	for _, bad := range []string{"../evil", "foo bar", "signal;drop", "<script>", "a.b"} {
		req := httptest.NewRequest(http.MethodGet, "/api/pick-directory?signal="+url.QueryEscape(bad), nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		body := rec.Body.String()
		if !strings.Contains(body, "outDir") {
			t.Errorf("signal %q: expected fallback to outDir, got %q", bad, body)
		}
	}
}

func TestDesktopPickDirectory_CanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := handler.DesktopPickDirectory(ctx)
	if err == nil {
		t.Error("expected error for canceled context")
	}
}
