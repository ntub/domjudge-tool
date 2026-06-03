package v4

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGeneralResource_Version_DecodesStringAndIntegerAPIVersion_Integration(t *testing.T) {
	tests := []struct {
		name        string
		fixturePath string
		want        string
	}{
		{
			name:        "v7 string",
			fixturePath: "../../../../testdata/api/v4/version.json",
			want:        "v4",
		},
		{
			name:        "v8 integer",
			fixturePath: "../../../../testdata/api/v4/v8/version.json",
			want:        "4",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fixture := readAPIFixture(t, tc.fixturePath)
			srv := jsonFixtureServer(t, map[string][]byte{
				"/api/v4/version": fixture,
			})
			c := New(Options{BaseURL: srv.URL})

			got, err := c.General.Version(context.Background())
			if err != nil {
				t.Fatalf("Version: %v", err)
			}
			if got != tc.want {
				t.Fatalf("Version = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestProblemsResource_List_DecodesIntegerAndIntegralFloatTimeLimit_Integration(t *testing.T) {
	tests := []struct {
		name    string
		body    []byte
		want    int
		wantErr bool
	}{
		{
			name: "integer",
			body: []byte(`[{"id":"1","short_name":"a","label":"A","time_limit":5,"name":"A"}]`),
			want: 5,
		},
		{
			name: "integral float",
			body: []byte(`[{"id":"1","short_name":"a","label":"A","time_limit":5.0,"name":"A"}]`),
			want: 5,
		},
		{
			name:    "non integral float",
			body:    []byte(`[{"id":"1","short_name":"a","label":"A","time_limit":1.5,"name":"A"}]`),
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := jsonFixtureServer(t, map[string][]byte{
				"/api/v4/contests/1/problems": tc.body,
			})
			c := New(Options{BaseURL: srv.URL})

			problems, err := c.Problems.List(context.Background(), "1")
			if tc.wantErr {
				if err == nil {
					t.Fatal("List returned nil error, want decode error")
				}
				if !strings.Contains(err.Error(), "time_limit") {
					t.Fatalf("err = %v, want time_limit decode error", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("List: %v", err)
			}
			if len(problems) != 1 {
				t.Fatalf("len(problems) = %d, want 1", len(problems))
			}
			if problems[0].TimeLimit != tc.want {
				t.Fatalf("TimeLimit = %d, want %d", problems[0].TimeLimit, tc.want)
			}
		})
	}
}

func TestProblemsResource_List_DecodesV8Fixture_Integration(t *testing.T) {
	fixture := readAPIFixture(t, "../../../../testdata/api/v4/v8/contest_1/problems.json")
	srv := jsonFixtureServer(t, map[string][]byte{
		"/api/v4/contests/8/problems": fixture,
	})
	c := New(Options{BaseURL: srv.URL})

	problems, err := c.Problems.List(context.Background(), "8")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(problems) != 3 {
		t.Fatalf("len(problems) = %d, want 3", len(problems))
	}
	if problems[0].TimeLimit != 5 {
		t.Fatalf("TimeLimit = %d, want 5", problems[0].TimeLimit)
	}
}

func TestResources_DecodeV8FixturesWithSilentDrift_Integration(t *testing.T) {
	// Audit result for P2-1: shipped commands do not read Contest.ExternalID,
	// Submission.ExternalID, Organization.URL/Hashtag, Team.Members, or
	// Judgement.Judgehost. The default JSON decoder already accepts missing or
	// unknown fields, so these tests lock decode success without adding unused
	// compatibility fields.
	srv := jsonFixtureServer(t, map[string][]byte{
		"/api/v4/contests":                 readAPIFixture(t, "../../../../testdata/api/v4/v8/contests.json"),
		"/api/v4/contests/8":               readAPIFixture(t, "../../../../testdata/api/v4/v8/contest_1/contest.json"),
		"/api/v4/contests/8/groups":        readAPIFixture(t, "../../../../testdata/api/v4/v8/contest_1/groups.json"),
		"/api/v4/contests/8/organizations": readAPIFixture(t, "../../../../testdata/api/v4/v8/contest_1/organizations.json"),
		"/api/v4/contests/8/scoreboard":    readAPIFixture(t, "../../../../testdata/api/v4/v8/contest_1/scoreboard.json"),
		"/api/v4/contests/8/submissions":   readAPIFixture(t, "../../../../testdata/api/v4/v8/contest_1/submissions.json"),
		"/api/v4/contests/8/teams":         readAPIFixture(t, "../../../../testdata/api/v4/v8/contest_1/teams.json"),
		"/api/v4/contests/8/judgements":    []byte(`[{"id":"1","submission_id":"748","valid":true,"judgement_type_id":"AC"}]`),
	})
	c := New(Options{BaseURL: srv.URL})
	ctx := context.Background()

	contests, err := c.Contests.List(ctx)
	if err != nil {
		t.Fatalf("Contests.List: %v", err)
	}
	if len(contests) == 0 {
		t.Fatal("Contests.List returned no contests")
	}

	contest, err := c.Contests.Get(ctx, "8")
	if err != nil {
		t.Fatalf("Contests.Get: %v", err)
	}
	if contest.ID != "8" {
		t.Fatalf("contest.ID = %q, want 8", contest.ID)
	}

	groups, err := c.Groups.List(ctx, "8")
	if err != nil {
		t.Fatalf("Groups.List: %v", err)
	}
	if len(groups) == 0 {
		t.Fatal("Groups.List returned no groups")
	}

	orgs, err := c.Organizations.List(ctx, "8")
	if err != nil {
		t.Fatalf("Organizations.List: %v", err)
	}
	if len(orgs) == 0 {
		t.Fatal("Organizations.List returned no organizations")
	}

	teams, err := c.Teams.List(ctx, "8")
	if err != nil {
		t.Fatalf("Teams.List: %v", err)
	}
	if len(teams) == 0 {
		t.Fatal("Teams.List returned no teams")
	}

	submissions, err := c.Submissions.List(ctx, "8", SubmissionsListOpts{})
	if err != nil {
		t.Fatalf("Submissions.List: %v", err)
	}
	if len(submissions) == 0 {
		t.Fatal("Submissions.List returned no submissions")
	}

	judgements, err := c.Judgements.List(ctx, "8", JudgementsListOpts{})
	if err != nil {
		t.Fatalf("Judgements.List: %v", err)
	}
	if len(judgements) != 1 {
		t.Fatalf("len(judgements) = %d, want 1", len(judgements))
	}

	scoreboard, err := c.Scoreboard.Get(ctx, "8", ScoreboardListOpts{})
	if err != nil {
		t.Fatalf("Scoreboard.Get: %v", err)
	}
	if len(scoreboard.Rows) == 0 {
		t.Fatal("Scoreboard.Get returned no rows")
	}
}

func readAPIFixture(t *testing.T, path string) []byte {
	t.Helper()

	fixture, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}
	return fixture
}

func jsonFixtureServer(t *testing.T, fixtures map[string][]byte) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fixture, ok := fixtures[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fixture)
	}))
	t.Cleanup(srv.Close)
	return srv
}
