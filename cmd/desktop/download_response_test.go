//go:build desktop

package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNativeDownloads_SaveAndKeepForm_Integration(t *testing.T) {
	for _, path := range []string{"/export-users", "/download-scoreboard"} {
		t.Run(path, func(t *testing.T) {
			dir := t.TempDir()
			picks := 0
			exports := 0
			h := nativeDownloads(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				exports++
				w.Header().Set("Content-Type", "text/csv")
				w.Header().Set("Content-Disposition", `attachment; filename="roster.csv"`)
				_, _ = io.WriteString(w, "username,name\n")
				_, _ = io.WriteString(w, "user1,Team Alpha\n")
			}), func(_ context.Context, name string) (io.WriteCloser, error) {
				picks++
				return os.Create(filepath.Join(dir, name))
			})
			for range 2 {
				r := httptest.NewRequest(http.MethodPost, path, nil)
				r.Header.Set("User-Agent", "Mozilla/5.0 "+desktopUserAgent)
				w := httptest.NewRecorder()
				h.ServeHTTP(w, r)
				if w.Code != http.StatusNoContent || w.Body.Len() != 0 || w.Header().Get("Content-Disposition") != "" {
					t.Fatalf("native response: status=%d, headers=%v, body=%q", w.Code, w.Header(), w.Body.String())
				}
				data, err := os.ReadFile(filepath.Join(dir, "roster.csv"))
				if err != nil || string(data) != "username,name\nuser1,Team Alpha\n" {
					t.Fatalf("saved CSV = %q, error=%v", data, err)
				}
			}
			if picks != 2 || exports != 2 {
				t.Fatalf("two downloads: picker=%d, exports=%d; want 2 each", picks, exports)
			}
		})
	}
}

func TestNativeDownloads_BrowserStillReceivesAttachment(t *testing.T) {
	h := nativeDownloads(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Disposition", `attachment; filename="users.json"`)
		_, _ = io.WriteString(w, `[{"username":"user1"}]`)
	}), func(context.Context, string) (io.WriteCloser, error) {
		t.Error("browser download opened native picker")
		return nil, nil
	})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/export-users", nil))
	if w.Code != http.StatusOK || w.Body.String() != `[{"username":"user1"}]` ||
		w.Header().Get("Content-Disposition") != `attachment; filename="users.json"` {
		t.Fatalf("browser attachment: status=%d, headers=%v, body=%q", w.Code, w.Header(), w.Body.String())
	}
}

func TestNativeDownloads_CancelAndErrors(t *testing.T) {
	for _, tc := range []struct {
		name       string
		exportFail bool
		pickerErr  error
		wantStatus int
		wantPicks  int
	}{
		{name: "cancel keeps form", wantStatus: http.StatusNoContent, wantPicks: 1},
		{name: "picker failure is not success", pickerErr: errors.New("cannot open output"), wantStatus: http.StatusInternalServerError, wantPicks: 1},
		{name: "export failure never opens picker", exportFail: true, wantStatus: http.StatusBadGateway},
	} {
		t.Run(tc.name, func(t *testing.T) {
			picks := 0
			h := nativeDownloads(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if tc.exportFail {
					http.Error(w, "upstream unavailable", http.StatusBadGateway)
					return
				}
				w.Header().Set("Content-Disposition", `attachment; filename="users.csv"`)
				_, _ = io.WriteString(w, "user1\n")
				_, _ = io.WriteString(w, "user2\n")
			}), func(context.Context, string) (io.WriteCloser, error) {
				picks++
				return nil, tc.pickerErr
			})
			r := httptest.NewRequest(http.MethodPost, "/export-users", nil)
			r.Header.Set("User-Agent", desktopUserAgent)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.wantStatus || picks != tc.wantPicks {
				t.Fatalf("status=%d, picker calls=%d; want %d, %d", w.Code, picks, tc.wantStatus, tc.wantPicks)
			}
			if tc.pickerErr != nil && !strings.Contains(w.Body.String(), tc.pickerErr.Error()) {
				t.Fatalf("save failure not reported: %q", w.Body.String())
			}
		})
	}
}
