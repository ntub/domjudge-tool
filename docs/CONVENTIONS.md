# Code conventions

Style baseline is **[Effective Go](https://go.dev/doc/effective_go)** plus
the [Google Go Style Guide](https://google.github.io/styleguide/go/).
This document records the project-specific rules that go beyond those.

## Formatting

- Run `just fmt` before committing. The pre-commit hook does this for you.
- Imports are grouped by `gci` in this order: standard library, third-party,
  `github.com/ntub/domjudge-tool/...` (see `.golangci.yml`).
- Tabs for indentation in Go; 2 spaces elsewhere (`.editorconfig` is law).

## Errors

- Always wrap errors with context: `fmt.Errorf("loading config: %w", err)`.
- Prefer sentinel errors (`var ErrFoo = errors.New(...)`) when callers need
  to branch on identity. Match with `errors.Is` / `errors.As`.
- Don't `log.Fatal` from library code. Return an error; let `cmd/` decide.
- Don't swallow errors. If you really need to discard one, comment **why**.

## Context

- Functions doing I/O take `ctx context.Context` as the **first** parameter.
- Don't store contexts in structs. Pass them through call chains.
- Honour `ctx.Done()` in long-running loops (HTTP retries, batch imports).

## `init()` is reserved for registries

Don't use `init()` for setup that has observable side effects (env reads,
HTTP calls, global state mutation). The only legitimate use is registering
something with a hard-coded table — e.g. wiring a `web.Gateway` impl into
a registry keyed by version.

## Logging vs printing

- The CLI prints **user-facing** output to `stdout` / `stderr`.
- Diagnostic logs use `log/slog` with structured key-value pairs.
- Library packages (`internal/client/...`, `internal/importer/...`) **do
  not** print directly. They return data; `cmd/` and `internal/cli/` print.

## Tests

See `TESTING.md`. The short version:

- Standard library `testing` only — no testify, no ginkgo.
- Table-driven, with `t.Run(tc.name, ...)` subtests.
- Fixtures live in `testdata/<package>/<case>.{html,json}`.
- Never hit a real DOMjudge. Use `httptest.Server`.

## Naming

- Exported names follow Go conventions (`MixedCaps`, no underscores).
- Acronyms are uppercase: `URL`, `HTTPClient`, `JSONBody`.
- `interface` types describe behaviour, not implementations: `Gateway`,
  `Reader`, not `IGateway`, `WebReader`.
- File names mirror the primary type or concern they hold (`team.go`,
  `team_test.go`, `client.go`).

## Concurrency

- Prefer channels for ownership transfer; `sync.Mutex` for shared state.
- `errgroup.Group` is the default for fan-out work.
- No goroutine leaks: every spawned goroutine has a clear exit condition
  tied to a context.

## Public surface area

`internal/...` packages are not importable from outside the module. Until
we have a stable external API, keep everything under `internal/`.

## Comments

- Every exported identifier in `internal/` that survives Phase 1 should have
  a doc comment. Until then, focus on package-level docs in `doc.go`.
- Don't write what the code does — write **why**, especially for non-obvious
  behaviour preserved from the Python CLI (see ROADMAP Phase 1 quirks list).

## TSV / multipart conventions (Phase 1.5+)

The Phase 1.5 API client uploads bulk-import payloads as multipart form
data. Two house rules apply:

- **One helper per package.** All multipart POSTs in
  `internal/client/api/v4` go through `(*Client).postMultipart` in
  `multipart.go`. Don't hand-roll `multipart.Writer` per call site —
  the helper centralises boundary generation, `Content-Type` header
  assembly, and error wrapping. New endpoints add a method on the
  relevant `*Resource` type and call `c.postMultipart` with a
  `map[string]multipartField` payload.
- **TSV bodies use `\n` line endings, not `\r\n`.** The DOMjudge
  importer rejects CRLF on some payload variants, and CCS
  (`accounts.tsv`, `groups.tsv`, `teams.tsv`) is specified as LF-only.
  The TSV builders in `internal/importer/tsv.go` emit `\n` exclusively;
  any future TSV builder must do the same. Don't substitute
  `encoding/csv` with `Comma = '\t'` — `encoding/csv` writes CRLF on
  Windows builds and quotes fields containing tabs, both of which break
  the import endpoints.
