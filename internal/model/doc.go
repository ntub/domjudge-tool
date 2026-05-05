// Package model holds the plain data types shared across the codebase
// (User, Team, Problem, Affiliation, Submission, Judgement and friends).
//
// The types mirror the Pydantic models in the Python reference CLI
// (`domjudge_tool_cli.models`) field-for-field. JSON tags use snake_case
// and Optional fields are pointer types without `omitempty` so that the
// JSON output preserves the Pydantic convention of emitting `null` for
// unset optional values.
//
// The package contains no I/O. Decoding and encoding are handled by
// callers (typically `internal/client/api/v4` for incoming JSON and
// `internal/exporter` for outgoing JSON or CSV).
package model
