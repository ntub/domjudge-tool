package model_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ntub/domjudge-tool/internal/model"
)

// roundTripGoldenJSON loads a fixture, unmarshals it into v, marshals it
// back, and asserts the bytes are identical to the input. The fixture
// must already be in Go's canonical `json.MarshalIndent("", "    ")`
// shape (no trailing newline) so the assertion is byte-exact.
func roundTripGoldenJSON(t *testing.T, fixture string, v interface{}) {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", "model", fixture)
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if err := json.Unmarshal(want, v); err != nil {
		t.Fatalf("unmarshal %s: %v", fixture, err)
	}
	got, err := json.MarshalIndent(v, "", "    ")
	if err != nil {
		t.Fatalf("marshal %s: %v", fixture, err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("round-trip drift for %s\n--- want ---\n%s\n--- got ---\n%s", fixture, want, got)
	}
}

func TestAffiliation_RoundTripGoldenJSON(t *testing.T) {
	var v model.Affiliation
	roundTripGoldenJSON(t, "affiliation.json", &v)
}

func TestUser_RoundTripGoldenJSON(t *testing.T) {
	var v model.User
	roundTripGoldenJSON(t, "user.json", &v)
}

func TestCreateUser_RoundTripGoldenJSON(t *testing.T) {
	var v model.CreateUser
	roundTripGoldenJSON(t, "create_user.json", &v)
}

func TestTeam_RoundTripGoldenJSON(t *testing.T) {
	var v model.Team
	roundTripGoldenJSON(t, "team.json", &v)
}

func TestProblem_RoundTripGoldenJSON(t *testing.T) {
	var v model.Problem
	roundTripGoldenJSON(t, "problem.json", &v)
}

func TestProblemItem_RoundTripGoldenJSON(t *testing.T) {
	var v model.ProblemItem
	roundTripGoldenJSON(t, "problem_item.json", &v)
}

func TestSubmission_RoundTripGoldenJSON(t *testing.T) {
	var v model.Submission
	roundTripGoldenJSON(t, "submission.json", &v)
}

func TestSubmissionFile_RoundTripGoldenJSON(t *testing.T) {
	var v model.SubmissionFile
	roundTripGoldenJSON(t, "submission_file.json", &v)
}

func TestJudgement_RoundTripGoldenJSON(t *testing.T) {
	var v model.Judgement
	roundTripGoldenJSON(t, "judgement.json", &v)
}

func TestJudgementType_RoundTripGoldenJSON(t *testing.T) {
	var v model.JudgementType
	roundTripGoldenJSON(t, "judgement_type.json", &v)
}

// TestUser_OptionalFieldsAreNullableNotOmitted asserts that an
// unmarshalled-then-marshalled User keeps `null` for unset Optional
// fields rather than omitting them. The Python `model.dict()` /
// `model.json()` output emits the keys with null values; matching that
// behaviour is what the snake_case-tags-without-omitempty convention
// buys us.
func TestUser_OptionalFieldsAreNullableNotOmitted(t *testing.T) {
	src := []byte(`{"last_login_time": null, "first_login_time": null, "team": null, "roles": ["team"], "id": "1", "username": "u", "name": "n", "email": null, "last_ip": null, "ip": null, "enabled": true, "team_id": null, "affiliation": null, "password": null}`)
	var u model.User
	if err := json.Unmarshal(src, &u); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out, err := json.Marshal(u)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	for _, key := range []string{
		"last_login_time", "first_login_time", "team", "email",
		"last_ip", "ip", "team_id", "affiliation", "password",
	} {
		raw, ok := got[key]
		if !ok {
			t.Errorf("expected key %q to be present (with null value), but it was omitted", key)
			continue
		}
		if string(raw) != "null" {
			t.Errorf("expected key %q to be null, got %s", key, raw)
		}
	}
}

// TestCreateUser_FromUser ports the Python `CreateUser.from_user`
// helper. The Python code copies a User dict, marks `is_exist=True`,
// and lets explicit kwargs override the defaults.
//
// Regression note: the Python version forwards every User field
// (including immutable ones like `id` and `team_id`) into CreateUser
// even though CreateUser doesn't have them — Pydantic v1 silently
// drops unknown keys. The Go port stays explicit: only the fields
// declared on CreateUser are populated, and `IsExist` defaults to
// `true`.
func TestCreateUser_FromUser(t *testing.T) {
	email := "u@example.com"
	password := "secret"
	affiliation := "ntub"
	u := model.User{
		ID:          "42",
		Username:    "u",
		Name:        "n",
		Email:       &email,
		Affiliation: &affiliation,
		Password:    &password,
	}
	cu := model.CreateUserFromUser(u)
	if cu.Username != "u" || cu.Name != "n" {
		t.Fatalf("required fields not copied: %+v", cu)
	}
	if cu.IsExist == nil || *cu.IsExist != true {
		t.Fatalf("IsExist should default to true, got %v", cu.IsExist)
	}
	if cu.Email == nil || *cu.Email != email {
		t.Fatalf("Email not copied: %v", cu.Email)
	}
	if cu.Password == nil || *cu.Password != password {
		t.Fatalf("Password not copied: %v", cu.Password)
	}
	if cu.Affiliation == nil || *cu.Affiliation != affiliation {
		t.Fatalf("Affiliation not copied: %v", cu.Affiliation)
	}
}

func TestCreateUser_FromUser_HonoursOverrides(t *testing.T) {
	u := model.User{ID: "42", Username: "u", Name: "old"}
	overrideName := "Overridden Name"
	overrideEmail := "new@example.com"
	overridePassword := "newsecret"
	overrideAffiliation := "newaff"
	overrideExist := false
	cu := model.CreateUserFromUser(u, model.CreateUserOverrides{
		Name:        &overrideName,
		Email:       &overrideEmail,
		Password:    &overridePassword,
		Affiliation: &overrideAffiliation,
		IsExist:     &overrideExist,
	})
	if cu.Name != overrideName {
		t.Errorf("Name override ignored: %s", cu.Name)
	}
	if cu.Email == nil || *cu.Email != overrideEmail {
		t.Errorf("Email override ignored: %v", cu.Email)
	}
	if cu.Password == nil || *cu.Password != overridePassword {
		t.Errorf("Password override ignored: %v", cu.Password)
	}
	if cu.Affiliation == nil || *cu.Affiliation != overrideAffiliation {
		t.Errorf("Affiliation override ignored: %v", cu.Affiliation)
	}
	if cu.IsExist == nil || *cu.IsExist != false {
		t.Errorf("IsExist override ignored: %v", cu.IsExist)
	}
}

// TestTeam_PreservesGroupIDsOrder asserts that round-tripping a Team
// keeps GroupIDs in input order. Important because the v7 web UI
// matches teams by group ID and a reordering would change category
// assignments.
func TestTeam_PreservesGroupIDsOrder(t *testing.T) {
	src := []byte(`{"group_ids": ["a","b","c"], "affiliation": null, "nationality": null, "id": "1", "icpc_id": null, "name": "T", "display_name": null, "organization_id": null, "members": null}`)
	var team model.Team
	if err := json.Unmarshal(src, &team); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !reflect.DeepEqual(team.GroupIDs, []string{"a", "b", "c"}) {
		t.Fatalf("group_ids reordered: %v", team.GroupIDs)
	}
}
