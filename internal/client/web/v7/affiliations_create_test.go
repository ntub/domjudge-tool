package v7

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/ntub/domjudge-tool/internal/client/web"
)

const affiliationsAddFixture = "../../../../testdata/web/v7/jury_affiliations_add.html"

func TestCreateAffiliation_PostsAllRequiredFields(t *testing.T) {
	addForm := mustReadFixture(t, affiliationsAddFixture)

	var (
		gotShortname string
		gotName      string
		gotCountry   string
		gotComments  string
		seenComments atomic.Bool
	)
	mux := http.NewServeMux()
	mux.HandleFunc("/jury/affiliations/add", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(addForm)
		case http.MethodPost:
			_ = r.ParseForm()
			gotShortname = r.PostForm.Get("team_affiliation[shortname]")
			gotName = r.PostForm.Get("team_affiliation[name]")
			gotCountry = r.PostForm.Get("team_affiliation[country]")
			if _, present := r.PostForm["team_affiliation[comments]"]; present {
				seenComments.Store(true)
				gotComments = r.PostForm.Get("team_affiliation[comments]")
			}
			http.Redirect(w, r, "/jury/affiliations/42", http.StatusFound)
		}
	})
	mux.HandleFunc("/jury/affiliations/42", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv)

	got, err := c.CreateAffiliation(context.Background(), "acme", "Acme Corp", "USA")
	if err != nil {
		t.Fatalf("CreateAffiliation: %v", err)
	}
	if gotShortname != "acme" {
		t.Errorf("shortname = %q, want acme", gotShortname)
	}
	if gotName != "Acme Corp" {
		t.Errorf("name = %q, want Acme Corp", gotName)
	}
	if gotCountry != "USA" {
		t.Errorf("country = %q, want USA", gotCountry)
	}
	if !seenComments.Load() {
		t.Errorf("team_affiliation[comments] should be present in POST (Python sets it to '')")
	}
	if gotComments != "" {
		t.Errorf("comments = %q, want empty string", gotComments)
	}

	if got == nil {
		t.Fatalf("got nil affiliation")
	}
	if got.ID == nil || *got.ID != "42" {
		t.Errorf("ID = %v, want 42", got.ID)
	}
	if got.ShortName != "acme" {
		t.Errorf("ShortName = %q, want acme", got.ShortName)
	}
	if got.Name != "Acme Corp" {
		t.Errorf("Name = %q, want Acme Corp", got.Name)
	}
	if got.Country != "USA" {
		t.Errorf("Country = %q, want USA", got.Country)
	}
}

func TestCreateAffiliation_StaysOnAdd_ReturnsErrCreateFailed(t *testing.T) {
	addForm := mustReadFixture(t, affiliationsAddFixture)
	mux := http.NewServeMux()
	mux.HandleFunc("/jury/affiliations/add", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_, _ = w.Write(addForm)
		case http.MethodPost:
			// Server returns the same form again (validation failure).
			_, _ = w.Write(addForm)
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv)

	_, err := c.CreateAffiliation(context.Background(), "x", "y", "z")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, web.ErrCreateFailed) {
		t.Errorf("errors.Is(err, web.ErrCreateFailed) = false; err = %v", err)
	}
}

func TestCreateAffiliation_500_WrapsServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)

	_, err := c.CreateAffiliation(context.Background(), "x", "y", "z")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, web.ErrServerError) {
		t.Errorf("errors.Is(err, web.ErrServerError) = false; err = %v", err)
	}
}
