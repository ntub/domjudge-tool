package importer_test

import (
	"context"
	"strings"
	"testing"

	"github.com/ntub/domjudge-tool/internal/importer"
	"github.com/ntub/domjudge-tool/internal/model"
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

// TestImportUsersTeams_Reporter_TicksOncePerRow asserts that a Reporter
// passed via ImportOptions sees one Tick per CSV row when every row
// flows through the scraper create path. Start is called once with the
// parsed-row total, and Finish is called once after the loop.
func TestImportUsersTeams_Reporter_TicksOncePerRow(t *testing.T) {
	users := usersAPIServer(t, []model.User{})
	gw := &mockGateway{
		createTeamID: "55",
		createUserID: "99",
	}

	body := importCSVHeader + "\n" +
		"alice,Alice,alice@example.org,ntub,pw1\n" +
		"bob,Bob,bob@example.org,ntub,pw2\n" +
		"carol,Carol,carol@example.org,ntub,pw3\n"

	rep := &recordingReporter{}
	out, err := importer.ImportUsersTeams(context.Background(), gw, users, strings.NewReader(body), importer.ImportOptions{
		AffiliationCountry: "TWN",
		Reporter:           rep,
	})
	if err != nil {
		t.Fatalf("ImportUsersTeams: %v", err)
	}
	if len(out) != 3 {
		t.Fatalf("returned %d users, want 3", len(out))
	}
	if rep.starts != 1 {
		t.Errorf("starts = %d, want 1", rep.starts)
	}
	if rep.total != 3 {
		t.Errorf("Start total = %d, want 3", rep.total)
	}
	if rep.ticks != 3 {
		t.Errorf("ticks = %d, want 3 (one per row)", rep.ticks)
	}
	if rep.finishes != 1 {
		t.Errorf("finishes = %d, want 1", rep.finishes)
	}
}

// TestImportUsersTeams_Reporter_NilUsesNop asserts that omitting
// ImportOptions.Reporter (nil) does not panic and behaves as a silent
// no-op — the library MUST default nil to progress.Nop().
func TestImportUsersTeams_Reporter_NilUsesNop(t *testing.T) {
	users := usersAPIServer(t, []model.User{})
	gw := &mockGateway{createTeamID: "1", createUserID: "2"}

	body := importCSVHeader + "\nalice,Alice,alice@example.org,ntub,pw1\n"
	if _, err := importer.ImportUsersTeams(context.Background(), gw, users, strings.NewReader(body), importer.ImportOptions{
		AffiliationCountry: "TWN",
	}); err != nil {
		t.Fatalf("ImportUsersTeams(nil reporter): %v", err)
	}
	if len(gw.createTeamAndUserCalls) != 1 {
		t.Errorf("CreateTeamAndUser call count = %d, want 1", len(gw.createTeamAndUserCalls))
	}
}

// TestImportUsersTeams_Reporter_TicksUpdatePath asserts that the update
// cohort (rows whose username already exists in the snapshot) ticks
// once per row through the scraper update branch.
func TestImportUsersTeams_Reporter_TicksUpdatePath(t *testing.T) {
	teamID := "3"
	existing := []model.User{
		{ID: "7", Username: "alice", Name: "OldAlice", TeamID: &teamID},
		{ID: "8", Username: "bob", Name: "OldBob", TeamID: &teamID},
	}
	users := usersAPIServer(t, existing)
	gw := &mockGateway{
		updateTeamID: teamID,
		updateUserID: "7",
	}

	body := importCSVHeader + "\n" +
		"alice,Alice,alice@example.org,ntub,pw1\n" +
		"bob,Bob,bob@example.org,ntub,pw2\n"

	rep := &recordingReporter{}
	if _, err := importer.ImportUsersTeams(context.Background(), gw, users, strings.NewReader(body), importer.ImportOptions{
		AffiliationCountry: "TWN",
		Reporter:           rep,
	}); err != nil {
		t.Fatalf("ImportUsersTeams: %v", err)
	}
	if rep.ticks != 2 {
		t.Errorf("ticks = %d, want 2 (one per updated row)", rep.ticks)
	}
	if rep.starts != 1 || rep.finishes != 1 {
		t.Errorf("starts=%d finishes=%d, want 1/1", rep.starts, rep.finishes)
	}
	if rep.total != 2 {
		t.Errorf("Start total = %d, want 2", rep.total)
	}
}
