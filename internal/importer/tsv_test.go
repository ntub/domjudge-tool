package importer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ntub/domjudge-tool/internal/model"
)

// TestFormatGroupsTSV_GoldenIdentical pins the CCS groups.tsv layout —
// `File_Version\t1\n` header followed by `{id}\t{name}\n` rows, LF only,
// no trailing CRLF. Drift here breaks the bulk import contract.
func TestFormatGroupsTSV_GoldenIdentical(t *testing.T) {
	groups := []model.Group{{ID: "13337", Name: "Companies"}}

	got := formatGroupsTSV(groups)
	want := mustReadGolden(t, "golden_groups.tsv")
	if got != want {
		t.Fatalf("groups TSV mismatch\n--- got ---\n%q\n--- want ---\n%q", got, want)
	}
}

// TestFormatTeamsTSV_GoldenIdentical pins the CCS teams2.tsv layout —
// `File_Version\t2\n` header followed by 8 tab-separated columns per team.
// Unicode names round-trip through the writer untouched (no escaping).
func TestFormatTeamsTSV_GoldenIdentical(t *testing.T) {
	categoryID := "13337"
	rows := []model.CreateUser{
		{Username: "alice", Name: "Alice", Affiliation: stringPtr("ACME")},
		{Username: "bob", Name: "Bob", Affiliation: stringPtr("NTUB")},
		{Username: "carol", Name: "Carol", Affiliation: stringPtr("NTUB")},
	}
	orgs := map[string]model.Organization{
		"ACME": {ID: "10", ShortName: "ACME", Name: "ACME Inc", Country: ""},
		"NTUB": {ID: "11", ShortName: "NTUB", Name: "NTUB", Country: "TWN"},
	}

	got := formatTeamsTSV(rows, categoryID, orgs)
	want := mustReadGolden(t, "golden_teams2.tsv")
	if got != want {
		t.Fatalf("teams2 TSV mismatch\n--- got ---\n%q\n--- want ---\n%q", got, want)
	}
}

// TestFormatAccountsTSV_RespectsPasswordOverride covers two cases: the
// password column is filled from the row's resolved password, and
// account names mirror CreateUser.Name (not username).
func TestFormatAccountsTSV_RespectsPasswordOverride(t *testing.T) {
	rows := []model.CreateUser{
		{Username: "alice", Name: "Alice", Password: stringPtr("pw-alice")},
		{Username: "bob", Name: "Bob", Password: stringPtr("pw-bob")},
		{Username: "carol", Name: "Carol", Password: stringPtr("pw-carol")},
	}

	got := formatAccountsTSV(rows)
	want := mustReadGolden(t, "golden_accounts.tsv")
	if got != want {
		t.Fatalf("accounts TSV mismatch\n--- got ---\n%q\n--- want ---\n%q", got, want)
	}
}

// TestFormatGroupsTSV_EmptyReturnsHeaderOnly locks the contract that an
// empty group slice still produces the File_Version header so the server
// receives a syntactically valid TSV.
func TestFormatGroupsTSV_EmptyReturnsHeaderOnly(t *testing.T) {
	got := formatGroupsTSV(nil)
	const want = "File_Version\t1\n"
	if got != want {
		t.Fatalf("empty groups TSV = %q, want %q", got, want)
	}
}

// TestFormatTeamsTSV_OrgMissingFallsBackToBlank locks that a row whose
// affiliation is not present in the orgs map degrades to blank
// instName/instShortName/country fields rather than panicking.
func TestFormatTeamsTSV_OrgMissingFallsBackToBlank(t *testing.T) {
	rows := []model.CreateUser{
		{Username: "ghost", Name: "Ghost", Affiliation: stringPtr("UNKNOWN")},
	}
	got := formatTeamsTSV(rows, "13337", map[string]model.Organization{})
	const want = "File_Version\t2\n" +
		"ghost\t\t13337\tGhost\t\t\t\t\n"
	if got != want {
		t.Fatalf("teams TSV missing-org = %q, want %q", got, want)
	}
}

// TestFormatTeamsTSV_NilAffiliationProducesEmptyOrgFields keeps a row
// without any affiliation column going through with empty org fields.
// The server treats an empty organization_id as "no organization".
func TestFormatTeamsTSV_NilAffiliationProducesEmptyOrgFields(t *testing.T) {
	rows := []model.CreateUser{{Username: "u", Name: "U", Affiliation: nil}}
	got := formatTeamsTSV(rows, "1", nil)
	const want = "File_Version\t2\n" +
		"u\t\t1\tU\t\t\t\t\n"
	if got != want {
		t.Fatalf("teams TSV nil affiliation = %q, want %q", got, want)
	}
}

// TestFormatAccountsTSV_NilPasswordRendersEmpty keeps the account row
// stable when a row arrives without a derived password — the server
// rejects the line, but the TSV stays syntactically valid.
func TestFormatAccountsTSV_NilPasswordRendersEmpty(t *testing.T) {
	rows := []model.CreateUser{{Username: "alice", Name: "Alice"}}
	got := formatAccountsTSV(rows)
	const want = "accounts\t1\n" +
		"team\tAlice\talice\t\n"
	if got != want {
		t.Fatalf("accounts TSV nil password = %q, want %q", got, want)
	}
}

func mustReadGolden(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", "importer", name)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v", name, err)
	}
	return string(b)
}
