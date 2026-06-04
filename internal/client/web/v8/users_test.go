package v8

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"

	"github.com/ntub/domjudge-tool/internal/client/web"
)

const userEditFormFixture = "../../../../testdata/web/v8/user_edit_form.html"

func TestSetUserPassword_Integration(t *testing.T) {
	editForm := mustReadFixture(t, userEditFormFixture)

	tests := []struct {
		name        string
		roles       []int
		enabled     bool
		stayOnForm  bool
		loginOnGET  bool
		wantErr     error
		wantRoles   []string
		wantEnabled string
	}{
		{
			name:        "success posts password enabled and roles",
			roles:       []int{3, 5},
			enabled:     true,
			wantRoles:   []string{"3", "5"},
			wantEnabled: "1",
		},
		{
			name:        "nil roles preserves form roles",
			roles:       nil,
			enabled:     false,
			wantRoles:   []string{"3"},
			wantEnabled: "0",
		},
		{
			name:       "stayed on edit returns update failed",
			roles:      []int{3},
			enabled:    true,
			stayOnForm: true,
			wantErr:    web.ErrUpdateFailed,
		},
		{
			name:       "redirect to login returns login failed",
			roles:      []int{3},
			enabled:    true,
			loginOnGET: true,
			wantErr:    web.ErrLoginFailed,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var posted url.Values
			mux := http.NewServeMux()
			mux.HandleFunc("/jury/users/7/edit", func(w http.ResponseWriter, r *http.Request) {
				switch r.Method {
				case http.MethodGet:
					if tc.loginOnGET {
						http.Redirect(w, r, "/login", http.StatusFound)
						return
					}
					_, _ = w.Write(editForm)
				case http.MethodPost:
					_ = r.ParseForm()
					posted = cloneValues(r.PostForm)
					if tc.stayOnForm {
						_, _ = w.Write(editForm)
						return
					}
					http.Redirect(w, r, "/jury/users/7", http.StatusFound)
				default:
					t.Errorf("method = %s, want GET or POST", r.Method)
				}
			})
			mux.HandleFunc("/jury/users/7", func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			})
			mux.HandleFunc("/login", func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			})
			srv := httptest.NewServer(mux)
			defer srv.Close()

			c := newTestClient(t, srv)
			err := c.SetUserPassword(context.Background(), "7", "new-password", tc.roles, tc.enabled)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("errors.Is(err, %v) = false; err = %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("SetUserPassword: %v", err)
			}
			assertPostedValue(t, posted, "user[plainPassword]", "new-password")
			assertPostedValue(t, posted, "user[enabled]", tc.wantEnabled)
			if got := posted["user[user_roles][]"]; !reflect.DeepEqual(got, tc.wantRoles) {
				t.Errorf("user[user_roles][] = %v, want %v", got, tc.wantRoles)
			}
		})
	}
}

func TestUpdateUserEmail_Integration(t *testing.T) {
	editForm := mustReadFixture(t, userEditFormFixture)

	tests := []struct {
		name       string
		stayOnForm bool
		wantErr    error
	}{
		{name: "success posts only email override"},
		{name: "stayed on edit returns update failed", stayOnForm: true, wantErr: web.ErrUpdateFailed},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var posted url.Values
			mux := http.NewServeMux()
			mux.HandleFunc("/jury/users/7/edit", func(w http.ResponseWriter, r *http.Request) {
				switch r.Method {
				case http.MethodGet:
					_, _ = w.Write(editForm)
				case http.MethodPost:
					_ = r.ParseForm()
					posted = cloneValues(r.PostForm)
					if tc.stayOnForm {
						_, _ = w.Write(editForm)
						return
					}
					http.Redirect(w, r, "/jury/users/7", http.StatusFound)
				default:
					t.Errorf("method = %s, want GET or POST", r.Method)
				}
			})
			mux.HandleFunc("/jury/users/7", func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			})
			srv := httptest.NewServer(mux)
			defer srv.Close()

			c := newTestClient(t, srv)
			err := c.UpdateUserEmail(context.Background(), "7", "user1@example.org")
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("errors.Is(err, %v) = false; err = %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("UpdateUserEmail: %v", err)
			}
			assertPostedValue(t, posted, "user[email]", "user1@example.org")
		})
	}
}
