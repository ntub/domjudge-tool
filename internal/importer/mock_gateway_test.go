package importer_test

import (
	"bytes"
	"context"
	"io"

	"github.com/ntub/domjudge-tool/internal/client/web"
	"github.com/ntub/domjudge-tool/internal/model"
)

// mockGateway is a hand-rolled test double for [web.Gateway]. Every
// method records its arguments and returns the configured zero or
// stubbed value, so tests can verify call order, captured arguments,
// and error injection without spinning up an HTTP server.
//
// All exported fields are written by tests before invoking the system
// under test. Capture fields are append-only and queried after the call
// returns. The struct is not safe for concurrent use; importer code
// runs serially.
type mockGateway struct {
	// Errors injected to make a method return non-nil. Zero means no
	// error.
	loginErr             error
	getAffiliationsErr   error
	getAffiliationErr    error
	createAffiliationErr error
	getProblemsErr       error
	setUserPasswordErr   error
	updateUserEmailErr   error
	deleteUsersErr       error
	createTeamUserErr    error
	updateTeamErr        error
	deleteTeamsErr       error

	// Stubbed return values for non-error fields.
	getAffiliationsResult   []model.Affiliation
	getAffiliationResult    *model.Affiliation
	createAffiliationResult *model.Affiliation
	getProblemsResult       []model.ProblemItem
	createTeamID            string
	createUserID            string
	updateTeamID            string
	updateUserID            string

	// Capture fields populated as the methods are called.
	loginCalls             int
	getAffiliationCalls    []string
	createAffiliationCalls []createAffCall
	setUserPasswordCalls   []setUserPasswordCall
	updateUserEmailCalls   []updateUserEmailCall
	deleteUsersIncludeCap  []string
	deleteUsersExcludeCap  []string
	deleteUsersCalls       int
	deleteTeamsIncludeCap  []string
	deleteTeamsExcludeCap  []string
	deleteTeamsCalls       int
	createTeamAndUserCalls []createTeamAndUserCall
	updateTeamCalls        []updateTeamCall

	// callOrder records the names of methods as they are invoked, in
	// order, so tests can assert "DeleteUsers fired before DeleteTeams".
	callOrder []string
}

type createAffCall struct {
	ShortName string
	Name      string
	Country   string
}

type setUserPasswordCall struct {
	UserID   string
	Password string
	Roles    []int
	Enabled  bool
}

type updateUserEmailCall struct {
	UserID string
	Email  string
}

type createTeamAndUserCall struct {
	User          model.CreateUser
	CategoryID    int
	AffiliationID int
	Enabled       bool
}

type updateTeamCall struct {
	User          model.User
	CategoryID    int
	AffiliationID int
	Enabled       bool
}

// Compile-time check that mockGateway satisfies web.Gateway.
var _ web.Gateway = (*mockGateway)(nil)

func (m *mockGateway) record(name string) { m.callOrder = append(m.callOrder, name) }

func (m *mockGateway) Login(ctx context.Context) error {
	m.record("Login")
	m.loginCalls++
	if err := ctx.Err(); err != nil {
		return err
	}
	return m.loginErr
}

func (m *mockGateway) GetAffiliations(_ context.Context) ([]model.Affiliation, error) {
	m.record("GetAffiliations")
	return m.getAffiliationsResult, m.getAffiliationsErr
}

func (m *mockGateway) GetAffiliation(_ context.Context, name string) (*model.Affiliation, error) {
	m.record("GetAffiliation")
	m.getAffiliationCalls = append(m.getAffiliationCalls, name)
	return m.getAffiliationResult, m.getAffiliationErr
}

func (m *mockGateway) CreateAffiliation(_ context.Context, shortname, name, country string) (*model.Affiliation, error) {
	m.record("CreateAffiliation")
	m.createAffiliationCalls = append(m.createAffiliationCalls, createAffCall{ShortName: shortname, Name: name, Country: country})
	if m.createAffiliationResult == nil && m.createAffiliationErr == nil {
		// Default: pretend the gateway minted ID 1 so tests that don't
		// care about the affiliation ID can omit the stub.
		idStr := "1"
		return &model.Affiliation{ID: &idStr, ShortName: shortname, Name: name, Country: country}, nil
	}
	return m.createAffiliationResult, m.createAffiliationErr
}

func (m *mockGateway) GetProblems(_ context.Context, _ web.ProblemFilter) ([]model.ProblemItem, error) {
	m.record("GetProblems")
	return m.getProblemsResult, m.getProblemsErr
}

func (m *mockGateway) SetUserPassword(_ context.Context, userID, password string, roles []int, enabled bool) error {
	m.record("SetUserPassword")
	m.setUserPasswordCalls = append(m.setUserPasswordCalls, setUserPasswordCall{
		UserID:   userID,
		Password: password,
		Roles:    append([]int(nil), roles...),
		Enabled:  enabled,
	})
	return m.setUserPasswordErr
}

func (m *mockGateway) UpdateUserEmail(_ context.Context, userID, email string) error {
	m.record("UpdateUserEmail")
	m.updateUserEmailCalls = append(m.updateUserEmailCalls, updateUserEmailCall{
		UserID: userID,
		Email:  email,
	})
	return m.updateUserEmailErr
}

func (m *mockGateway) DeleteUsers(_ context.Context, include, exclude []string) error {
	m.record("DeleteUsers")
	m.deleteUsersCalls++
	m.deleteUsersIncludeCap = append([]string(nil), include...)
	m.deleteUsersExcludeCap = append([]string(nil), exclude...)
	return m.deleteUsersErr
}

func (m *mockGateway) CreateTeamAndUser(_ context.Context, user model.CreateUser, categoryID, affiliationID int, enabled bool) (string, string, error) {
	m.record("CreateTeamAndUser")
	m.createTeamAndUserCalls = append(m.createTeamAndUserCalls, createTeamAndUserCall{
		User:          user,
		CategoryID:    categoryID,
		AffiliationID: affiliationID,
		Enabled:       enabled,
	})
	return m.createTeamID, m.createUserID, m.createTeamUserErr
}

func (m *mockGateway) UpdateTeam(_ context.Context, user model.User, categoryID, affiliationID int, enabled bool) (string, string, error) {
	m.record("UpdateTeam")
	m.updateTeamCalls = append(m.updateTeamCalls, updateTeamCall{
		User:          user,
		CategoryID:    categoryID,
		AffiliationID: affiliationID,
		Enabled:       enabled,
	})
	return m.updateTeamID, m.updateUserID, m.updateTeamErr
}

func (m *mockGateway) DeleteTeams(_ context.Context, include, exclude []string) error {
	m.record("DeleteTeams")
	m.deleteTeamsCalls++
	m.deleteTeamsIncludeCap = append([]string(nil), include...)
	m.deleteTeamsExcludeCap = append([]string(nil), exclude...)
	return m.deleteTeamsErr
}

// Download is a no-op stub for the importer-side mock; importer tests
// don't exercise the download path. PR-7 widened web.Gateway with
// Download for download-problems support.
func (m *mockGateway) Download(_ context.Context, _ string) (io.ReadCloser, error) {
	m.record("Download")
	return io.NopCloser(bytes.NewReader(nil)), nil
}
