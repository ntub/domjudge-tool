package csvio_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ntub/domjudge-tool/internal/util/csvio"
)

var importHeader = []string{"username", "name", "email", "affiliation", "password"}

func TestStripBOM(t *testing.T) {
	cases := []struct {
		name string
		in   []byte
		want []byte
	}{
		{name: "strips leading UTF-8 BOM", in: []byte("\xEF\xBB\xBFusername,name"), want: []byte("username,name")},
		{name: "preserves non-BOM input", in: []byte("username,name"), want: []byte("username,name")},
		{name: "empty stays empty", in: []byte{}, want: []byte{}},
		{name: "only BOM becomes empty", in: []byte("\xEF\xBB\xBF"), want: []byte{}},
		{name: "BOM-like prefix but not full BOM kept", in: []byte("\xEFhi"), want: []byte("\xEFhi")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := csvio.StripBOM(tc.in)
			if !bytes.Equal(got, tc.want) {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestReadAll_HappyPath(t *testing.T) {
	in := strings.NewReader("username,name,email,affiliation,password\nntubuser,test,ntubuser@mail.com,ntub,testpassword\n")
	rows, err := csvio.ReadAll(in, importHeader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := [][]string{{"ntubuser", "test", "ntubuser@mail.com", "ntub", "testpassword"}}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("rows = %v, want %v", rows, want)
	}
}

func TestReadAll_StripsBOMFromHeader(t *testing.T) {
	path := filepath.Join("..", "..", "..", "testdata", "importer", "with_bom.csv")
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = f.Close() })
	rows, err := csvio.ReadAll(f, importHeader)
	if err != nil {
		t.Fatalf("ReadAll on BOM-prefixed input: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
}

func TestReadAll_RejectsHeaderMismatchOrder(t *testing.T) {
	in := strings.NewReader("name,username,email,affiliation,password\nfoo,bar,baz,qux,quux\n")
	_, err := csvio.ReadAll(in, importHeader)
	if !errors.Is(err, csvio.ErrHeaderMismatch) {
		t.Fatalf("err = %v, want ErrHeaderMismatch", err)
	}
	if !strings.Contains(err.Error(), "username") {
		t.Fatalf("error should name the offending column, got: %v", err)
	}
}

func TestReadAll_RejectsHeaderMismatchCount(t *testing.T) {
	in := strings.NewReader("username,name\nfoo,bar\n")
	_, err := csvio.ReadAll(in, importHeader)
	if !errors.Is(err, csvio.ErrHeaderMismatch) {
		t.Fatalf("err = %v, want ErrHeaderMismatch", err)
	}
}

func TestReadAll_EmptyInputErrors(t *testing.T) {
	_, err := csvio.ReadAll(strings.NewReader(""), importHeader)
	if err == nil {
		t.Fatal("expected error on empty input")
	}
}

// TestImportTemplateGolden_HeadersByteIdentical locks the current
// canonical import-users-teams template header. The first five columns
// preserve the Phase 1 Python-compatible order; category_id is optional
// and appended.
func TestImportTemplateGolden_HeadersByteIdentical(t *testing.T) {
	templateHeader := []string{"username", "name", "email", "affiliation", "password", "category_id"}
	path := filepath.Join("..", "..", "..", "testdata", "importer", "template_python_golden.csv")
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open template: %v", err)
	}
	t.Cleanup(func() { _ = f.Close() })
	rows, err := csvio.ReadAll(f, templateHeader)
	if err != nil {
		t.Fatalf("ReadAll on golden template: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1 (sample row)", len(rows))
	}
}

func TestWrite_RoundTrip(t *testing.T) {
	rows := [][]string{
		{"ntubuser", "test", "ntubuser@mail.com", "ntub", "testpassword"},
	}
	var buf bytes.Buffer
	if err := csvio.Write(&buf, importHeader, rows); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, err := csvio.ReadAll(&buf, importHeader)
	if err != nil {
		t.Fatalf("re-ReadAll: %v", err)
	}
	if !reflect.DeepEqual(got, rows) {
		t.Fatalf("round-trip drift: got %v, want %v", got, rows)
	}
}
