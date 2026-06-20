package v4

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/ntub/domjudge-tool/internal/model"
)

// ImportsResource wraps the bulk import endpoints under /api/v4.
type ImportsResource struct{ c *Client }

// GroupsTSV imports DOMjudge groups from a CCS groups.tsv payload.
func (r *ImportsResource) GroupsTSV(ctx context.Context, tsv io.Reader) ([]string, error) {
	return r.postIDs(ctx, "/users/groups", map[string]multipartField{
		"tsv": multipartFile{filename: "groups.tsv", contentType: "text/tab-separated-values", reader: tsv},
	})
}

// GroupsJSON imports DOMjudge groups from a JSON payload.
func (r *ImportsResource) GroupsJSON(ctx context.Context, body []model.Group) ([]string, error) {
	field, err := jsonFile("groups.json", body)
	if err != nil {
		return nil, err
	}
	return r.postIDs(ctx, "/users/groups", map[string]multipartField{"json": field})
}

// Organizations imports DOMjudge organizations from a JSON payload.
func (r *ImportsResource) Organizations(ctx context.Context, body []model.Organization) ([]string, error) {
	field, err := jsonFile("organizations.json", body)
	if err != nil {
		return nil, err
	}
	return r.postIDs(ctx, "/users/organizations", map[string]multipartField{"json": field})
}

// TeamsTSV imports DOMjudge teams from a CCS teams2.tsv payload.
func (r *ImportsResource) TeamsTSV(ctx context.Context, tsv io.Reader) ([]string, error) {
	return r.postIDs(ctx, "/users/teams", map[string]multipartField{
		"tsv": multipartFile{filename: "teams2.tsv", contentType: "text/tab-separated-values", reader: tsv},
	})
}

// TeamsJSON imports DOMjudge teams from a JSON payload.
func (r *ImportsResource) TeamsJSON(ctx context.Context, body []model.TeamImport) ([]string, error) {
	field, err := jsonFile("teams.json", body)
	if err != nil {
		return nil, err
	}
	return r.postIDs(ctx, "/users/teams", map[string]multipartField{"json": field})
}

// Accounts imports DOMjudge accounts from a CCS accounts.tsv payload.
func (r *ImportsResource) Accounts(ctx context.Context, tsv io.Reader) ([]string, error) {
	return r.postIDs(ctx, "/users/accounts", map[string]multipartField{
		"tsv": multipartFile{filename: "accounts.tsv", contentType: "text/tab-separated-values", reader: tsv},
	})
}

// Contest imports a DOMjudge contest from a combined contest YAML payload and
// returns the server-assigned contest id.
func (r *ImportsResource) Contest(ctx context.Context, yaml io.Reader) (string, error) {
	ids, err := r.postIDs(ctx, "/contests", map[string]multipartField{
		"yaml": multipartFile{filename: "contest.yaml", contentType: "application/x-yaml", reader: yaml},
	})
	if err != nil {
		return "", err
	}
	return singleImportID(ids)
}

// Problem imports one ICPC problem ZIP into contest cid and returns the new
// server-assigned problem id.
func (r *ImportsResource) Problem(ctx context.Context, cid, problemID string, zip io.Reader) (string, error) {
	if cid == "" {
		return "", fmt.Errorf("%w: cid must not be empty", ErrInvalidArgument)
	}
	if problemID == "" {
		return "", fmt.Errorf("%w: problemID must not be empty", ErrInvalidArgument)
	}
	ids, err := r.postIDs(ctx, fmt.Sprintf("/contests/%s/problems", cid), map[string]multipartField{
		"problem": multipartValue{value: problemID},
		"zip[]":   multipartFile{filename: problemID + ".zip", contentType: "application/zip", reader: zip},
	})
	if err != nil {
		return "", err
	}
	return singleImportID(ids)
}

// ProblemPackageUpdate imports one ICPC problem ZIP into an existing problem
// id using DOMjudge 8.x's `zip` file field.
func (r *ImportsResource) ProblemPackageUpdate(ctx context.Context, cid, problemID, filename string, zip io.Reader) (string, error) {
	if cid == "" {
		return "", fmt.Errorf("%w: cid must not be empty", ErrInvalidArgument)
	}
	if problemID == "" {
		return "", fmt.Errorf("%w: problemID must not be empty", ErrInvalidArgument)
	}
	if filename == "" {
		filename = problemID + ".zip"
	}
	ids, err := r.postIDs(ctx, fmt.Sprintf("/contests/%s/problems", cid), map[string]multipartField{
		"problem": multipartValue{value: problemID},
		"zip":     multipartFile{filename: filename, contentType: "application/zip", reader: zip},
	})
	if err != nil {
		return "", err
	}
	return singleImportID(ids)
}

// ProblemPackage imports one problem ZIP into contest cid as a new problem.
// Unlike Problem, it omits the optional `problem` form field; DOMjudge treats
// that field as an update target and rejects new imports when the id is absent.
func (r *ImportsResource) ProblemPackage(ctx context.Context, cid, filename string, zip io.Reader) error {
	if cid == "" {
		return fmt.Errorf("%w: cid must not be empty", ErrInvalidArgument)
	}
	if filename == "" {
		filename = "problem.zip"
	}
	if err := r.c.postMultipart(ctx, fmt.Sprintf("/contests/%s/problems", cid), map[string]multipartField{
		"zip": multipartFile{filename: filename, contentType: "application/zip", reader: zip},
	}, nil); err != nil {
		return err
	}
	return nil
}

// ProblemPackageZipArray imports one problem ZIP using the legacy DOMjudge 7
// multipart file field name (`zip[]`). DOMjudge 8 switched this create path to
// `zip`, so callers should select this only for 7.x servers.
func (r *ImportsResource) ProblemPackageZipArray(ctx context.Context, cid, filename string, zip io.Reader) error {
	if cid == "" {
		return fmt.Errorf("%w: cid must not be empty", ErrInvalidArgument)
	}
	if filename == "" {
		filename = "problem.zip"
	}
	if err := r.c.postMultipart(ctx, fmt.Sprintf("/contests/%s/problems", cid), map[string]multipartField{
		"zip[]": multipartFile{filename: filename, contentType: "application/zip", reader: zip},
	}, nil); err != nil {
		return err
	}
	return nil
}

func (r *ImportsResource) postIDs(ctx context.Context, path string, fields map[string]multipartField) ([]string, error) {
	var ids importIDs
	if err := r.c.postMultipart(ctx, path, fields, &ids); err != nil {
		return nil, err
	}
	return []string(ids), nil
}

func jsonFile(filename string, body interface{}) (multipartFile, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return multipartFile{}, fmt.Errorf("encode %s: %w", filename, err)
	}
	return multipartFile{
		filename:    filename,
		contentType: "application/json",
		reader:      bytes.NewReader(raw),
	}, nil
}

func singleImportID(ids []string) (string, error) {
	if len(ids) == 0 || ids[0] == "" {
		return "", ErrEmptyResponse
	}
	return ids[0], nil
}

type importIDs []string

func (ids *importIDs) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return ErrEmptyResponse
	}

	if list, ok := decodeImportIDList(data); ok {
		*ids = list
		return nil
	}

	var single string
	if err := json.Unmarshal(data, &single); err == nil {
		*ids = []string{single}
		return nil
	}

	var objects []map[string]json.RawMessage
	if err := json.Unmarshal(data, &objects); err == nil {
		out := make([]string, 0, len(objects))
		for _, obj := range objects {
			id, ok := firstStringField(obj, "id", "externalid", "problem_id")
			if !ok {
				return fmt.Errorf("decode import ids: object has no id field")
			}
			out = append(out, id)
		}
		*ids = out
		return nil
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err == nil {
		for _, key := range []string{"ids", "problem_ids"} {
			raw, ok := obj[key]
			if !ok {
				continue
			}
			if list, ok := decodeImportIDList(raw); ok {
				*ids = list
				return nil
			}
		}
		if id, ok := firstStringField(obj, "id", "cid", "contest_id", "problem_id", "new_id"); ok {
			*ids = []string{id}
			return nil
		}
	}

	return fmt.Errorf("decode import ids: unsupported response %s", strings.TrimSpace(string(data)))
}

func decodeImportIDList(data []byte) ([]string, bool) {
	var stringIDs []string
	if err := json.Unmarshal(data, &stringIDs); err == nil {
		return stringIDs, true
	}
	var numbers []json.Number
	if err := json.Unmarshal(data, &numbers); err != nil {
		return nil, false
	}
	out := make([]string, 0, len(numbers))
	for _, number := range numbers {
		if number.String() == "" {
			return nil, false
		}
		out = append(out, number.String())
	}
	return out, true
}

func firstStringField(obj map[string]json.RawMessage, keys ...string) (string, bool) {
	for _, key := range keys {
		raw, ok := obj[key]
		if !ok {
			continue
		}
		var value string
		if err := json.Unmarshal(raw, &value); err == nil && value != "" {
			return value, true
		}
		var numeric json.Number
		if err := json.Unmarshal(raw, &numeric); err == nil && numeric.String() != "" {
			return numeric.String(), true
		}
	}
	return "", false
}
