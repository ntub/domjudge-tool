//go:build desktop

package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/ntub/domjudge-tool/internal/client/web"
	"github.com/ntub/domjudge-tool/internal/desktop/handler"
	"github.com/ntub/domjudge-tool/internal/desktop/profile"
)

func TestDesktopSetPassword_VersionedWrite_Integration(t *testing.T) {
	login, err := os.ReadFile("../../testdata/web/v8/login.html")
	if err != nil {
		t.Fatal(err)
	}
	edit, err := os.ReadFile("../../testdata/web/v8/user_edit_form.html")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name      string
		version   string
		loginFail bool
	}{
		{name: "v7 API reset", version: "7.3.4"},
		{name: "v8 scraper reset", version: "8.3.1"},
		{name: "v8 login failure preserves password", version: "8.3.1", loginFail: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var saved atomic.Value
			saved.Store("original")
			mux := http.NewServeMux()
			mux.HandleFunc("GET /api/v4/users", func(w http.ResponseWriter, _ *http.Request) {
				_, _ = io.WriteString(w, `[{"id":"7","username":"user1","name":"Team Alpha"}]`)
			})
			mux.HandleFunc("POST /api/v4/users/accounts", func(w http.ResponseWriter, r *http.Request) {
				if strings.HasPrefix(tc.version, "8.") {
					http.Error(w, "account already exists", http.StatusBadRequest)
					return
				}
				file, _, err := r.FormFile("tsv")
				if err != nil {
					t.Errorf("accounts upload: %v", err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				defer func() { _ = file.Close() }()
				body, err := io.ReadAll(file)
				if err != nil || string(body) != "accounts\t1\nteam\tTeam Alpha\tuser1\trotated\n" {
					t.Errorf("accounts payload = %q, error = %v", body, err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				saved.Store("rotated")
				_, _ = io.WriteString(w, `["7"]`)
			})
			mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet || tc.loginFail {
					_, _ = w.Write(login)
					return
				}
				http.Redirect(w, r, "/jury", http.StatusSeeOther)
			})
			mux.HandleFunc("GET /jury", func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			})
			mux.HandleFunc("/jury/users/7/edit", func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet {
					_, _ = w.Write(edit)
					return
				}
				if err := r.ParseForm(); err != nil {
					t.Errorf("parse edit: %v", err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				if r.FormValue("user[username]") != "user1" || r.FormValue("user[plainPassword]") != "rotated" ||
					r.FormValue("user[enabled]") != "1" || !reflect.DeepEqual(r.PostForm["user[user_roles][]"], []string{"3"}) {
					t.Errorf("password edit changes wrong account or roles: %v", r.PostForm)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				saved.Store("rotated")
				http.Redirect(w, r, "/jury/users/7", http.StatusSeeOther)
			})
			mux.HandleFunc("GET /jury/users/7", func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			})
			srv := httptest.NewServer(mux)
			t.Cleanup(srv.Close)
			result, err := desktopRunSetPassword(context.Background(), profile.ServerProfile{
				Host: srv.URL, Username: "admin", Password: "secret", Version: tc.version,
			}, handler.SetPasswordOpts{Username: "user1", Password: "rotated"})
			if tc.loginFail {
				if !errors.Is(err, web.ErrLoginFailed) || saved.Load() != "original" {
					t.Fatalf("failed login: error=%v, stored password=%v", err, saved.Load())
				}
				return
			}
			if err != nil {
				t.Fatalf("reset password: %v", err)
			}
			if saved.Load() != "rotated" || result.Username != "user1" || result.Password != "rotated" || result.Generated {
				t.Fatalf("reset result=%+v, stored password=%v", result, saved.Load())
			}
		})
	}
}
