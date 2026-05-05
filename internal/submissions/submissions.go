// Package submissions implements the contest-level submission download
// orchestration used by the `download-submissions` CLI. It is Layer-3
// — sits above apiv4 (Layer-2) and below the CLI (Layer-4) — and
// reuses the existing apiv4 client + model types verbatim.
//
// Mirrors Python `domjudge_tool_cli/commands/submissions/_submissions.py`'s
// `download_contest_files` bit-for-bit (path layout, skip-on-missing,
// post-download rename, optional zip extraction).
package submissions

// Options configures a Download call.
type Options struct {
	// Mode picks the per-submission folder layout, mirroring the Python
	// reference's `file_path()`:
	//   1: team_<name>/problem_<short_name>/
	//   2: problem_<short_name>/team_<name>/   (Python default)
	//   other: contest_<cid>/
	Mode int

	// OutputPath, when non-empty, is prepended to the relative folder
	// computed from Mode. Empty means "write under the current working
	// directory", matching Python.
	OutputPath string

	// Strict, when non-nil, is forwarded to the apiv4 submissions
	// list query. Phase 1 has no behavioural effect downstream
	// (Python's `contest_files` forwards but never consumes it);
	// kept on the surface API for parity.
	Strict *bool

	// Extract, when true, unzips the downloaded archive in place,
	// renames the alphabetically-first regular entry to
	// `<judgement>_<id><ext>` beside the zip, and removes the zip.
	// Mirrors Python `is_extract`.
	Extract bool
}
