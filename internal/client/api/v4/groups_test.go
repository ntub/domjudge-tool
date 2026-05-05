package v4

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

const groupsFixture = `[
  {"id":"2","name":"Participants","sortorder":1,"hidden":false},
  {"id":"3","name":"Hidden","sortorder":2,"hidden":true}
]`

func TestGroupsResource_List(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4/contests/2/groups" {
			t.Errorf("path = %q, want /api/v4/contests/2/groups", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(groupsFixture))
	}))
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL})
	groups, err := c.Groups.List(context.Background(), "2")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(groups) != 2 {
		t.Fatalf("len(groups) = %d, want 2", len(groups))
	}
	if !groups[1].Hidden {
		t.Fatalf("groups[1].Hidden = false, want true")
	}
}

func TestGroupsResource_Get(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4/contests/2/groups/2" {
			t.Errorf("path = %q, want /api/v4/contests/2/groups/2", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"2","name":"Participants","sortorder":1,"hidden":false}`))
	}))
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL})
	group, err := c.Groups.Get(context.Background(), "2", "2")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if group == nil || group.Name != "Participants" {
		t.Fatalf("group = %+v, want Participants", group)
	}
}

func TestGroupsResource_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL})
	_, err := c.Groups.List(context.Background(), "2")
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("err = %v, want ErrUnauthorized", err)
	}
}
