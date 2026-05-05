package submissions_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	apiv4 "github.com/ntub/domjudge-tool/internal/client/api/v4"
	"github.com/ntub/domjudge-tool/internal/model"
	"github.com/ntub/domjudge-tool/internal/submissions"
)

// fixture sets up a routed httptest.Server returning the supplied
// indexes + per-id submission zip body. Returns the server (caller
// closes), the configured *apiv4.Client, and a per-id download counter.
type fixture struct {
	srv          *httptest.Server
	api          *apiv4.Client
	downloadHits *atomic.Int32
}

type fakeContest struct {
	Teams          []model.Team
	Problems       []model.Problem
	Submissions    []model.Submission
	JudgementTypes []model.JudgementType
	Judgements     []model.Judgement
	// Per-submission zip body indexed by submission ID. Missing entries
	// trigger a 500.
	ZipBodyByID map[string][]byte
	// 500-status submissions for the abort-on-error tests.
	ServerErrorByID map[string]bool
}

func newFixture(t *testing.T, c fakeContest) *fixture {
	t.Helper()
	if c.ZipBodyByID == nil {
		c.ZipBodyByID = make(map[string][]byte)
	}
	if c.ServerErrorByID == nil {
		c.ServerErrorByID = make(map[string]bool)
	}

	hits := &atomic.Int32{}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/contests/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/v4/contests/contestX")
		switch {
		case path == "/teams":
			writeJSON(w, c.Teams)
		case path == "/problems":
			writeJSON(w, c.Problems)
		case path == "/submissions":
			writeJSON(w, c.Submissions)
		case path == "/judgement-types":
			writeJSON(w, c.JudgementTypes)
		case path == "/judgements":
			writeJSON(w, c.Judgements)
		case strings.HasPrefix(path, "/submissions/") && strings.HasSuffix(path, "/files"):
			id := strings.TrimSuffix(strings.TrimPrefix(path, "/submissions/"), "/files")
			if c.ServerErrorByID[id] {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			body, ok := c.ZipBodyByID[id]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			hits.Add(1)
			w.Header().Set("Content-Type", "application/zip")
			w.Header().Set("Content-Disposition", `attachment; filename="server.zip"`)
			_, _ = w.Write(body)
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	})
	srv := httptest.NewServer(mux)
	api := apiv4.New(apiv4.Options{BaseURL: srv.URL, Username: "u", Password: "p"})

	return &fixture{srv: srv, api: api, downloadHits: hits}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// makeContestZip returns a zip with one inner file `${name}.txt` so the
// extract path has something to rename.
func makeContestZip(t *testing.T, content string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("source.txt")
	if err != nil {
		t.Fatalf("create entry: %v", err)
	}
	if _, err := w.Write([]byte(content)); err != nil {
		t.Fatalf("write entry: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zw: %v", err)
	}
	return buf.Bytes()
}

func TestDownload_HappyPath_Mode2(t *testing.T) {
	dst := t.TempDir()
	zipBody := makeContestZip(t, "submission-1")
	teams := []model.Team{{ID: "100", Name: "alpha"}}
	probs := []model.Problem{{ID: "P1", ShortName: "easy"}}
	jtypes := []model.JudgementType{{ID: "JT-AC", Name: "Accepted"}}
	jtAC := "JT-AC"
	subs := []model.Submission{{ID: "S1", TeamID: "100", ProblemID: "P1"}}
	jdgs := []model.Judgement{{ID: "J1", SubmissionID: "S1", JudgementTypeID: &jtAC}}

	f := newFixture(t, fakeContest{
		Teams: teams, Problems: probs, Submissions: subs,
		JudgementTypes: jtypes, Judgements: jdgs,
		ZipBodyByID: map[string][]byte{"S1": zipBody},
	})
	defer f.srv.Close()

	count, err := submissions.Download(context.Background(), f.api, "contestX", submissions.Options{
		Mode:       2,
		OutputPath: dst,
	})
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}

	want := filepath.Join(dst, "problem_easy", "team_alpha", "accepted_S1.zip")
	if _, err := os.Stat(want); err != nil {
		t.Errorf("expected zip at %s: %v", want, err)
	}
}

func TestDownload_Mode1_TeamFirst(t *testing.T) {
	dst := t.TempDir()
	zipBody := makeContestZip(t, "submission-1")
	jtAC := "JT-AC"
	f := newFixture(t, fakeContest{
		Teams:          []model.Team{{ID: "1", Name: "blue"}},
		Problems:       []model.Problem{{ID: "P", ShortName: "hello"}},
		Submissions:    []model.Submission{{ID: "S", TeamID: "1", ProblemID: "P"}},
		JudgementTypes: []model.JudgementType{{ID: "JT-AC", Name: "Accepted"}},
		Judgements:     []model.Judgement{{ID: "J", SubmissionID: "S", JudgementTypeID: &jtAC}},
		ZipBodyByID:    map[string][]byte{"S": zipBody},
	})
	defer f.srv.Close()

	if _, err := submissions.Download(context.Background(), f.api, "contestX", submissions.Options{
		Mode: 1, OutputPath: dst,
	}); err != nil {
		t.Fatalf("Download: %v", err)
	}
	want := filepath.Join(dst, "team_blue", "problem_hello", "accepted_S.zip")
	if _, err := os.Stat(want); err != nil {
		t.Errorf("expected zip at %s: %v", want, err)
	}
}

func TestDownload_ModeOther_ContestFolder(t *testing.T) {
	dst := t.TempDir()
	zipBody := makeContestZip(t, "submission-1")
	jtAC := "JT-AC"
	f := newFixture(t, fakeContest{
		Teams:          []model.Team{{ID: "1", Name: "blue"}},
		Problems:       []model.Problem{{ID: "P", ShortName: "hello"}},
		Submissions:    []model.Submission{{ID: "S", TeamID: "1", ProblemID: "P"}},
		JudgementTypes: []model.JudgementType{{ID: "JT-AC", Name: "Accepted"}},
		Judgements:     []model.Judgement{{ID: "J", SubmissionID: "S", JudgementTypeID: &jtAC}},
		ZipBodyByID:    map[string][]byte{"S": zipBody},
	})
	defer f.srv.Close()

	if _, err := submissions.Download(context.Background(), f.api, "contestX", submissions.Options{
		Mode: 99, OutputPath: dst,
	}); err != nil {
		t.Fatalf("Download: %v", err)
	}
	want := filepath.Join(dst, "contest_contestX", "accepted_S.zip")
	if _, err := os.Stat(want); err != nil {
		t.Errorf("expected zip at %s: %v", want, err)
	}
}

func TestDownload_SkipsMissingTeamOrProblem(t *testing.T) {
	dst := t.TempDir()
	jtAC := "JT-AC"
	f := newFixture(t, fakeContest{
		Teams:          []model.Team{{ID: "1", Name: "blue"}},
		Problems:       []model.Problem{{ID: "P", ShortName: "hello"}},
		JudgementTypes: []model.JudgementType{{ID: "JT-AC", Name: "Accepted"}},
		Judgements:     []model.Judgement{{ID: "J", SubmissionID: "S-bad", JudgementTypeID: &jtAC}},
		Submissions: []model.Submission{
			// Missing team:
			{ID: "S-bad", TeamID: "999", ProblemID: "P"},
		},
		ZipBodyByID: map[string][]byte{},
	})
	defer f.srv.Close()

	count, err := submissions.Download(context.Background(), f.api, "contestX", submissions.Options{
		Mode: 2, OutputPath: dst,
	})
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	if count != 0 {
		t.Errorf("count = %d, want 0 (submission skipped)", count)
	}
}

func TestDownload_AbortsOnDownloadError(t *testing.T) {
	dst := t.TempDir()
	zipBody := makeContestZip(t, "submission-1")
	jtAC := "JT-AC"
	f := newFixture(t, fakeContest{
		Teams:          []model.Team{{ID: "1", Name: "blue"}},
		Problems:       []model.Problem{{ID: "P", ShortName: "hello"}},
		JudgementTypes: []model.JudgementType{{ID: "JT-AC", Name: "Accepted"}},
		Submissions: []model.Submission{
			{ID: "ok-1", TeamID: "1", ProblemID: "P"},
			{ID: "broken", TeamID: "1", ProblemID: "P"},
			{ID: "ok-2", TeamID: "1", ProblemID: "P"},
		},
		Judgements: []model.Judgement{
			{ID: "J1", SubmissionID: "ok-1", JudgementTypeID: &jtAC},
			{ID: "J2", SubmissionID: "broken", JudgementTypeID: &jtAC},
			{ID: "J3", SubmissionID: "ok-2", JudgementTypeID: &jtAC},
		},
		ZipBodyByID: map[string][]byte{
			"ok-1":   zipBody,
			"ok-2":   zipBody,
			"broken": zipBody,
		},
		ServerErrorByID: map[string]bool{"broken": true},
	})
	defer f.srv.Close()

	_, err := submissions.Download(context.Background(), f.api, "contestX", submissions.Options{
		Mode: 2, OutputPath: dst,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestDownload_Extract_RenamesAndRemovesZip(t *testing.T) {
	dst := t.TempDir()
	zipBody := makeContestZip(t, "src-content")
	jtAC := "JT-AC"
	f := newFixture(t, fakeContest{
		Teams:          []model.Team{{ID: "1", Name: "blue"}},
		Problems:       []model.Problem{{ID: "P", ShortName: "hello"}},
		Submissions:    []model.Submission{{ID: "S", TeamID: "1", ProblemID: "P"}},
		JudgementTypes: []model.JudgementType{{ID: "JT-AC", Name: "Accepted"}},
		Judgements:     []model.Judgement{{ID: "J", SubmissionID: "S", JudgementTypeID: &jtAC}},
		ZipBodyByID:    map[string][]byte{"S": zipBody},
	})
	defer f.srv.Close()

	if _, err := submissions.Download(context.Background(), f.api, "contestX", submissions.Options{
		Mode: 2, OutputPath: dst, Extract: true,
	}); err != nil {
		t.Fatalf("Download: %v", err)
	}
	folder := filepath.Join(dst, "problem_hello", "team_blue")
	wantSrc := filepath.Join(folder, "accepted_S.txt")
	if _, err := os.Stat(wantSrc); err != nil {
		t.Errorf("expected extracted file at %s: %v", wantSrc, err)
	}
	zipPath := filepath.Join(folder, "accepted_S.zip")
	if _, err := os.Stat(zipPath); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("zip should be removed after extract; stat err = %v", err)
	}
}

func TestDownload_CtxCancelled(t *testing.T) {
	dst := t.TempDir()
	jtAC := "JT-AC"
	f := newFixture(t, fakeContest{
		Teams:          []model.Team{{ID: "1", Name: "blue"}},
		Problems:       []model.Problem{{ID: "P", ShortName: "hello"}},
		Submissions:    []model.Submission{{ID: "S", TeamID: "1", ProblemID: "P"}},
		JudgementTypes: []model.JudgementType{{ID: "JT-AC", Name: "Accepted"}},
		Judgements:     []model.Judgement{{ID: "J", SubmissionID: "S", JudgementTypeID: &jtAC}},
	})
	defer f.srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := submissions.Download(ctx, f.api, "contestX", submissions.Options{
		Mode: 2, OutputPath: dst,
	})
	if err == nil {
		t.Fatal("expected ctx cancelled error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("errors.Is(err, context.Canceled) = false; err = %v", err)
	}
}

func TestDownload_PathPrefix(t *testing.T) {
	tmp := t.TempDir()
	dst := filepath.Join(tmp, "mygrading")
	zipBody := makeContestZip(t, "src")
	jtAC := "JT-AC"
	f := newFixture(t, fakeContest{
		Teams:          []model.Team{{ID: "1", Name: "blue"}},
		Problems:       []model.Problem{{ID: "P", ShortName: "hello"}},
		Submissions:    []model.Submission{{ID: "S", TeamID: "1", ProblemID: "P"}},
		JudgementTypes: []model.JudgementType{{ID: "JT-AC", Name: "Accepted"}},
		Judgements:     []model.Judgement{{ID: "J", SubmissionID: "S", JudgementTypeID: &jtAC}},
		ZipBodyByID:    map[string][]byte{"S": zipBody},
	})
	defer f.srv.Close()
	if _, err := submissions.Download(context.Background(), f.api, "contestX", submissions.Options{
		Mode: 2, OutputPath: dst,
	}); err != nil {
		t.Fatalf("Download: %v", err)
	}
	want := filepath.Join(dst, "problem_hello", "team_blue", "accepted_S.zip")
	if _, err := os.Stat(want); err != nil {
		t.Errorf("expected zip at %s: %v", want, err)
	}
}
