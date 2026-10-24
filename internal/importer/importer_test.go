package importer_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	apiv4 "github.com/ntub/domjudge-tool/internal/client/api/v4"
	"github.com/ntub/domjudge-tool/internal/config"
	"github.com/ntub/domjudge-tool/internal/importer"
	"github.com/ntub/domjudge-tool/internal/model"
)

// importCSVHeader preserves the legacy five-column input order.
const importCSVHeader = "username,name,email,affiliation,password"

const importCSVHeaderWithCategory = importCSVHeader + ",category_id"

// usersAPIServer wires an httptest.Server at GET /api/v4/users
// returning the supplied snapshot, and returns an *apiv4.UsersResource
// pointing at it. The server is registered with t.Cleanup.
func usersAPIServer(t *testing.T, snapshot []model.User) *apiv4.UsersResource {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(snapshot)
	}))
	t.Cleanup(srv.Close)
	c := apiv4.New(apiv4.Options{BaseURL: srv.URL, Username: "admin", Password: "secret"})
	return c.Users
}

// TestCSVImport_HeadersByteIdentical verifies that the legacy five-column
// header remains accepted. The optional category_id column is tested separately.
func TestCSVImport_HeadersByteIdentical(t *testing.T) {
	users := usersAPIServer(t, []model.User{})
	gw := &mockGateway{}

	body := importCSVHeader + "\nntubuser,Test,ntubuser@example.com,ntub,pw1\n"
	_, err := importer.ImportUsersTeams(context.Background(), gw, users, strings.NewReader(body), importer.ImportOptions{
		AffiliationCountry: "TWN",
	})
	if err != nil {
		t.Fatalf("ImportUsersTeams failed on canonical header: %v", err)
	}
}

func TestCSVImport_BOMStripped(t *testing.T) {
	users := usersAPIServer(t, []model.User{})
	gw := &mockGateway{}

	path := filepath.Join("..", "..", "testdata", "importer", "with_bom.csv")
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open with_bom.csv: %v", err)
	}
	t.Cleanup(func() { _ = f.Close() })

	if _, err := importer.ImportUsersTeams(context.Background(), gw, users, f, importer.ImportOptions{
		AffiliationCountry: "TWN",
	}); err != nil {
		t.Fatalf("ImportUsersTeams on BOM input: %v", err)
	}
}

func TestImportUsersTeams_CreateNewUser(t *testing.T) {
	users := usersAPIServer(t, []model.User{})
	gw := &mockGateway{
		createTeamID: "55",
		createUserID: "99",
	}

	body := importCSVHeader + "\nntubuser,Test User,ntubuser@example.com,ntub,pw1\n"
	out, err := importer.ImportUsersTeams(context.Background(), gw, users, strings.NewReader(body), importer.ImportOptions{
		AffiliationCountry: "TWN",
	})
	if err != nil {
		t.Fatalf("ImportUsersTeams: %v", err)
	}

	if len(gw.createTeamAndUserCalls) != 1 {
		t.Fatalf("CreateTeamAndUser called %d times, want 1", len(gw.createTeamAndUserCalls))
	}
	if gw.createTeamAndUserCalls[0].User.Username != "ntubuser" {
		t.Errorf("CreateTeamAndUser username = %q, want ntubuser", gw.createTeamAndUserCalls[0].User.Username)
	}
	if len(gw.setUserPasswordCalls) != 1 {
		t.Fatalf("SetUserPassword called %d times, want 1", len(gw.setUserPasswordCalls))
	}
	if gw.setUserPasswordCalls[0].UserID != "99" {
		t.Errorf("SetUserPassword userID = %q, want 99", gw.setUserPasswordCalls[0].UserID)
	}
	if len(out) != 1 {
		t.Fatalf("returned %d users, want 1", len(out))
	}
}

func TestImportUsersTeams_UpdateExistingUser(t *testing.T) {
	existing := model.User{
		ID:       "7",
		Username: "ntubuser",
		Name:     "Existing",
		TeamID:   strPtr("3"),
	}
	users := usersAPIServer(t, []model.User{existing})
	gw := &mockGateway{
		updateTeamID: "3",
		updateUserID: "7",
	}

	body := importCSVHeader + "\nntubuser,Test User,ntubuser@example.com,ntub,pw1\n"
	if _, err := importer.ImportUsersTeams(context.Background(), gw, users, strings.NewReader(body), importer.ImportOptions{
		AffiliationCountry: "TWN",
	}); err != nil {
		t.Fatalf("ImportUsersTeams: %v", err)
	}

	if len(gw.updateTeamCalls) != 1 {
		t.Fatalf("UpdateTeam called %d times, want 1", len(gw.updateTeamCalls))
	}
	if len(gw.createTeamAndUserCalls) != 0 {
		t.Errorf("CreateTeamAndUser must not be called for existing user, got %d calls", len(gw.createTeamAndUserCalls))
	}
	if gw.updateTeamCalls[0].User.Name != "Test User" {
		t.Errorf("UpdateTeam User.Name = %q, want %q (CSV name must override snapshot)", gw.updateTeamCalls[0].User.Name, "Test User")
	}
}

func TestImportUsersTeams_CategoryIDColumnOverridesFallback(t *testing.T) {
	existing := model.User{
		ID:       "7",
		Username: "alice",
		Name:     "Existing",
		TeamID:   strPtr("3"),
	}
	users := usersAPIServer(t, []model.User{existing})
	gw := &mockGateway{
		createTeamID: "55",
		createUserID: "99",
		updateTeamID: "3",
		updateUserID: "7",
	}
	fallbackCategoryID := 21

	body := importCSVHeaderWithCategory +
		"\nalice,Alice,alice@example.org,,pw1,7" +
		"\nbob,Bob,bob@example.org,,pw2,\n"
	if _, err := importer.ImportUsersTeams(context.Background(), gw, users, strings.NewReader(body), importer.ImportOptions{
		CategoryID:         &fallbackCategoryID,
		AffiliationCountry: "TWN",
	}); err != nil {
		t.Fatalf("ImportUsersTeams: %v", err)
	}

	if len(gw.updateTeamCalls) != 1 {
		t.Fatalf("UpdateTeam calls = %d, want 1", len(gw.updateTeamCalls))
	}
	if got := gw.updateTeamCalls[0].CategoryID; got != 7 {
		t.Errorf("UpdateTeam CategoryID = %d, want CSV override 7", got)
	}
	if len(gw.createTeamAndUserCalls) != 1 {
		t.Fatalf("CreateTeamAndUser calls = %d, want 1", len(gw.createTeamAndUserCalls))
	}
	if got := gw.createTeamAndUserCalls[0].CategoryID; got != fallbackCategoryID {
		t.Errorf("CreateTeamAndUser CategoryID = %d, want fallback %d", got, fallbackCategoryID)
	}
}

func TestImportUsersTeams_MissingCategoryIDColumnUsesFallback(t *testing.T) {
	users := usersAPIServer(t, []model.User{})
	gw := &mockGateway{
		createTeamID: "55",
		createUserID: "99",
	}
	fallbackCategoryID := 21

	body := importCSVHeader + "\nalice,Alice,alice@example.org,,pw1\n"
	if _, err := importer.ImportUsersTeams(context.Background(), gw, users, strings.NewReader(body), importer.ImportOptions{
		CategoryID:         &fallbackCategoryID,
		AffiliationCountry: "TWN",
	}); err != nil {
		t.Fatalf("ImportUsersTeams: %v", err)
	}

	if len(gw.createTeamAndUserCalls) != 1 {
		t.Fatalf("CreateTeamAndUser calls = %d, want 1", len(gw.createTeamAndUserCalls))
	}
	if got := gw.createTeamAndUserCalls[0].CategoryID; got != fallbackCategoryID {
		t.Errorf("CreateTeamAndUser CategoryID = %d, want fallback %d", got, fallbackCategoryID)
	}
}

func TestImportUsersTeams_IgnoreExisting(t *testing.T) {
	existing := model.User{ID: "7", Username: "ntubuser", TeamID: strPtr("3")}
	users := usersAPIServer(t, []model.User{existing})
	gw := &mockGateway{}

	body := importCSVHeader + "\nntubuser,Test,ntubuser@example.com,ntub,pw1\n"
	if _, err := importer.ImportUsersTeams(context.Background(), gw, users, strings.NewReader(body), importer.ImportOptions{
		AffiliationCountry: "TWN",
		IgnoreExisting:     true,
	}); err != nil {
		t.Fatalf("ImportUsersTeams: %v", err)
	}

	if len(gw.updateTeamCalls) != 0 {
		t.Errorf("UpdateTeam was called %d times under IgnoreExisting, want 0", len(gw.updateTeamCalls))
	}
	if len(gw.createTeamAndUserCalls) != 0 {
		t.Errorf("CreateTeamAndUser was called %d times under IgnoreExisting, want 0", len(gw.createTeamAndUserCalls))
	}
	if len(gw.setUserPasswordCalls) != 0 {
		t.Errorf("SetUserPassword was called %d times under IgnoreExisting, want 0", len(gw.setUserPasswordCalls))
	}
}

func TestImportUsersTeams_DeleteExisting(t *testing.T) {
	existing := model.User{ID: "7", Username: "ntubuser", TeamID: strPtr("3")}
	users := usersAPIServer(t, []model.User{existing})
	gw := &mockGateway{}

	body := importCSVHeader + "\nntubuser,Test,ntubuser@example.com,ntub,pw1\n"
	if _, err := importer.ImportUsersTeams(context.Background(), gw, users, strings.NewReader(body), importer.ImportOptions{
		AffiliationCountry: "TWN",
		DeleteExisting:     true,
	}); err != nil {
		t.Fatalf("ImportUsersTeams: %v", err)
	}

	// DeleteUsers and DeleteTeams must precede the first creation call.
	deleteUsersIdx, deleteTeamsIdx, createIdx := -1, -1, -1
	for i, name := range gw.callOrder {
		switch name {
		case "DeleteUsers":
			if deleteUsersIdx == -1 {
				deleteUsersIdx = i
			}
		case "DeleteTeams":
			if deleteTeamsIdx == -1 {
				deleteTeamsIdx = i
			}
		case "CreateTeamAndUser":
			if createIdx == -1 {
				createIdx = i
			}
		}
	}
	if deleteUsersIdx < 0 {
		t.Fatalf("DeleteUsers never called; callOrder=%v", gw.callOrder)
	}
	if deleteTeamsIdx < 0 {
		t.Fatalf("DeleteTeams never called; callOrder=%v", gw.callOrder)
	}
	if createIdx < 0 {
		t.Fatalf("CreateTeamAndUser never called; callOrder=%v", gw.callOrder)
	}
	if deleteUsersIdx >= createIdx || deleteTeamsIdx >= createIdx {
		t.Errorf("delete pass must precede creation; callOrder=%v", gw.callOrder)
	}
}

func TestImportUsersTeams_GeneratesPassword(t *testing.T) {
	users := usersAPIServer(t, []model.User{})
	gw := &mockGateway{}

	// Empty password column.
	body := importCSVHeader + "\nntubuser,Test,ntubuser@example.com,ntub,\n"
	out, err := importer.ImportUsersTeams(context.Background(), gw, users, strings.NewReader(body), importer.ImportOptions{
		AffiliationCountry: "TWN",
	})
	if err != nil {
		t.Fatalf("ImportUsersTeams: %v", err)
	}

	if len(gw.setUserPasswordCalls) != 1 {
		t.Fatalf("SetUserPassword calls = %d, want 1", len(gw.setUserPasswordCalls))
	}
	if gw.setUserPasswordCalls[0].Password == "" {
		t.Errorf("generated password is empty")
	}
	if len(out) != 1 || out[0].Password == nil || *out[0].Password == "" {
		t.Errorf("returned slice missing password: %+v", out)
	}
}

func TestImportUsersTeams_NewPasswordFlag(t *testing.T) {
	users := usersAPIServer(t, []model.User{})
	gw := &mockGateway{}

	body := importCSVHeader + "\nntubuser,Test,ntubuser@example.com,ntub,not-empty\n"
	out, err := importer.ImportUsersTeams(context.Background(), gw, users, strings.NewReader(body), importer.ImportOptions{
		AffiliationCountry: "TWN",
		NewPassword:        true,
	})
	if err != nil {
		t.Fatalf("ImportUsersTeams: %v", err)
	}

	if len(gw.setUserPasswordCalls) != 1 {
		t.Fatalf("SetUserPassword calls = %d, want 1", len(gw.setUserPasswordCalls))
	}
	if gw.setUserPasswordCalls[0].Password == "not-empty" {
		t.Errorf("NewPassword=true must override CSV password, got CSV value passed through")
	}
	if out[0].Password == nil || *out[0].Password == "not-empty" {
		t.Errorf("returned slice still has CSV password under NewPassword=true: %+v", out[0].Password)
	}
}

func TestImportUsersTeams_AffiliationLookupFound(t *testing.T) {
	users := usersAPIServer(t, []model.User{})
	gw := &mockGateway{
		getAffiliationResult: &model.Affiliation{ID: strPtr("42"), Name: "ntub", ShortName: "ntub", Country: "TWN"},
	}

	body := importCSVHeader + "\nntubuser,Test,ntubuser@example.com,ntub,pw1\n"
	if _, err := importer.ImportUsersTeams(context.Background(), gw, users, strings.NewReader(body), importer.ImportOptions{
		AffiliationCountry: "TWN",
	}); err != nil {
		t.Fatalf("ImportUsersTeams: %v", err)
	}

	if len(gw.createAffiliationCalls) != 0 {
		t.Errorf("CreateAffiliation must not run when GetAffiliation found a match; got %d calls", len(gw.createAffiliationCalls))
	}
	if len(gw.createTeamAndUserCalls) != 1 {
		t.Fatalf("CreateTeamAndUser calls = %d, want 1", len(gw.createTeamAndUserCalls))
	}
	if gw.createTeamAndUserCalls[0].AffiliationID != 42 {
		t.Errorf("CreateTeamAndUser AffiliationID = %d, want 42", gw.createTeamAndUserCalls[0].AffiliationID)
	}
}

func TestImportUsersTeams_AffiliationLookupNotFound(t *testing.T) {
	users := usersAPIServer(t, []model.User{})
	gw := &mockGateway{
		// nil result == not found
		getAffiliationResult:    nil,
		createAffiliationResult: &model.Affiliation{ID: strPtr("77"), Name: "ntub", ShortName: "ntub", Country: "TWN"},
	}

	body := importCSVHeader + "\nntubuser,Test,ntubuser@example.com,ntub,pw1\n"
	if _, err := importer.ImportUsersTeams(context.Background(), gw, users, strings.NewReader(body), importer.ImportOptions{
		AffiliationCountry: "TWN",
	}); err != nil {
		t.Fatalf("ImportUsersTeams: %v", err)
	}

	if len(gw.createAffiliationCalls) != 1 {
		t.Fatalf("CreateAffiliation calls = %d, want 1", len(gw.createAffiliationCalls))
	}
	got := gw.createAffiliationCalls[0]
	if got.Country != "TWN" {
		t.Errorf("CreateAffiliation country = %q, want TWN", got.Country)
	}
	if got.Name != "ntub" {
		t.Errorf("CreateAffiliation name = %q, want ntub", got.Name)
	}
	if gw.createTeamAndUserCalls[0].AffiliationID != 77 {
		t.Errorf("CreateTeamAndUser AffiliationID = %d, want 77 (from CreateAffiliation)", gw.createTeamAndUserCalls[0].AffiliationID)
	}
}

func TestImportUsersTeamsWithReport_AffiliationMissingCreatesOnceAndCaches(t *testing.T) {
	users := usersAPIServer(t, []model.User{})
	gw := &mockGateway{
		getAffiliationResult:    nil,
		createAffiliationResult: &model.Affiliation{ID: strPtr("77"), Name: "ntub", ShortName: "ntub", Country: "TWN"},
	}

	body := importCSVHeader +
		"\nalice,Alice,alice@example.org,ntub,pw1" +
		"\nbob,Bob,bob@example.org,ntub,pw2\n"
	result, err := importer.ImportUsersTeamsWithReport(context.Background(), gw, users, strings.NewReader(body), importer.ImportOptions{
		AffiliationCountry: "TWN",
	})
	if err != nil {
		t.Fatalf("ImportUsersTeamsWithReport: %v", err)
	}

	if len(gw.getAffiliationCalls) != 1 {
		t.Fatalf("GetAffiliation calls = %d, want 1 (cached duplicate affiliation)", len(gw.getAffiliationCalls))
	}
	if len(gw.createAffiliationCalls) != 1 {
		t.Fatalf("CreateAffiliation calls = %d, want 1", len(gw.createAffiliationCalls))
	}
	if len(gw.createTeamAndUserCalls) != 2 {
		t.Fatalf("CreateTeamAndUser calls = %d, want 2", len(gw.createTeamAndUserCalls))
	}
	for i, call := range gw.createTeamAndUserCalls {
		if call.AffiliationID != 77 {
			t.Errorf("CreateTeamAndUser call %d AffiliationID = %d, want 77", i, call.AffiliationID)
		}
	}
	if result.Summary.AffiliationsCreated != 1 {
		t.Errorf("AffiliationsCreated = %d, want 1", result.Summary.AffiliationsCreated)
	}
	if len(result.Summary.AffiliationsCreatedNames) != 1 || result.Summary.AffiliationsCreatedNames[0] != "ntub" {
		t.Errorf("AffiliationsCreatedNames = %v, want [ntub]", result.Summary.AffiliationsCreatedNames)
	}
}

func TestImportUsersTeamsWithReport_AffiliationFoundReportsReusedCounts(t *testing.T) {
	users := usersAPIServer(t, []model.User{})
	gw := &mockGateway{
		getAffiliationResult: &model.Affiliation{ID: strPtr("42"), Name: "ntub", ShortName: "ntub", Country: "TWN"},
	}

	body := importCSVHeader +
		"\nalice,Alice,alice@example.org,ntub,pw1" +
		"\nbob,Bob,bob@example.org,ntub,pw2\n"
	result, err := importer.ImportUsersTeamsWithReport(context.Background(), gw, users, strings.NewReader(body), importer.ImportOptions{
		AffiliationCountry: "TWN",
	})
	if err != nil {
		t.Fatalf("ImportUsersTeamsWithReport: %v", err)
	}

	if len(gw.getAffiliationCalls) != 1 {
		t.Fatalf("GetAffiliation calls = %d, want 1", len(gw.getAffiliationCalls))
	}
	if len(gw.createAffiliationCalls) != 0 {
		t.Fatalf("CreateAffiliation calls = %d, want 0", len(gw.createAffiliationCalls))
	}
	if result.Summary.AffiliationsReusedUnique != 1 {
		t.Errorf("AffiliationsReusedUnique = %d, want 1", result.Summary.AffiliationsReusedUnique)
	}
	if result.Summary.AffiliationsReusedRows != 2 {
		t.Errorf("AffiliationsReusedRows = %d, want 2", result.Summary.AffiliationsReusedRows)
	}
}

func TestImportUsersTeamsWithReport_BlankAffiliationUsesFallbackWithoutLookup(t *testing.T) {
	users := usersAPIServer(t, []model.User{})
	gw := &mockGateway{}
	fallbackAffID := 285

	body := importCSVHeader +
		"\nalice,Alice,alice@example.org,,pw1" +
		"\nbob,Bob,bob@example.org,   ,pw2\n"
	result, err := importer.ImportUsersTeamsWithReport(context.Background(), gw, users, strings.NewReader(body), importer.ImportOptions{
		AffiliationID:      &fallbackAffID,
		AffiliationCountry: "TWN",
	})
	if err != nil {
		t.Fatalf("ImportUsersTeamsWithReport: %v", err)
	}

	if len(gw.getAffiliationCalls) != 0 {
		t.Fatalf("GetAffiliation calls = %d, want 0", len(gw.getAffiliationCalls))
	}
	if len(gw.createAffiliationCalls) != 0 {
		t.Fatalf("CreateAffiliation calls = %d, want 0", len(gw.createAffiliationCalls))
	}
	if len(gw.createTeamAndUserCalls) != 2 {
		t.Fatalf("CreateTeamAndUser calls = %d, want 2", len(gw.createTeamAndUserCalls))
	}
	for i, call := range gw.createTeamAndUserCalls {
		if call.AffiliationID != fallbackAffID {
			t.Errorf("CreateTeamAndUser call %d AffiliationID = %d, want %d", i, call.AffiliationID, fallbackAffID)
		}
		if call.User.Affiliation != nil {
			t.Errorf("CreateTeamAndUser call %d Affiliation = %v, want nil for blank/whitespace CSV field", i, call.User.Affiliation)
		}
	}
	if result.Summary.FallbackAffiliationIDRows != 2 {
		t.Errorf("FallbackAffiliationIDRows = %d, want 2", result.Summary.FallbackAffiliationIDRows)
	}
	if result.Summary.FallbackAffiliationID != fallbackAffID {
		t.Errorf("FallbackAffiliationID = %d, want %d", result.Summary.FallbackAffiliationID, fallbackAffID)
	}
}

func TestImportUsersTeamsWithReport_UserSummaryCounts(t *testing.T) {
	teamID := "3"
	existing := model.User{ID: "7", Username: "alice", Name: "Old Alice", TeamID: &teamID}

	tests := []struct {
		name        string
		opts        importer.ImportOptions
		wantCreated int
		wantUpdated int
		wantSkipped int
		wantRows    int
	}{
		{
			name:        "created and updated",
			opts:        importer.ImportOptions{AffiliationCountry: "TWN"},
			wantCreated: 1,
			wantUpdated: 1,
			wantSkipped: 0,
			wantRows:    2,
		},
		{
			name:        "skipped existing",
			opts:        importer.ImportOptions{AffiliationCountry: "TWN", IgnoreExisting: true},
			wantCreated: 1,
			wantUpdated: 0,
			wantSkipped: 1,
			wantRows:    1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			users := usersAPIServer(t, []model.User{existing})
			gw := &mockGateway{
				createTeamID: "55",
				createUserID: "99",
				updateTeamID: teamID,
				updateUserID: "7",
			}

			body := importCSVHeader +
				"\nalice,Alice,alice@example.org,,pw1" +
				"\nbob,Bob,bob@example.org,,pw2\n"
			result, err := importer.ImportUsersTeamsWithReport(context.Background(), gw, users, strings.NewReader(body), tc.opts)
			if err != nil {
				t.Fatalf("ImportUsersTeamsWithReport: %v", err)
			}

			if result.Summary.InputRows != 2 {
				t.Errorf("InputRows = %d, want 2", result.Summary.InputRows)
			}
			if result.Summary.ProcessedRows != tc.wantRows {
				t.Errorf("ProcessedRows = %d, want %d", result.Summary.ProcessedRows, tc.wantRows)
			}
			if result.Summary.CreatedUsers != tc.wantCreated {
				t.Errorf("CreatedUsers = %d, want %d", result.Summary.CreatedUsers, tc.wantCreated)
			}
			if result.Summary.UpdatedUsers != tc.wantUpdated {
				t.Errorf("UpdatedUsers = %d, want %d", result.Summary.UpdatedUsers, tc.wantUpdated)
			}
			if result.Summary.SkippedExistingUsers != tc.wantSkipped {
				t.Errorf("SkippedExistingUsers = %d, want %d", result.Summary.SkippedExistingUsers, tc.wantSkipped)
			}
			if len(result.Users) != tc.wantRows {
				t.Errorf("Users len = %d, want %d", len(result.Users), tc.wantRows)
			}
		})
	}
}

func TestImportUsersTeams_EmptyEmail_BecomesNil(t *testing.T) {
	users := usersAPIServer(t, []model.User{})
	gw := &mockGateway{}

	// Empty email column.
	body := importCSVHeader + "\nntubuser,Test,,ntub,pw1\n"
	out, err := importer.ImportUsersTeams(context.Background(), gw, users, strings.NewReader(body), importer.ImportOptions{
		AffiliationCountry: "TWN",
	})
	if err != nil {
		t.Fatalf("ImportUsersTeams: %v", err)
	}

	if len(out) != 1 {
		t.Fatalf("returned %d users, want 1", len(out))
	}
	if out[0].Email != nil {
		t.Errorf("CreateUser.Email = %v, want nil for empty CSV column", out[0].Email)
	}
}

func TestImportUsersTeams_ContextCancelled(t *testing.T) {
	users := usersAPIServer(t, []model.User{})
	gw := &mockGateway{}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	body := importCSVHeader + "\nntubuser,Test,ntubuser@example.com,ntub,pw1\n"
	_, err := importer.ImportUsersTeams(ctx, gw, users, strings.NewReader(body), importer.ImportOptions{
		AffiliationCountry: "TWN",
	})
	if err == nil {
		t.Fatalf("expected error on cancelled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

// TestImportUsersTeams_HeaderMismatch covers the csvio error path: a
// CSV body with the wrong header order surfaces as a wrapped
// csvio.ErrHeaderMismatch and the gateway sees no create/update
// activity (login already happened, so loginCalls==1 is fine).
func TestImportUsersTeams_HeaderMismatch(t *testing.T) {
	users := usersAPIServer(t, []model.User{})
	gw := &mockGateway{}

	body := "user,username,email,affiliation,password\nTest,ntubuser,ntubuser@example.com,ntub,pw1\n"
	out, err := importer.ImportUsersTeams(context.Background(), gw, users, strings.NewReader(body), importer.ImportOptions{
		AffiliationCountry: "TWN",
	})
	if err == nil {
		t.Fatalf("expected ErrHeaderMismatch, got nil")
	}
	if out != nil {
		t.Errorf("expected nil output on header error, got %v", out)
	}
	if !strings.Contains(err.Error(), "header") {
		t.Errorf("error %q should mention header drift", err.Error())
	}
	if len(gw.createTeamAndUserCalls) != 0 {
		t.Errorf("CreateTeamAndUser fired despite header error: %d calls", len(gw.createTeamAndUserCalls))
	}
}

// TestImportUsersTeams_PartialSliceOnError pins the per-row abort
// contract: when a gateway call fails on row N, the returned slice
// contains the N-1 rows already processed.
func TestImportUsersTeams_PartialSliceOnError(t *testing.T) {
	users := usersAPIServer(t, []model.User{})
	gw := &mockGateway{
		createTeamID: "1",
		createUserID: "10",
		// Make SetUserPassword fail; one row will be processed up to
		// that point, then the second row would have been processed
		// but we abort before any new state is appended.
		setUserPasswordErr: errors.New("boom"),
	}

	body := importCSVHeader +
		"\nalice,Alice,alice@example.com,ntub,pw1" +
		"\nbob,Bob,bob@example.com,ntub,pw2\n"
	out, err := importer.ImportUsersTeams(context.Background(), gw, users, strings.NewReader(body), importer.ImportOptions{
		AffiliationCountry: "TWN",
	})
	if err == nil {
		t.Fatalf("expected error from SetUserPassword stub")
	}
	if len(out) != 0 {
		t.Errorf("partial slice = %d rows, want 0 (SetUserPassword failed before append)", len(out))
	}
}

// TestImportUsersTeams_IsExist_NewUser verifies that the CreateUser
// returned for a brand-new (not-in-snapshot) row has IsExist == &false.
func TestImportUsersTeams_IsExist_NewUser(t *testing.T) {
	users := usersAPIServer(t, []model.User{})
	gw := &mockGateway{createTeamID: "1", createUserID: "10"}

	body := importCSVHeader + "\nntubuser,Test,ntubuser@example.com,ntub,pw1\n"
	out, err := importer.ImportUsersTeams(context.Background(), gw, users, strings.NewReader(body), importer.ImportOptions{
		AffiliationCountry: "TWN",
	})
	if err != nil {
		t.Fatalf("ImportUsersTeams: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("returned %d users, want 1", len(out))
	}
	if out[0].IsExist == nil || *out[0].IsExist {
		t.Errorf("IsExist = %v, want &false for new user", out[0].IsExist)
	}
}

// TestImportUsersTeams_IsExist_ExistingUser verifies that the
// CreateUser returned for an existing (in-snapshot) row has
// IsExist == &true.
func TestImportUsersTeams_IsExist_ExistingUser(t *testing.T) {
	existing := model.User{ID: "7", Username: "ntubuser", Name: "Old", TeamID: strPtr("3")}
	users := usersAPIServer(t, []model.User{existing})
	gw := &mockGateway{updateTeamID: "3", updateUserID: "7"}

	body := importCSVHeader + "\nntubuser,Test User,ntubuser@example.com,ntub,pw1\n"
	out, err := importer.ImportUsersTeams(context.Background(), gw, users, strings.NewReader(body), importer.ImportOptions{
		AffiliationCountry: "TWN",
	})
	if err != nil {
		t.Fatalf("ImportUsersTeams: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("returned %d users, want 1", len(out))
	}
	if out[0].IsExist == nil || !*out[0].IsExist {
		t.Errorf("IsExist = %v, want &true for existing user", out[0].IsExist)
	}
}

// TestImportUsersTeams_IsExist_DeleteExisting verifies that a user
// processed via the DeleteExisting path (deleted then re-created) has
// IsExist == &false in the output — it was re-created fresh.
func TestImportUsersTeams_IsExist_DeleteExisting(t *testing.T) {
	existing := model.User{ID: "7", Username: "ntubuser", TeamID: strPtr("3")}
	users := usersAPIServer(t, []model.User{existing})
	gw := &mockGateway{createTeamID: "55", createUserID: "99"}

	body := importCSVHeader + "\nntubuser,Test,ntubuser@example.com,ntub,pw1\n"
	out, err := importer.ImportUsersTeams(context.Background(), gw, users, strings.NewReader(body), importer.ImportOptions{
		AffiliationCountry: "TWN",
		DeleteExisting:     true,
	})
	if err != nil {
		t.Fatalf("ImportUsersTeams: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("returned %d users, want 1", len(out))
	}
	if out[0].IsExist == nil || *out[0].IsExist {
		t.Errorf("IsExist = %v, want &false for DeleteExisting re-created user", out[0].IsExist)
	}
}

// TestImportUsersTeams_MalformedRow_DeleteExistingNoMutations locks the
// regression: a row with wrong column count must be caught by
// validateRows before preDelete fires, so no user or team is deleted.
func TestImportUsersTeams_MalformedRow_DeleteExistingNoMutations(t *testing.T) {
	existing := model.User{ID: "7", Username: "alice", TeamID: strPtr("3")}
	users := usersAPIServer(t, []model.User{existing})
	gw := &mockGateway{}

	// "alice" has only one column — malformed row.
	body := importCSVHeader + "\nalice\n"
	_, err := importer.ImportUsersTeams(context.Background(), gw, users, strings.NewReader(body), importer.ImportOptions{
		AffiliationCountry: "TWN",
		DeleteExisting:     true,
	})
	if err == nil {
		t.Fatal("expected error for malformed row")
	}
	if gw.deleteUsersCalls != 0 {
		t.Errorf("DeleteUsers called %d times, want 0: malformed CSV must be rejected before any mutation", gw.deleteUsersCalls)
	}
	if gw.deleteTeamsCalls != 0 {
		t.Errorf("DeleteTeams called %d times, want 0: malformed CSV must be rejected before any mutation", gw.deleteTeamsCalls)
	}
	if len(gw.createTeamAndUserCalls) != 0 {
		t.Errorf("CreateTeamAndUser called %d times, want 0", len(gw.createTeamAndUserCalls))
	}
	if len(gw.updateTeamCalls) != 0 {
		t.Errorf("UpdateTeam called %d times, want 0", len(gw.updateTeamCalls))
	}
}

// TestImportUsersTeams_DuplicateUsername_NoMutations locks that a CSV
// with duplicate usernames is rejected before any gateway mutation.
// With DeleteExisting the first occurrence would delete, and the second
// would fail; without it the create path would hit the same team twice.
func TestImportUsersTeams_DuplicateUsername_NoMutations(t *testing.T) {
	existing := model.User{ID: "7", Username: "alice", TeamID: strPtr("3")}
	users := usersAPIServer(t, []model.User{existing})
	gw := &mockGateway{}

	body := importCSVHeader +
		"\nalice,Alice,alice@example.com,ntub,pw1" +
		"\nalice,AliceDup,alice@example.com,ntub,pw2\n"
	_, err := importer.ImportUsersTeams(context.Background(), gw, users, strings.NewReader(body), importer.ImportOptions{
		AffiliationCountry: "TWN",
		DeleteExisting:     true,
	})
	if err == nil {
		t.Fatal("expected error for duplicate username")
	}
	if gw.deleteUsersCalls != 0 {
		t.Errorf("DeleteUsers called %d times, want 0: duplicate username must be rejected before any mutation", gw.deleteUsersCalls)
	}
	if gw.deleteTeamsCalls != 0 {
		t.Errorf("DeleteTeams called %d times, want 0", gw.deleteTeamsCalls)
	}
	if len(gw.createTeamAndUserCalls) != 0 {
		t.Errorf("CreateTeamAndUser called %d times, want 0", len(gw.createTeamAndUserCalls))
	}
}

// TestImportUsersTeams_DefaultRouter_CreateGoesScraper keeps default creation
// on the scraper even when an API client is available. CCS accounts.tsv lacks
// an explicit team link, so bulk creation can attach users to unrelated teams.
func TestImportUsersTeams_DefaultRouter_CreateGoesScraper(t *testing.T) {
	users := usersAPIServer(t, []model.User{})
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/v4/users/") && r.Method == http.MethodPost {
			t.Errorf("api bulk import endpoint must not be hit on default router; got %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	t.Cleanup(apiSrv.Close)

	gw := &mockGateway{createTeamID: "55", createUserID: "99"}
	api := apiv4.New(apiv4.Options{BaseURL: apiSrv.URL})
	body := importCSVHeader + "\nntubuser,Test,ntubuser@example.com,,pw1\n"
	if _, err := importer.ImportUsersTeams(context.Background(), gw, users, strings.NewReader(body), importer.ImportOptions{
		AffiliationCountry: "TWN",
		Router:             importer.NewDefaultRouter(&config.DomServerClient{}),
		API:                api,
	}); err != nil {
		t.Fatalf("ImportUsersTeams: %v", err)
	}
	if len(gw.createTeamAndUserCalls) != 1 {
		t.Fatalf("CreateTeamAndUser calls = %d, want 1 (scraper path)", len(gw.createTeamAndUserCalls))
	}
}

// TestImportUsersTeams_LoginErrorPropagates locks the contract that
// any error from gw.Login surfaces wrapped, before any other gateway
// call runs.
func TestImportUsersTeams_LoginErrorPropagates(t *testing.T) {
	users := usersAPIServer(t, []model.User{})
	gw := &mockGateway{loginErr: errors.New("denied")}

	body := importCSVHeader + "\nntubuser,Test,ntubuser@example.com,ntub,pw1\n"
	out, err := importer.ImportUsersTeams(context.Background(), gw, users, strings.NewReader(body), importer.ImportOptions{
		AffiliationCountry: "TWN",
	})
	if err == nil {
		t.Fatalf("expected error when Login fails")
	}
	if !strings.Contains(err.Error(), "login") {
		t.Errorf("error %q should mention login", err.Error())
	}
	if out != nil {
		t.Errorf("expected nil out, got %v", out)
	}
	if len(gw.createTeamAndUserCalls) != 0 || len(gw.updateTeamCalls) != 0 {
		t.Errorf("no create/update calls should run after Login failure")
	}
}
