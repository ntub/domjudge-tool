package submissions_test

import (
	"context"
	"testing"

	"github.com/ntub/domjudge-tool/internal/model"
	"github.com/ntub/domjudge-tool/internal/submissions"
)

// recordingReporter satisfies progress.Reporter structurally so this
// file does not import the progress package directly. Tracks Start /
// Tick / Finish counts so tests can assert call shape.
type recordingReporter struct {
	starts   int
	ticks    int
	finishes int
	total    int
}

func (r *recordingReporter) Start(total int, _ string) {
	r.starts++
	r.total = total
}
func (r *recordingReporter) Tick()   { r.ticks++ }
func (r *recordingReporter) Finish() { r.finishes++ }

// TestSubmissionsDownload_Reporter_TicksOncePerSubmission asserts that
// the Reporter sees one Tick per successfully-downloaded submission and
// one Start sized to the full submissions list. Skipped (not-in-index)
// submissions must NOT bump the bar — that is the behaviour the spec
// pins ("only successful rows").
func TestSubmissionsDownload_Reporter_TicksOncePerSubmission(t *testing.T) {
	dst := t.TempDir()
	zipBody := makeContestZip(t, "submission-1")
	jtAC := "JT-AC"
	teams := []model.Team{{ID: "1", Name: "alpha"}}
	probs := []model.Problem{{ID: "P", ShortName: "easy"}}
	jtypes := []model.JudgementType{{ID: "JT-AC", Name: "Accepted"}}
	subs := []model.Submission{
		{ID: "S1", TeamID: "1", ProblemID: "P"},
		{ID: "S2", TeamID: "1", ProblemID: "P"},
		{ID: "S3", TeamID: "1", ProblemID: "P"},
	}
	jdgs := []model.Judgement{
		{ID: "J1", SubmissionID: "S1", JudgementTypeID: &jtAC},
		{ID: "J2", SubmissionID: "S2", JudgementTypeID: &jtAC},
		{ID: "J3", SubmissionID: "S3", JudgementTypeID: &jtAC},
	}

	f := newFixture(t, fakeContest{
		Teams: teams, Problems: probs, Submissions: subs,
		JudgementTypes: jtypes, Judgements: jdgs,
		ZipBodyByID: map[string][]byte{
			"S1": zipBody, "S2": zipBody, "S3": zipBody,
		},
	})
	defer f.srv.Close()

	rep := &recordingReporter{}
	count, err := submissions.Download(context.Background(), f.api, "contestX", submissions.Options{
		Mode:       2,
		OutputPath: dst,
		Reporter:   rep,
	})
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	if count != 3 {
		t.Fatalf("count = %d, want 3", count)
	}
	if rep.starts != 1 {
		t.Errorf("starts = %d, want 1", rep.starts)
	}
	if rep.total != len(subs) {
		t.Errorf("Start total = %d, want %d", rep.total, len(subs))
	}
	if rep.ticks != count {
		t.Errorf("ticks = %d, want %d (one per successful submission)", rep.ticks, count)
	}
	if rep.finishes != 1 {
		t.Errorf("finishes = %d, want 1", rep.finishes)
	}
}

// TestSubmissionsDownload_Reporter_SkippedDoesNotTick asserts that
// submissions whose team/problem are not in the contest index are
// skipped silently AND do not Tick — the bar reflects "successfully
// downloaded" rather than "iterations started".
func TestSubmissionsDownload_Reporter_SkippedDoesNotTick(t *testing.T) {
	dst := t.TempDir()
	zipBody := makeContestZip(t, "ok")
	jtAC := "JT-AC"
	teams := []model.Team{{ID: "1", Name: "alpha"}}
	probs := []model.Problem{{ID: "P", ShortName: "easy"}}
	jtypes := []model.JudgementType{{ID: "JT-AC", Name: "Accepted"}}
	subs := []model.Submission{
		{ID: "S-ok", TeamID: "1", ProblemID: "P"},
		// Missing-team submission — must be skipped.
		{ID: "S-skip", TeamID: "999", ProblemID: "P"},
	}
	jdgs := []model.Judgement{
		{ID: "J1", SubmissionID: "S-ok", JudgementTypeID: &jtAC},
		{ID: "J2", SubmissionID: "S-skip", JudgementTypeID: &jtAC},
	}
	f := newFixture(t, fakeContest{
		Teams: teams, Problems: probs, Submissions: subs,
		JudgementTypes: jtypes, Judgements: jdgs,
		ZipBodyByID: map[string][]byte{"S-ok": zipBody},
	})
	defer f.srv.Close()

	rep := &recordingReporter{}
	count, err := submissions.Download(context.Background(), f.api, "contestX", submissions.Options{
		Mode:       2,
		OutputPath: dst,
		Reporter:   rep,
	})
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1", count)
	}
	if rep.ticks != 1 {
		t.Errorf("ticks = %d, want 1 (skipped sub must not bump)", rep.ticks)
	}
	if rep.total != len(subs) {
		t.Errorf("Start total = %d, want %d (sized to the full list)", rep.total, len(subs))
	}
}
