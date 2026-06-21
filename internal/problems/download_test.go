package problems_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/ntub/domjudge-tool/internal/client/web"
	"github.com/ntub/domjudge-tool/internal/model"
	"github.com/ntub/domjudge-tool/internal/problems"
	"github.com/ntub/domjudge-tool/internal/util/progress"
)

// stubGateway implements web.Gateway for the problems package tests.
// Only GetProblems and Download are wired; others return zero values.
type stubGateway struct {
	items       []model.ProblemItem
	itemsErr    error
	bodies      map[string][]byte
	downloadErr error
	downloads   []string
}

var _ web.Gateway = (*stubGateway)(nil)

func (s *stubGateway) Login(_ context.Context) error { return nil }

func (s *stubGateway) GetAffiliations(_ context.Context) ([]model.Affiliation, error) {
	return nil, nil
}

func (s *stubGateway) GetAffiliation(_ context.Context, _ string) (*model.Affiliation, error) {
	return nil, nil
}

func (s *stubGateway) CreateAffiliation(_ context.Context, _, _, _ string) (*model.Affiliation, error) {
	return nil, nil
}

func (s *stubGateway) GetProblems(_ context.Context, _ web.ProblemFilter) ([]model.ProblemItem, error) {
	return s.items, s.itemsErr
}

func (s *stubGateway) SetUserPassword(_ context.Context, _, _ string, _ []int, _ bool) error {
	return nil
}

func (s *stubGateway) UpdateUserEmail(_ context.Context, _, _ string) error { return nil }

func (s *stubGateway) DeleteUsers(_ context.Context, _, _ []string) error { return nil }

func (s *stubGateway) CreateTeamAndUser(_ context.Context, _ model.CreateUser, _, _ int, _ bool) (string, string, error) {
	return "", "", nil
}

func (s *stubGateway) UpdateTeam(_ context.Context, _ model.User, _, _ int, _ bool) (string, string, error) {
	return "", "", nil
}

func (s *stubGateway) DeleteTeams(_ context.Context, _, _ []string) error { return nil }

func (s *stubGateway) Download(_ context.Context, path string) (io.ReadCloser, error) {
	s.downloads = append(s.downloads, path)
	if s.downloadErr != nil {
		return nil, s.downloadErr
	}
	body, ok := s.bodies[path]
	if !ok {
		return nil, fmt.Errorf("stub: no body for %s", path)
	}
	return io.NopCloser(bytes.NewReader(body)), nil
}

// nopReporter satisfies progress.Reporter without output.
type nopReporter struct{ ticks int }

func (n *nopReporter) Start(_ int, _ string) {}
func (n *nopReporter) Tick()                 { n.ticks++ }
func (n *nopReporter) Finish()               {}

var _ progress.Reporter = (*nopReporter)(nil)

func exportPath(s string) *string { return &s }

func TestDownload_HappyPath(t *testing.T) {
	dir := t.TempDir()
	path := "/jury/problem/1/export"
	gw := &stubGateway{
		items: []model.ProblemItem{
			{ID: "1", Name: "Hello World", ExportFilePath: exportPath(path)},
		},
		bodies: map[string][]byte{path: []byte("ZIP_DATA")},
	}
	rep := &nopReporter{}
	res, err := problems.Download(context.Background(), gw, web.ProblemFilter{}, dir, rep)
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	if res.Downloaded != 1 {
		t.Errorf("Downloaded=%d, want 1", res.Downloaded)
	}
	if res.Skipped != 0 {
		t.Errorf("Skipped=%d, want 0", res.Skipped)
	}
	if rep.ticks != 1 {
		t.Errorf("Tick calls=%d, want 1", rep.ticks)
	}
	filename := filepath.Join(dir, "1_Hello-World.zip")
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}
	if string(data) != "ZIP_DATA" {
		t.Errorf("file content=%q, want ZIP_DATA", data)
	}
}

func TestDownload_SkipsNilExportPath(t *testing.T) {
	dir := t.TempDir()
	gw := &stubGateway{
		items: []model.ProblemItem{
			{ID: "1", Name: "NoExport", ExportFilePath: nil},
		},
		bodies: map[string][]byte{},
	}
	rep := &nopReporter{}
	res, err := problems.Download(context.Background(), gw, web.ProblemFilter{}, dir, rep)
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	if res.Downloaded != 0 {
		t.Errorf("Downloaded=%d, want 0", res.Downloaded)
	}
	if res.Skipped != 1 {
		t.Errorf("Skipped=%d, want 1", res.Skipped)
	}
	if rep.ticks != 0 {
		t.Errorf("Tick calls=%d, want 0 (skipped items don't tick)", rep.ticks)
	}
}

func TestDownload_OutDirIsAbsolute(t *testing.T) {
	dir := t.TempDir()
	gw := &stubGateway{items: nil, bodies: map[string][]byte{}}
	rep := &nopReporter{}
	res, err := problems.Download(context.Background(), gw, web.ProblemFilter{}, dir, rep)
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	if !filepath.IsAbs(res.OutDir) {
		t.Errorf("OutDir=%q is not absolute", res.OutDir)
	}
}

func TestDownload_ListError(t *testing.T) {
	dir := t.TempDir()
	gw := &stubGateway{itemsErr: fmt.Errorf("network error")}
	rep := &nopReporter{}
	_, err := problems.Download(context.Background(), gw, web.ProblemFilter{}, dir, rep)
	if err == nil {
		t.Fatal("want error, got nil")
	}
}

func TestDownload_DownloadError(t *testing.T) {
	dir := t.TempDir()
	path := "/jury/problem/1/export"
	gw := &stubGateway{
		items:       []model.ProblemItem{{ID: "1", Name: "P", ExportFilePath: exportPath(path)}},
		downloadErr: fmt.Errorf("download failed"),
		bodies:      map[string][]byte{},
	}
	rep := &nopReporter{}
	_, err := problems.Download(context.Background(), gw, web.ProblemFilter{}, dir, rep)
	if err == nil {
		t.Fatal("want error, got nil")
	}
}

func TestDownload_FilenameSpacesToDash(t *testing.T) {
	dir := t.TempDir()
	path := "/jury/problem/2/export"
	gw := &stubGateway{
		items: []model.ProblemItem{
			{ID: "2", Name: "Two Words", ExportFilePath: exportPath(path)},
		},
		bodies: map[string][]byte{path: []byte("data")},
	}
	rep := &nopReporter{}
	_, err := problems.Download(context.Background(), gw, web.ProblemFilter{}, dir, rep)
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	want := filepath.Join(dir, "2_Two-Words.zip")
	if _, err := os.Stat(want); err != nil {
		t.Errorf("expected file %s: %v", want, err)
	}
}
