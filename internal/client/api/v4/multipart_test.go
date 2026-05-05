package v4

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPostMultipart_OneFile_BoundaryAndContentDisposition(t *testing.T) {
	const tsv = "id\tname\n1\tParticipants\n"
	wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("admin:secret"))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.Header.Get("Authorization") != wantAuth {
			t.Errorf("Authorization = %q, want %q", r.Header.Get("Authorization"), wantAuth)
		}
		if got := r.Header.Get("Content-Type"); !strings.HasPrefix(got, "multipart/form-data; boundary=") {
			t.Errorf("Content-Type = %q, want multipart boundary", got)
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("ParseMultipartForm: %v", err)
		}
		file, header, err := r.FormFile("tsv")
		if err != nil {
			t.Fatalf("FormFile(tsv): %v", err)
		}
		defer file.Close()
		if header.Filename != "groups.tsv" {
			t.Errorf("filename = %q, want groups.tsv", header.Filename)
		}
		body, err := io.ReadAll(file)
		if err != nil {
			t.Fatalf("ReadAll file: %v", err)
		}
		if string(body) != tsv {
			t.Errorf("file body = %q, want %q", string(body), tsv)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`["1"]`))
	}))
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL, Username: "admin", Password: "secret"})
	var ids []string
	err := c.postMultipart(context.Background(), "/users/groups", map[string]multipartField{
		"tsv": multipartFile{filename: "groups.tsv", contentType: "text/tab-separated-values", reader: strings.NewReader(tsv)},
	}, &ids)
	if err != nil {
		t.Fatalf("postMultipart: %v", err)
	}
	if len(ids) != 1 || ids[0] != "1" {
		t.Fatalf("ids = %v, want [1]", ids)
	}
}

func TestPostMultipart_MultiFieldsAndFile(t *testing.T) {
	const zipBody = "zip bytes"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("ParseMultipartForm: %v", err)
		}
		if got := r.FormValue("problem"); got != "boolfind" {
			t.Errorf("problem field = %q, want boolfind", got)
		}
		file, header, err := r.FormFile("zip[]")
		if err != nil {
			t.Fatalf("FormFile(zip[]): %v", err)
		}
		defer file.Close()
		if header.Filename != "boolfind.zip" {
			t.Errorf("filename = %q, want boolfind.zip", header.Filename)
		}
		body, err := io.ReadAll(file)
		if err != nil {
			t.Fatalf("ReadAll file: %v", err)
		}
		if string(body) != zipBody {
			t.Errorf("zip body = %q, want %q", string(body), zipBody)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"3"}`))
	}))
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL})
	var ids importIDs
	err := c.postMultipart(context.Background(), "/contests/2/problems", map[string]multipartField{
		"problem": multipartValue{value: "boolfind"},
		"zip[]":   multipartFile{filename: "boolfind.zip", contentType: "application/zip", reader: strings.NewReader(zipBody)},
	}, &ids)
	if err != nil {
		t.Fatalf("postMultipart: %v", err)
	}
	if len(ids) != 1 || ids[0] != "3" {
		t.Fatalf("ids = %v, want [3]", ids)
	}
}

func TestPostMultipart_ContextCanceled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := c.postMultipart(ctx, "/users/groups", map[string]multipartField{
		"tsv": multipartFile{filename: "groups.tsv", reader: strings.NewReader("id\tname\n")},
	}, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestPostMultipart_ServerErrorReturnsHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL})
	err := c.postMultipart(context.Background(), "/users/groups", map[string]multipartField{
		"tsv": multipartFile{filename: "groups.tsv", reader: strings.NewReader("id\tname\n")},
	}, nil)
	if !errors.Is(err, ErrServerError) {
		t.Fatalf("err = %v, want ErrServerError", err)
	}
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) || httpErr.StatusCode != http.StatusInternalServerError {
		t.Fatalf("err = %v, want HTTPError status 500", err)
	}
}

func TestPostMultipart_InvalidFieldRejectedBeforeRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected request: %s", r.URL.Path)
	}))
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL})
	err := c.postMultipart(context.Background(), "/users/groups", map[string]multipartField{
		"tsv": multipartFile{filename: "groups.tsv"},
	}, nil)
	if !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("err = %v, want ErrInvalidArgument", err)
	}
}
