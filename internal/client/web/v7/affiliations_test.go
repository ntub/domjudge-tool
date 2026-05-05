package v7

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ntub/domjudge-tool/internal/client/web"
)

const (
	affiliationsListFixture     = "../../../../testdata/web/v7/jury_affiliations_list.html"
	affiliationsWithFlagFixture = "../../../../testdata/web/v7/affiliations_list_with_flag.html"
	tbodyEmptyFixture           = "../../../../testdata/web/v7/tbody_empty.html"
	tbodyMalformedFixture       = "../../../../testdata/web/v7/tbody_malformed.html"
)

// serveFixture returns a server that always serves the named file at any path.
func serveFixture(t *testing.T, path string) *httptest.Server {
	t.Helper()
	body := mustReadFixture(t, path)
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(body)
	}))
}

func TestGetAffiliations_HappyPath_TextFallback(t *testing.T) {
	srv := serveFixture(t, affiliationsListFixture)
	defer srv.Close()

	c := newTestClient(t, srv)

	got, err := c.GetAffiliations(context.Background())
	if err != nil {
		t.Fatalf("GetAffiliations: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	if got[0].ID == nil || *got[0].ID != "8" {
		t.Errorf("got[0].ID = %v, want 8", got[0].ID)
	}
	if got[0].ShortName != "course001" {
		t.Errorf("got[0].ShortName = %q, want course001", got[0].ShortName)
	}
	if got[0].Name != "course002" {
		t.Errorf("got[0].Name = %q, want course002", got[0].Name)
	}
	// Text-fallback branch: NTUB capture has no <img alt="...">.
	if got[0].Country != "TWN" {
		t.Errorf("got[0].Country = %q, want TWN (text-fallback)", got[0].Country)
	}
}

func TestGetAffiliations_CountryFromImgAlt(t *testing.T) {
	srv := serveFixture(t, affiliationsWithFlagFixture)
	defer srv.Close()

	c := newTestClient(t, srv)

	got, err := c.GetAffiliations(context.Background())
	if err != nil {
		t.Fatalf("GetAffiliations: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	if got[0].Country != "USA" {
		t.Errorf("got[0].Country = %q, want USA (img alt)", got[0].Country)
	}
	if got[1].Country != "JPN" {
		t.Errorf("got[1].Country = %q, want JPN (img alt)", got[1].Country)
	}
	// Locks Python parity: when <img> exists but has no alt attr,
	// Python's `.img.get("alt", "")` returns "". The Go port must NOT
	// fall back to the anchor's text in this case.
	if got[2].Country != "" {
		t.Errorf("got[2].Country = %q, want \"\" (img with no alt → empty per Python parity)", got[2].Country)
	}
}

func TestGetAffiliations_EmptyTable_ReturnsEmptySlice(t *testing.T) {
	srv := serveFixture(t, tbodyEmptyFixture)
	defer srv.Close()

	c := newTestClient(t, srv)

	got, err := c.GetAffiliations(context.Background())
	if err != nil {
		t.Fatalf("GetAffiliations: %v", err)
	}
	if got == nil {
		t.Fatalf("got nil, want empty slice")
	}
	if len(got) != 0 {
		t.Errorf("len = %d, want 0", len(got))
	}
}

func TestGetAffiliations_MalformedRow_ReturnsErrMalformedRow(t *testing.T) {
	srv := serveFixture(t, tbodyMalformedFixture)
	defer srv.Close()

	c := newTestClient(t, srv)

	_, err := c.GetAffiliations(context.Background())
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, web.ErrMalformedRow) {
		t.Errorf("errors.Is(err, web.ErrMalformedRow) = false; err = %v", err)
	}
}

func TestGetAffiliations_500_WrapsServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)

	_, err := c.GetAffiliations(context.Background())
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, web.ErrServerError) {
		t.Errorf("errors.Is(err, web.ErrServerError) = false; err = %v", err)
	}
}

func TestGetAffiliation_FoundByName(t *testing.T) {
	srv := serveFixture(t, affiliationsListFixture)
	defer srv.Close()

	c := newTestClient(t, srv)

	// Row 1: shortname=course001 name=course002
	got, err := c.GetAffiliation(context.Background(), "course002")
	if err != nil {
		t.Fatalf("GetAffiliation: %v", err)
	}
	if got == nil {
		t.Fatalf("got nil, want match by Name")
	}
	if got.Name != "course002" {
		t.Errorf("got Name = %q, want course002", got.Name)
	}
}

func TestGetAffiliation_FoundByShortname(t *testing.T) {
	srv := serveFixture(t, affiliationsListFixture)
	defer srv.Close()

	c := newTestClient(t, srv)

	// Row 2: shortname=course004 name=course005
	got, err := c.GetAffiliation(context.Background(), "course004")
	if err != nil {
		t.Fatalf("GetAffiliation: %v", err)
	}
	if got == nil {
		t.Fatalf("got nil, want match by ShortName")
	}
	if got.ShortName != "course004" {
		t.Errorf("got ShortName = %q, want course004", got.ShortName)
	}
}

func TestGetAffiliation_NotFound_ReturnsNilNilNil(t *testing.T) {
	srv := serveFixture(t, affiliationsListFixture)
	defer srv.Close()

	c := newTestClient(t, srv)

	got, err := c.GetAffiliation(context.Background(), "no-such-affiliation")
	if err != nil {
		t.Fatalf("GetAffiliation: %v", err)
	}
	if got != nil {
		t.Errorf("got %+v, want nil", got)
	}
}

func TestGetAffiliation_PropagatesListError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)

	_, err := c.GetAffiliation(context.Background(), "anything")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, web.ErrServerError) {
		t.Errorf("errors.Is(err, web.ErrServerError) = false; err = %v", err)
	}
}

func TestGetAffiliations_ContextCancelled(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-block:
		}
	}))
	defer srv.Close()
	defer close(block)

	c := newTestClient(t, srv)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.GetAffiliations(ctx)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("errors.Is(err, context.Canceled) = false; err = %v", err)
	}
}
