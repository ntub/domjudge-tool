package problems_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync/atomic"

	"github.com/ntub/domjudge-tool/internal/client/web"
	"github.com/ntub/domjudge-tool/internal/model"
)

// mockGateway is a minimal web.Gateway double for the cli/problems
// tests. Only Login, GetProblems, and Download are wired with real
// recording fields; the rest of the Gateway surface returns zero
// values and is never exercised by download-problems.
type mockGateway struct {
	loginCalls       atomic.Int32
	loginErr         error
	getProblemsCalls atomic.Int32
	getProblemsArg   web.ProblemFilter
	getProblemsRes   []model.ProblemItem
	getProblemsErr   error
	downloadBodies   map[string][]byte
	downloadCalls    []string
	downloadErr      error
}

var _ web.Gateway = (*mockGateway)(nil)

func (m *mockGateway) Login(_ context.Context) error {
	m.loginCalls.Add(1)
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

func (m *mockGateway) GetProblems(_ context.Context, f web.ProblemFilter) ([]model.ProblemItem, error) {
	m.getProblemsCalls.Add(1)
	m.getProblemsArg = f
	return m.getProblemsRes, m.getProblemsErr
}

func (m *mockGateway) SetUserPassword(_ context.Context, _, _ string, _ []int, _ bool) error {
	return nil
}

func (m *mockGateway) UpdateUserEmail(_ context.Context, _, _ string) error { return nil }

func (m *mockGateway) DeleteUsers(_ context.Context, _, _ []string) error { return nil }

func (m *mockGateway) CreateTeamAndUser(_ context.Context, _ model.CreateUser, _, _ int, _ bool) (string, string, error) {
	return "", "", nil
}

func (m *mockGateway) UpdateTeam(_ context.Context, _ model.User, _, _ int, _ bool) (string, string, error) {
	return "", "", nil
}

func (m *mockGateway) DeleteTeams(_ context.Context, _, _ []string) error { return nil }

func (m *mockGateway) Download(_ context.Context, path string) (io.ReadCloser, error) {
	m.downloadCalls = append(m.downloadCalls, path)
	if m.downloadErr != nil {
		return nil, m.downloadErr
	}
	body, ok := m.downloadBodies[path]
	if !ok {
		return nil, fmt.Errorf("mock gateway: no body registered for %s", path)
	}
	return io.NopCloser(bytes.NewReader(body)), nil
}
