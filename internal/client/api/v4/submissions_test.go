package v4

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ntub/domjudge-tool/internal/model"
)

func strPtr(s string) *string { return &s }
func boolPtr(b bool) *bool    { return &b }

func TestSubmissionsResource_List(t *testing.T) {
	fixturePath := "../../../../testdata/api/v4/submissions_list.json"
	fixture, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	type wantFn func(t *testing.T, got []model.Submission, err error)

	tests := []struct {
		name       string
		opts       SubmissionsListOpts
		handler    http.HandlerFunc
		preCancel  bool
		assertions wantFn
	}{
		{
			name: "happy path",
			opts: SubmissionsListOpts{},
			handler: func(w http.ResponseWriter, r *http.Request) {
				if !strings.HasSuffix(r.URL.Path, "/api/v4/contests/1/submissions") {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(fixture)
			},
			assertions: func(t *testing.T, got []model.Submission, err error) {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if len(got) != 1 {
					t.Fatalf("got len=%d, want 1", len(got))
				}
				if got[0].ID != "1" {
					t.Errorf("got ID=%q, want %q", got[0].ID, "1")
				}
			},
		},
		{
			name: "empty opts no query string",
			opts: SubmissionsListOpts{},
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.URL.RawQuery != "" {
					t.Errorf("expected empty query string, got %q", r.URL.RawQuery)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(fixture)
			},
			assertions: func(t *testing.T, _ []model.Submission, err error) {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			},
		},
		{
			name: "all opts query string",
			opts: SubmissionsListOpts{
				IDs:        []string{"1", "2"},
				LanguageID: strPtr("cpp"),
				Strict:     boolPtr(true),
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				vals, perr := url.ParseQuery(r.URL.RawQuery)
				if perr != nil {
					t.Fatalf("parse query: %v", perr)
				}
				ids := vals["ids[]"]
				if len(ids) != 2 || ids[0] != "1" || ids[1] != "2" {
					t.Errorf("got ids[]=%v, want [1 2]", ids)
				}
				if got := vals.Get("language_id"); got != "cpp" {
					t.Errorf("got language_id=%q, want %q", got, "cpp")
				}
				if got := vals.Get("strict"); got != "true" {
					t.Errorf("got strict=%q, want %q", got, "true")
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(fixture)
			},
			assertions: func(t *testing.T, _ []model.Submission, err error) {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			},
		},
		{
			name: "401 unauthorized",
			opts: SubmissionsListOpts{},
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
			},
			assertions: func(t *testing.T, _ []model.Submission, err error) {
				if !errors.Is(err, ErrUnauthorized) {
					t.Errorf("got err=%v, want ErrUnauthorized", err)
				}
			},
		},
		{
			name: "404 not found",
			opts: SubmissionsListOpts{},
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			assertions: func(t *testing.T, _ []model.Submission, err error) {
				if !errors.Is(err, ErrNotFound) {
					t.Errorf("got err=%v, want ErrNotFound", err)
				}
			},
		},
		{
			name: "500 server error",
			opts: SubmissionsListOpts{},
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			assertions: func(t *testing.T, _ []model.Submission, err error) {
				if !errors.Is(err, ErrServerError) {
					t.Errorf("got err=%v, want ErrServerError", err)
				}
			},
		},
		{
			name: "malformed json",
			opts: SubmissionsListOpts{},
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{bad json`))
			},
			assertions: func(t *testing.T, _ []model.Submission, err error) {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if !strings.Contains(err.Error(), "decode") {
					t.Errorf("error %q does not contain %q", err.Error(), "decode")
				}
			},
		},
		{
			name: "context cancelled",
			opts: SubmissionsListOpts{},
			handler: func(_ http.ResponseWriter, r *http.Request) {
				select {
				case <-r.Context().Done():
				case <-make(chan struct{}):
				}
			},
			preCancel: true,
			assertions: func(t *testing.T, _ []model.Submission, err error) {
				if !errors.Is(err, context.Canceled) {
					t.Errorf("got err=%v, want context.Canceled", err)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(tc.handler)
			defer srv.Close()

			c := New(Options{BaseURL: srv.URL, Username: "admin", Password: "secret"})

			ctx := context.Background()
			if tc.preCancel {
				cctx, cancel := context.WithCancel(ctx)
				cancel()
				ctx = cctx
			}

			got, err := c.Submissions.List(ctx, "1", tc.opts)
			tc.assertions(t, got, err)
		})
	}
}

func TestSubmissionsResource_Get(t *testing.T) {
	fixturePath := "../../../../testdata/api/v4/submission.json"
	fixture, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	type wantFn func(t *testing.T, got *model.Submission, err error)

	tests := []struct {
		name       string
		handler    http.HandlerFunc
		preCancel  bool
		assertions wantFn
	}{
		{
			name: "happy path",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if !strings.HasSuffix(r.URL.Path, "/api/v4/contests/1/submissions/1") {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(fixture)
			},
			assertions: func(t *testing.T, got *model.Submission, err error) {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got == nil {
					t.Fatalf("got nil result")
				}
				if got.ID != "1" {
					t.Errorf("got ID=%q, want %q", got.ID, "1")
				}
			},
		},
		{
			name: "401 unauthorized",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
			},
			assertions: func(t *testing.T, _ *model.Submission, err error) {
				if !errors.Is(err, ErrUnauthorized) {
					t.Errorf("got err=%v, want ErrUnauthorized", err)
				}
			},
		},
		{
			name: "404 not found",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			assertions: func(t *testing.T, _ *model.Submission, err error) {
				if !errors.Is(err, ErrNotFound) {
					t.Errorf("got err=%v, want ErrNotFound", err)
				}
			},
		},
		{
			name: "500 server error",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			assertions: func(t *testing.T, _ *model.Submission, err error) {
				if !errors.Is(err, ErrServerError) {
					t.Errorf("got err=%v, want ErrServerError", err)
				}
			},
		},
		{
			name: "malformed json",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{bad json`))
			},
			assertions: func(t *testing.T, _ *model.Submission, err error) {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if !strings.Contains(err.Error(), "decode") {
					t.Errorf("error %q does not contain %q", err.Error(), "decode")
				}
			},
		},
		{
			name: "context cancelled",
			handler: func(_ http.ResponseWriter, r *http.Request) {
				select {
				case <-r.Context().Done():
				case <-make(chan struct{}):
				}
			},
			preCancel: true,
			assertions: func(t *testing.T, _ *model.Submission, err error) {
				if !errors.Is(err, context.Canceled) {
					t.Errorf("got err=%v, want context.Canceled", err)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(tc.handler)
			defer srv.Close()

			c := New(Options{BaseURL: srv.URL, Username: "admin", Password: "secret"})

			ctx := context.Background()
			if tc.preCancel {
				cctx, cancel := context.WithCancel(ctx)
				cancel()
				ctx = cctx
			}

			got, err := c.Submissions.Get(ctx, "1", "1")
			tc.assertions(t, got, err)
		})
	}
}

// Locks Python submissions.py:58: os.path.isdir(file_path) panics when file_path is None.
func TestSubmissionsFiles_NilFilePath_DoesNotPanic(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Errorf("server should not be called when dst is empty")
	}))
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL, Username: "admin", Password: "secret"})

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Submissions.Files panicked with empty dst: %v", r)
		}
	}()

	_, err := c.Submissions.Files(context.Background(), "1", "1", "")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("got err=%v, want ErrInvalidArgument", err)
	}
}

func TestSubmissionsResource_Files(t *testing.T) {
	zipPath := "../../../../testdata/api/v4/submission_files.zip"
	zipBytes, err := os.ReadFile(zipPath)
	if err != nil {
		t.Fatalf("read zip fixture: %v", err)
	}

	type wantFn func(t *testing.T, dst, got string, err error)

	tests := []struct {
		name       string
		dstFn      func(t *testing.T) string
		handler    http.HandlerFunc
		preCancel  bool
		assertions wantFn
	}{
		{
			name:  "happy path",
			dstFn: func(t *testing.T) string { return t.TempDir() },
			handler: func(w http.ResponseWriter, r *http.Request) {
				if !strings.HasSuffix(r.URL.Path, "/api/v4/contests/1/submissions/1/files") {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
				w.Header().Set("Content-Type", "application/zip")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(zipBytes)
			},
			assertions: func(t *testing.T, _, got string, err error) {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got == "" {
					t.Fatalf("expected non-empty path")
				}
				if _, statErr := os.Stat(got); statErr != nil {
					t.Errorf("file does not exist at %q: %v", got, statErr)
				}
			},
		},
		{
			name:  "empty dst returns ErrInvalidArgument",
			dstFn: func(_ *testing.T) string { return "" },
			handler: func(_ http.ResponseWriter, _ *http.Request) {
				t.Errorf("server should not be called when dst is empty")
			},
			assertions: func(t *testing.T, _, _ string, err error) {
				if !errors.Is(err, ErrInvalidArgument) {
					t.Errorf("got err=%v, want ErrInvalidArgument", err)
				}
			},
		},
		{
			name: "dst does not exist, creates it",
			dstFn: func(t *testing.T) string {
				return filepath.Join(t.TempDir(), "nested", "subdir")
			},
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/zip")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(zipBytes)
			},
			assertions: func(t *testing.T, _, got string, err error) {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got == "" {
					t.Fatalf("expected non-empty path")
				}
				if _, statErr := os.Stat(got); statErr != nil {
					t.Errorf("file does not exist at %q: %v", got, statErr)
				}
			},
		},
		{
			name:  "401 unauthorized",
			dstFn: func(t *testing.T) string { return t.TempDir() },
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
			},
			assertions: func(t *testing.T, _, _ string, err error) {
				if !errors.Is(err, ErrUnauthorized) {
					t.Errorf("got err=%v, want ErrUnauthorized", err)
				}
			},
		},
		{
			name:  "404 not found",
			dstFn: func(t *testing.T) string { return t.TempDir() },
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			assertions: func(t *testing.T, _, _ string, err error) {
				if !errors.Is(err, ErrNotFound) {
					t.Errorf("got err=%v, want ErrNotFound", err)
				}
			},
		},
		{
			name:  "500 server error",
			dstFn: func(t *testing.T) string { return t.TempDir() },
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			assertions: func(t *testing.T, _, _ string, err error) {
				if !errors.Is(err, ErrServerError) {
					t.Errorf("got err=%v, want ErrServerError", err)
				}
			},
		},
		{
			name:  "context cancelled",
			dstFn: func(t *testing.T) string { return t.TempDir() },
			handler: func(_ http.ResponseWriter, r *http.Request) {
				select {
				case <-r.Context().Done():
				case <-make(chan struct{}):
				}
			},
			preCancel: true,
			assertions: func(t *testing.T, _, _ string, err error) {
				if !errors.Is(err, context.Canceled) {
					t.Errorf("got err=%v, want context.Canceled", err)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(tc.handler)
			defer srv.Close()

			c := New(Options{BaseURL: srv.URL, Username: "admin", Password: "secret"})

			ctx := context.Background()
			if tc.preCancel {
				cctx, cancel := context.WithCancel(ctx)
				cancel()
				ctx = cctx
			}

			dst := tc.dstFn(t)
			got, err := c.Submissions.Files(ctx, "1", "1", dst)
			tc.assertions(t, dst, got, err)
		})
	}
}

func TestSubmissionsResource_SourceCode(t *testing.T) {
	fixturePath := "../../../../testdata/api/v4/source_code.json"
	fixture, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	type wantFn func(t *testing.T, got []model.SubmissionFile, err error)

	tests := []struct {
		name       string
		handler    http.HandlerFunc
		preCancel  bool
		assertions wantFn
	}{
		{
			name: "happy path",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if !strings.HasSuffix(r.URL.Path, "/api/v4/contests/1/submissions/1/source-code") {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(fixture)
			},
			assertions: func(t *testing.T, got []model.SubmissionFile, err error) {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if len(got) != 1 {
					t.Fatalf("got len=%d, want 1", len(got))
				}
				if got[0].ID != "1" {
					t.Errorf("got ID=%q, want %q", got[0].ID, "1")
				}
				if got[0].Filename == nil {
					t.Errorf("expected non-nil Filename")
				}
			},
		},
		{
			name: "empty array returns ErrEmptyResponse",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`[]`))
			},
			assertions: func(t *testing.T, _ []model.SubmissionFile, err error) {
				if !errors.Is(err, ErrEmptyResponse) {
					t.Errorf("got err=%v, want ErrEmptyResponse", err)
				}
			},
		},
		{
			name: "401 unauthorized",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
			},
			assertions: func(t *testing.T, _ []model.SubmissionFile, err error) {
				if !errors.Is(err, ErrUnauthorized) {
					t.Errorf("got err=%v, want ErrUnauthorized", err)
				}
			},
		},
		{
			name: "404 not found",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			assertions: func(t *testing.T, _ []model.SubmissionFile, err error) {
				if !errors.Is(err, ErrNotFound) {
					t.Errorf("got err=%v, want ErrNotFound", err)
				}
			},
		},
		{
			name: "500 server error",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			assertions: func(t *testing.T, _ []model.SubmissionFile, err error) {
				if !errors.Is(err, ErrServerError) {
					t.Errorf("got err=%v, want ErrServerError", err)
				}
			},
		},
		{
			name: "malformed json",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{bad json`))
			},
			assertions: func(t *testing.T, _ []model.SubmissionFile, err error) {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if !strings.Contains(err.Error(), "decode") {
					t.Errorf("error %q does not contain %q", err.Error(), "decode")
				}
			},
		},
		{
			name: "context cancelled",
			handler: func(_ http.ResponseWriter, r *http.Request) {
				select {
				case <-r.Context().Done():
				case <-make(chan struct{}):
				}
			},
			preCancel: true,
			assertions: func(t *testing.T, _ []model.SubmissionFile, err error) {
				if !errors.Is(err, context.Canceled) {
					t.Errorf("got err=%v, want context.Canceled", err)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(tc.handler)
			defer srv.Close()

			c := New(Options{BaseURL: srv.URL, Username: "admin", Password: "secret"})

			ctx := context.Background()
			if tc.preCancel {
				cctx, cancel := context.WithCancel(ctx)
				cancel()
				ctx = cctx
			}

			got, err := c.Submissions.SourceCode(ctx, "1", "1")
			tc.assertions(t, got, err)
		})
	}
}
