package handler_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/ntub/domjudge-tool/internal/webui/db"
	"github.com/ntub/domjudge-tool/internal/webui/handler"
	"github.com/ntub/domjudge-tool/internal/webui/profile"
)

func newTestHandler(t *testing.T) http.Handler {
	t.Helper()
	h, _ := newTestHandlerWithStore(t)
	return h
}

func newTestHandlerWithStore(t *testing.T) (http.Handler, *profile.Store) {
	t.Helper()
	gdb, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		t.Fatalf("raw db: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	store, err := profile.NewStore(gdb)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	return handler.New(store), store
}

func TestProfiles_List_Empty(t *testing.T) {
	h := newTestHandler(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/profiles", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Server Profiles") {
		t.Error("body should contain 'Server Profiles'")
	}
}

func TestProfiles_New_Form(t *testing.T) {
	h := newTestHandler(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/profiles/new", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Add Server") {
		t.Error("body should contain 'Add Server'")
	}
}

func TestProfiles_Create_Redirect(t *testing.T) {
	h := newTestHandler(t)
	form := url.Values{
		"name":     {"prod"},
		"host":     {"https://test-station.example.org"},
		"username": {"admin"},
		"password": {"secret"},
		"version":  {"7.3.2"},
	}
	req := httptest.NewRequest(http.MethodPost, "/profiles",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("status: got %d, want 303", rec.Code)
	}
	if loc := rec.Header().Get("Location"); !strings.HasPrefix(loc, "/profiles") {
		t.Errorf("Location: got %q, want prefix /profiles", loc)
	}
}

func TestProfiles_Create_ListShows(t *testing.T) {
	h := newTestHandler(t)
	form := url.Values{
		"name":     {"prod"},
		"host":     {"https://test-station.example.org"},
		"username": {"admin"},
		"password": {"secret"},
	}
	req := httptest.NewRequest(http.MethodPost, "/profiles",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	h.ServeHTTP(httptest.NewRecorder(), req)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/profiles", nil))
	if !strings.Contains(rec.Body.String(), "prod") {
		t.Error("list should show created profile name")
	}
}

func TestProfiles_Delete_Redirect(t *testing.T) {
	h := newTestHandler(t)

	form := url.Values{
		"name": {"to-delete"}, "host": {"https://test-station.example.org"},
		"username": {"admin"}, "password": {"secret"},
	}
	req := httptest.NewRequest(http.MethodPost, "/profiles",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	h.ServeHTTP(httptest.NewRecorder(), req)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/profiles/1/delete", nil))
	if rec.Code != http.StatusSeeOther {
		t.Errorf("delete status: got %d, want 303", rec.Code)
	}
}

func TestProfiles_Edit_Form(t *testing.T) {
	h := newTestHandler(t)

	form := url.Values{
		"name": {"prod"}, "host": {"https://test-station.example.org"},
		"username": {"admin"}, "password": {"secret"},
	}
	req := httptest.NewRequest(http.MethodPost, "/profiles",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	h.ServeHTTP(httptest.NewRecorder(), req)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/profiles/1/edit", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "prod") {
		t.Error("edit form should pre-fill profile name")
	}
}

func TestProfiles_SetDefault(t *testing.T) {
	h, store := newTestHandlerWithStore(t)
	ctx := context.Background()
	var ids []int64
	for _, name := range []string{"first", "second"} {
		id, err := store.Create(ctx, profile.ServerProfile{Name: name, Host: "https://test-station.example.org"})
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		ids = append(ids, id)
	}
	for _, selected := range []int64{ids[0], ids[1], ids[0]} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, fmt.Sprintf("/profiles/%d/default", selected), nil))
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("set-default status = %d, want 303", rec.Code)
		}
		loc, err := rec.Result().Location()
		if err != nil {
			t.Fatalf("set-default location: %v", err)
		}
		if loc.Path != "/profiles" || loc.Query().Get("flash") != "default-set" || loc.Query().Get("kind") != "success" {
			t.Errorf("set-default redirect = %s", loc)
		}
		for _, id := range ids {
			p, err := store.Get(ctx, id)
			if err != nil {
				t.Fatalf("get profile %d: %v", id, err)
			}
			if p.IsDefault != (id == selected) {
				t.Errorf("profile %d default = %t, selected profile = %d", id, p.IsDefault, selected)
			}
		}
	}

	for _, path := range []string{"/profiles/invalid/default", "/profiles/invalid/delete"} {
		t.Run(path, func(t *testing.T) {
			before, err := store.List(ctx)
			if err != nil {
				t.Fatalf("profiles before invalid request: %v", err)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, nil))
			if rec.Code != http.StatusBadRequest {
				t.Errorf("invalid id status = %d, want 400", rec.Code)
			}
			after, err := store.List(ctx)
			if err != nil {
				t.Fatalf("profiles after invalid request: %v", err)
			}
			if !reflect.DeepEqual(after, before) {
				t.Errorf("invalid request mutated profiles: before=%+v after=%+v", before, after)
			}
		})
	}
}

func TestProfiles_Create_PersistsSubmittedFields(t *testing.T) {
	for _, disableSSL := range []bool{false, true} {
		t.Run(fmt.Sprintf("disable_ssl=%t", disableSSL), func(t *testing.T) {
			h, store := newTestHandlerWithStore(t)
			form := url.Values{
				"name": {"  contest server  "}, "host": {"  https://contest.example.org  "},
				"username": {"  operator  "}, "password": {" password with spaces "},
				"version": {"  8.3.1  "}, "contest_id": {"  contest-42  "},
			}
			if disableSSL {
				form.Set("disable_ssl", "on")
			}
			req := httptest.NewRequest(http.MethodPost, "/profiles", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusSeeOther {
				t.Fatalf("create status = %d, want 303; body=%s", rec.Code, rec.Body.String())
			}
			loc, err := rec.Result().Location()
			if err != nil {
				t.Fatalf("create redirect: %v", err)
			}
			if loc.Path != "/profiles" || loc.Query().Get("flash") != "created" || loc.Query().Get("kind") != "success" {
				t.Errorf("create redirect = %s", loc)
			}
			profiles, err := store.List(context.Background())
			if err != nil || len(profiles) != 1 {
				t.Fatalf("created profiles = %+v, err = %v; want one profile", profiles, err)
			}
			contestID := "contest-42"
			assertSubmittedProfile(t, profiles[0], profile.ServerProfile{
				Name: "contest server", Host: "https://contest.example.org", Username: "operator",
				Password: " password with spaces ", Version: "8.3.1", ContestID: &contestID, DisableSSL: disableSSL,
			})
		})
	}
}

func TestProfiles_Create_ValidationError(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		code int
	}{
		{name: "duplicate name", body: url.Values{
			"name": {"retained"}, "host": {"https://replacement.example.org"},
			"username": {"replacement"}, "password": {"replacement"},
		}.Encode(), code: http.StatusOK},
		{name: "malformed form", body: "name=%zz", code: http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, store := newTestHandlerWithStore(t)
			ctx := context.Background()
			_, err := store.Create(ctx, profile.ServerProfile{
				Name: "retained", Host: "https://original.example.org", Username: "original", Password: "original",
			})
			if err != nil {
				t.Fatalf("seed profile: %v", err)
			}
			before, err := store.List(ctx)
			if err != nil {
				t.Fatalf("profiles before create: %v", err)
			}
			req := httptest.NewRequest(http.MethodPost, "/profiles", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tc.code {
				t.Errorf("create status = %d, want %d", rec.Code, tc.code)
			}
			if tc.code == http.StatusOK {
				body := rec.Body.String()
				handlerTag(t, body, "form", "action", "/profiles")
				alert := handlerTag(t, body, "div", "role", "alert")
				if !strings.Contains(" "+alert["class"]+" ", " alert-error ") {
					t.Errorf("create error alert classification = %q", alert["class"])
				}
				host := handlerTag(t, body, "input", "name", "host")
				if host["value"] != "https://replacement.example.org" {
					t.Errorf("rejected form host = %q, want submitted host", host["value"])
				}
			}
			after, err := store.List(ctx)
			if err != nil {
				t.Fatalf("profiles after rejected create: %v", err)
			}
			if !reflect.DeepEqual(after, before) {
				t.Errorf("rejected create mutated profiles: before=%+v after=%+v", before, after)
			}
		})
	}
}

func TestProfiles_Edit_Errors(t *testing.T) {
	h := newTestHandler(t)

	rec1 := httptest.NewRecorder()
	h.ServeHTTP(rec1, httptest.NewRequest(http.MethodGet, "/profiles/bad/edit", nil))
	if rec1.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec1.Code)
	}

	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/profiles/999/edit", nil))
	if rec2.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", rec2.Code)
	}
}

func TestProfiles_Update(t *testing.T) {
	h, store := newTestHandlerWithStore(t)
	ctx := context.Background()
	firstID, err := store.Create(ctx, profile.ServerProfile{
		Name: "original", Host: "https://original.example.org", Username: "original", Password: "original",
	})
	if err != nil {
		t.Fatalf("seed first profile: %v", err)
	}
	secondID, err := store.Create(ctx, profile.ServerProfile{
		Name: "other", Host: "https://other.example.org", Username: "other", Password: "other",
	})
	if err != nil {
		t.Fatalf("seed second profile: %v", err)
	}
	contestID := "contest-7"
	for _, tc := range []struct {
		name string
		form url.Values
		want profile.ServerProfile
	}{
		{
			name: "replace fields and enable insecure SSL",
			form: url.Values{
				"name": {"  renamed  "}, "host": {"  https://updated.example.org  "},
				"username": {"  updated  "}, "password": {" updated secret "},
				"version": {"  8.3.1  "}, "contest_id": {"  contest-7  "}, "disable_ssl": {"on"},
			},
			want: profile.ServerProfile{
				Name: "renamed", Host: "https://updated.example.org", Username: "updated",
				Password: " updated secret ", Version: "8.3.1", DisableSSL: true, ContestID: &contestID,
			},
		},
		{
			name: "clear optional contest and disable insecure SSL",
			form: url.Values{
				"name": {"renamed"}, "host": {"https://updated.example.org"},
				"username": {"updated"}, "password": {"another secret"}, "version": {"7.3.2"}, "contest_id": {"  "},
			},
			want: profile.ServerProfile{
				Name: "renamed", Host: "https://updated.example.org", Username: "updated",
				Password: "another secret", Version: "7.3.2",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			otherBefore, err := store.Get(ctx, secondID)
			if err != nil {
				t.Fatalf("get unrelated profile: %v", err)
			}
			req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/profiles/%d", firstID), strings.NewReader(tc.form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusSeeOther {
				t.Fatalf("update status = %d, want 303", rec.Code)
			}
			loc, err := rec.Result().Location()
			if err != nil {
				t.Fatalf("update redirect: %v", err)
			}
			if loc.Path != "/profiles" || loc.Query().Get("flash") != "saved" || loc.Query().Get("kind") != "success" {
				t.Errorf("update redirect = %s", loc)
			}
			got, err := store.Get(ctx, firstID)
			if err != nil {
				t.Fatalf("read updated profile: %v", err)
			}
			assertSubmittedProfile(t, got, tc.want)
			otherAfter, err := store.Get(ctx, secondID)
			if err != nil {
				t.Fatalf("read unrelated profile: %v", err)
			}
			if !reflect.DeepEqual(otherAfter, otherBefore) {
				t.Errorf("update mutated unrelated profile: before=%+v after=%+v", otherBefore, otherAfter)
			}
		})
	}

	for _, tc := range []struct {
		name string
		path string
		body string
		code int
	}{
		{name: "invalid id", path: "/profiles/bad", body: "name=rejected", code: http.StatusBadRequest},
		{name: "malformed form", path: fmt.Sprintf("/profiles/%d", firstID), body: "name=%zz", code: http.StatusBadRequest},
		{name: "duplicate name", path: fmt.Sprintf("/profiles/%d", secondID), body: url.Values{
			"name": {"renamed"}, "host": {"https://rejected.example.org"}, "password": {"rejected"},
		}.Encode(), code: http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before, err := store.List(ctx)
			if err != nil {
				t.Fatalf("profiles before rejected update: %v", err)
			}
			req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tc.code {
				t.Errorf("update status = %d, want %d", rec.Code, tc.code)
			}
			if tc.code == http.StatusOK {
				handlerTag(t, rec.Body.String(), "form", "action", tc.path)
				alert := handlerTag(t, rec.Body.String(), "div", "role", "alert")
				if !strings.Contains(" "+alert["class"]+" ", " alert-error ") {
					t.Errorf("update error alert classification = %q", alert["class"])
				}
			}
			after, err := store.List(ctx)
			if err != nil {
				t.Fatalf("profiles after rejected update: %v", err)
			}
			if !reflect.DeepEqual(after, before) {
				t.Errorf("rejected update mutated profiles: before=%+v after=%+v", before, after)
			}
		})
	}
}

func assertSubmittedProfile(t *testing.T, got, want profile.ServerProfile) {
	t.Helper()
	for field, values := range map[string][2]string{
		"name": {got.Name, want.Name}, "host": {got.Host, want.Host},
		"username": {got.Username, want.Username}, "password": {got.Password, want.Password},
		"version": {got.Version, want.Version},
	} {
		if values[0] != values[1] {
			t.Errorf("profile %s = %q, want %q", field, values[0], values[1])
		}
	}
	if got.DisableSSL != want.DisableSSL {
		t.Errorf("profile DisableSSL = %t, want %t", got.DisableSSL, want.DisableSSL)
	}
	if !reflect.DeepEqual(got.ContestID, want.ContestID) {
		t.Errorf("profile ContestID = %v, want %v", got.ContestID, want.ContestID)
	}
}
