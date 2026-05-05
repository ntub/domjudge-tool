package v4

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const organizationsFixture = `[
  {"id":"1","shortname":"NTUB","name":"Sample University","country":"TWN"},
  {"id":"2","shortname":"NCKU","name":"Another University","country":"TWN"}
]`

func TestOrganizationsResource_List(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		if r.URL.Path != "/api/v4/contests/2/organizations" {
			t.Errorf("path = %q, want /api/v4/contests/2/organizations", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(organizationsFixture))
	}))
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL})
	orgs, err := c.Organizations.List(context.Background(), "2")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(orgs) != 2 {
		t.Fatalf("len(orgs) = %d, want 2", len(orgs))
	}
	if got, want := orgs[0].Country, "TWN"; got != want {
		t.Fatalf("Country = %q, want %q", got, want)
	}
}

func TestOrganizationsResource_Get(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4/contests/2/organizations/1" {
			t.Errorf("path = %q, want /api/v4/contests/2/organizations/1", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"1","shortname":"NTUB","name":"Sample University","country":"TWN"}`))
	}))
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL})
	org, err := c.Organizations.Get(context.Background(), "2", "1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if org == nil || org.Name != "Sample University" {
		t.Fatalf("org = %+v, want Sample University", org)
	}
}

func TestOrganizationsResource_FindByName(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(organizationsFixture))
	}))
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL})
	org, err := c.Organizations.FindByName(context.Background(), "2", "Sample University")
	if err != nil {
		t.Fatalf("FindByName: %v", err)
	}
	if org == nil || org.ID != "1" {
		t.Fatalf("org = %+v, want ID=1", org)
	}

	org, err = c.Organizations.FindByName(context.Background(), "2", "Missing University")
	if err != nil {
		t.Fatalf("FindByName missing: %v", err)
	}
	if org != nil {
		t.Fatalf("org = %+v, want nil", org)
	}
}

func TestOrganizationsResource_ListErrors(t *testing.T) {
	tests := []struct {
		name   string
		status int
		target error
	}{
		{name: "404", status: http.StatusNotFound, target: ErrNotFound},
		{name: "500", status: http.StatusInternalServerError, target: ErrServerError},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				http.Error(w, strings.ToLower(tc.name), tc.status)
			}))
			defer srv.Close()

			c := New(Options{BaseURL: srv.URL})
			_, err := c.Organizations.List(context.Background(), "2")
			if !errors.Is(err, tc.target) {
				t.Fatalf("err = %v, want %v", err, tc.target)
			}
		})
	}
}
