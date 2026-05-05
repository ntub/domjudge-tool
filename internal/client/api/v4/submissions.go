package v4

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/url"
	"os"
	"path/filepath"

	"github.com/ntub/domjudge-tool/internal/model"
	"github.com/ntub/domjudge-tool/internal/util/pathx"
)

// SubmissionsResource wraps the /contests/{cid}/submissions endpoints.
type SubmissionsResource struct{ c *Client }

// SubmissionsListOpts holds optional query parameters for List.
// Nil / empty fields are omitted from the query string.
type SubmissionsListOpts struct {
	IDs        []string
	LanguageID *string
	Strict     *bool
}

// List returns all submissions in contest cid matching opts from
// GET /api/v4/contests/{cid}/submissions.
func (r *SubmissionsResource) List(ctx context.Context, cid string, opts SubmissionsListOpts) ([]model.Submission, error) {
	q := url.Values{}
	for _, id := range opts.IDs {
		q.Add("ids[]", id)
	}
	if opts.LanguageID != nil {
		q.Set("language_id", *opts.LanguageID)
	}
	if opts.Strict != nil {
		if *opts.Strict {
			q.Set("strict", "true")
		} else {
			q.Set("strict", "false")
		}
	}
	var out []model.Submission
	if err := r.c.getJSON(ctx, fmt.Sprintf("/contests/%s/submissions", cid)+buildQuery(q), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Get returns a single submission from
// GET /api/v4/contests/{cid}/submissions/{id}.
func (r *SubmissionsResource) Get(ctx context.Context, cid, id string) (*model.Submission, error) {
	var out model.Submission
	if err := r.c.getJSON(ctx, fmt.Sprintf("/contests/%s/submissions/%s", cid, id), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Files downloads the submission ZIP to dst and returns the written file
// path. Returns ErrInvalidArgument when dst is empty. The output filename
// is taken from the Content-Disposition header when present; otherwise it
// falls back to id+".zip". pathx.SafeFilename is applied for defense-in-depth.
//
// Locks Python submissions.py:58 — os.path.isdir(file_path) panics when
// file_path is None. The Go implementation checks dst before any I/O.
func (r *SubmissionsResource) Files(ctx context.Context, cid, id, dst string) (string, error) {
	if dst == "" {
		return "", fmt.Errorf("%w: dst must not be empty", ErrInvalidArgument)
	}

	resp, err := r.c.getStream(ctx, fmt.Sprintf("/contests/%s/submissions/%s/files", cid, id))
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	// Derive filename from Content-Disposition, fall back to id+".zip".
	filename := id + ".zip"
	if cd := resp.Header.Get("Content-Disposition"); cd != "" {
		if _, params, err := mime.ParseMediaType(cd); err == nil {
			if fn, ok := params["filename"]; ok && fn != "" {
				filename = fn
			}
		}
	}
	filename = pathx.SafeFilename(filename)
	if filename == "" {
		filename = id + ".zip"
	}

	if err := os.MkdirAll(dst, 0o755); err != nil {
		return "", fmt.Errorf("create dst dir: %w", err)
	}

	out := filepath.Join(dst, filename)
	f, err := os.Create(out)
	if err != nil {
		return "", fmt.Errorf("create output file: %w", err)
	}
	defer func() { _ = f.Close() }()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return "", fmt.Errorf("write zip: %w", err)
	}
	return out, nil
}

// SourceCode returns the source-code file list for submission id from
// GET /api/v4/contests/{cid}/submissions/{id}/source-code.
// Returns ErrEmptyResponse when the server returns an empty array.
func (r *SubmissionsResource) SourceCode(ctx context.Context, cid, id string) ([]model.SubmissionFile, error) {
	var out []model.SubmissionFile
	if err := r.c.getJSON(ctx, fmt.Sprintf("/contests/%s/submissions/%s/source-code", cid, id), &out); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, ErrEmptyResponse
	}
	return out, nil
}
