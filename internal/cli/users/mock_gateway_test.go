package users_test

import (
	"bytes"
	"context"
	"io"

	"github.com/ntub/domjudge-tool/internal/client/web"
	"github.com/ntub/domjudge-tool/internal/model"
)

// mockGateway is a hand-rolled web.Gateway double for users-subtree
// tests. The struct mirrors internal/importer/mock_gateway_test.go;
// tests record arguments without spinning up an HTTP server.
type mockGateway struct {
	loginCalls         int
	loginErr           error
	setPasswordErr     error
	setPasswordCalls   []setPasswordCall
	deleteUsersCalls   int
	deleteUsersInclude []string
	deleteUsersExclude []string
	deleteUsersErr     error
	deleteTeamsCalls   int
	deleteTeamsInclude []string
	deleteTeamsExclude []string
	deleteTeamsErr     error
}

type setPasswordCall struct {
	UserID   string
	Password string
	Roles    []int
	Enabled  bool
}

var _ web.Gateway = (*mockGateway)(nil)

func (m *mockGateway) Login(ctx context.Context) error {
	m.loginCalls++
	if err := ctx.Err(); err != nil {
		return err
	}
	return m.loginErr
}

func (m *mockGateway) GetAffiliations(_ context.Context) ([]model.Affiliation, error) {
	return nil, nil
}

func (m *mockGateway) GetAffiliation(_ context.Context, _ string) (*model.Affiliation, error) {
	return nil, nil
}

func (m *mockGateway) CreateAffiliation(_ context.Context, _, _, _ string) (*model.Affiliation, error) {
	return nil, nil
}

func (m *mockGateway) GetProblems(_ context.Context, _ web.ProblemFilter) ([]model.ProblemItem, error) {
	return nil, nil
}

func (m *mockGateway) SetUserPassword(_ context.Context, userID, password string, roles []int, enabled bool) error {
	m.setPasswordCalls = append(m.setPasswordCalls, setPasswordCall{
		UserID:   userID,
		Password: password,
		Roles:    append([]int(nil), roles...),
		Enabled:  enabled,
	})
	return m.setPasswordErr
}

func (m *mockGateway) UpdateUserEmail(_ context.Context, _, _ string) error { return nil }

func (m *mockGateway) DeleteUsers(_ context.Context, include, exclude []string) error {
	m.deleteUsersCalls++
	m.deleteUsersInclude = append([]string(nil), include...)
	m.deleteUsersExclude = append([]string(nil), exclude...)
	return m.deleteUsersErr
}

func (m *mockGateway) CreateTeamAndUser(_ context.Context, _ model.CreateUser, _, _ int, _ bool) (string, string, error) {
	return "", "", nil
}

func (m *mockGateway) UpdateTeam(_ context.Context, _ model.User, _, _ int, _ bool) (string, string, error) {
	return "", "", nil
}

func (m *mockGateway) DeleteTeams(_ context.Context, include, exclude []string) error {
	m.deleteTeamsCalls++
	m.deleteTeamsInclude = append([]string(nil), include...)
	m.deleteTeamsExclude = append([]string(nil), exclude...)
	return m.deleteTeamsErr
}

// Download is a no-op stub; users-subtree tests don't exercise it.
// PR-7 added Gateway.Download for download-problems.
func (m *mockGateway) Download(_ context.Context, _ string) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(nil)), nil
}
