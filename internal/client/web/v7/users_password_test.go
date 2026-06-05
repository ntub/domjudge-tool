package v7

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/ntub/domjudge-tool/internal/client/web"
)

const userEditFixture = "../../../../testdata/web/v7/jury_user_edit.html"

func TestSetUserPassword_PostsHiddenAndOverrides(t *testing.T) {
	editForm := mustReadFixture(t, userEditFixture)

	var (
		gotPassword string
		gotEnabled  string
		gotRoles    []string
	)
	mux := http.NewServeMux()
	mux.HandleFunc("/jury/users/42/edit", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_, _ = w.Write(editForm)
		case http.MethodPost:
			_ = r.ParseForm()
			gotPassword = r.PostForm.Get("user[plainPassword]")
			gotEnabled = r.PostForm.Get("user[enabled]")
			gotRoles = r.PostForm["user[user_roles][]"]
			http.Redirect(w, r, "/jury/users/42", http.StatusFound)
		}
	})
	mux.HandleFunc("/jury/users/42", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv)

	if err := c.SetUserPassword(context.Background(), "42", "newpw", []int{3, 5}, true); err != nil {
		t.Fatalf("SetUserPassword: %v", err)
	}
	if gotPassword != "newpw" {
		t.Errorf("plainPassword = %q, want newpw", gotPassword)
	}
	if gotEnabled != "1" {
		t.Errorf("enabled = %q, want 1", gotEnabled)
	}
	want := []string{"3", "5"}
	if !reflect.DeepEqual(gotRoles, want) {
		t.Errorf("user[user_roles][] = %v, want %v", gotRoles, want)
	}
}

func TestUpdateUserEmail_PostsEmailOverride(t *testing.T) {
	editForm := mustReadFixture(t, userEditFixture)

	var gotEmail string
	mux := http.NewServeMux()
	mux.HandleFunc("/jury/users/42/edit", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_, _ = w.Write(editForm)
		case http.MethodPost:
			_ = r.ParseForm()
			gotEmail = r.PostForm.Get("user[email]")
			http.Redirect(w, r, "/jury/users/42", http.StatusFound)
		}
	})
	mux.HandleFunc("/jury/users/42", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv)

	if err := c.UpdateUserEmail(context.Background(), "42", "alice@example.org"); err != nil {
		t.Fatalf("UpdateUserEmail: %v", err)
	}
	if gotEmail != "alice@example.org" {
		t.Errorf("user[email] = %q, want alice@example.org", gotEmail)
	}
}

func TestSetUserPassword_EnabledFalseSendsZero(t *testing.T) {
	editForm := mustReadFixture(t, userEditFixture)
	var gotEnabled string
	mux := http.NewServeMux()
	mux.HandleFunc("/jury/users/42/edit", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_, _ = w.Write(editForm)
		case http.MethodPost:
			_ = r.ParseForm()
			gotEnabled = r.PostForm.Get("user[enabled]")
			http.Redirect(w, r, "/jury/users/42", http.StatusFound)
		}
	})
	mux.HandleFunc("/jury/users/42", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv)
	if err := c.SetUserPassword(context.Background(), "42", "pw", []int{1}, false); err != nil {
		t.Fatalf("SetUserPassword: %v", err)
	}
	if gotEnabled != "0" {
		t.Errorf("enabled = %q, want 0 (disabled)", gotEnabled)
	}
}

func TestSetUserPassword_UnspecifiedRolesPreservesTeamMember(t *testing.T) {
	editForm := mustReadFixture(t, userEditFixture)
	tests := []struct {
		name  string
		roles []int
	}{
		{name: "nil", roles: nil},
		{name: "empty", roles: []int{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var gotRoles []string
			mux := http.NewServeMux()
			mux.HandleFunc("/jury/users/42/edit", func(w http.ResponseWriter, r *http.Request) {
				switch r.Method {
				case http.MethodGet:
					_, _ = w.Write(editForm)
				case http.MethodPost:
					_ = r.ParseForm()
					gotRoles = r.PostForm["user[user_roles][]"]
					http.Redirect(w, r, "/jury/users/42", http.StatusFound)
				}
			})
			mux.HandleFunc("/jury/users/42", func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			})
			srv := httptest.NewServer(mux)
			defer srv.Close()

			c := newTestClient(t, srv)
			if err := c.SetUserPassword(context.Background(), "42", "pw", tc.roles, true); err != nil {
				t.Fatalf("SetUserPassword: %v", err)
			}
			want := []string{"3"}
			if !reflect.DeepEqual(gotRoles, want) {
				t.Errorf("user[user_roles][] = %v, want preserved Team Member %v", gotRoles, want)
			}
		})
	}
}

func TestSetUserPassword_StaysOnEdit_ReturnsErr(t *testing.T) {
	editForm := mustReadFixture(t, userEditFixture)
	mux := http.NewServeMux()
	mux.HandleFunc("/jury/users/42/edit", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(editForm)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv)
	err := c.SetUserPassword(context.Background(), "42", "pw", []int{1}, true)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, web.ErrUpdateFailed) {
		t.Errorf("errors.Is(err, web.ErrUpdateFailed) = false; err = %v", err)
	}
}

func TestSetUserPassword_500_WrapsServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	err := c.SetUserPassword(context.Background(), "42", "pw", []int{1}, true)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, web.ErrServerError) {
		t.Errorf("errors.Is(err, web.ErrServerError) = false; err = %v", err)
	}
}
