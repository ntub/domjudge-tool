# Testing

The testing strategy for `domjudge-tool` is deliberately small. The goal is
reliable signal, not assertion gymnastics.

## Framework

- **Standard library `testing`** + `testing/httptest`.
- **No testify or ginkgo.** Keep tests on the standard library to minimize
  the dependency surface.

## Style

- Table-driven tests with `t.Run(tc.name, ...)` subtests.
- One test function per behaviour, not per code path.
- Use `t.Helper()` in shared setup.
- `t.Cleanup()` for teardown — never `defer` in a helper.

Example skeleton:

```go
func TestParseTeamRow(t *testing.T) {
    tests := []struct {
        name    string
        in      string
        want    Team
        wantErr bool
    }{
        {name: "happy path", in: "...", want: Team{...}},
        {name: "missing column", in: "...", wantErr: true},
    }
    for _, tc := range tests {
        t.Run(tc.name, func(t *testing.T) {
            got, err := ParseTeamRow(tc.in)
            if (err != nil) != tc.wantErr {
                t.Fatalf("err = %v, wantErr = %v", err, tc.wantErr)
            }
            if !reflect.DeepEqual(got, tc.want) {
                t.Fatalf("got %+v, want %+v", got, tc.want)
            }
        })
    }
}
```

## Fixtures

- Live under `testdata/<package>/<case>.{html,json,csv}`.
- Loaded via `os.ReadFile(filepath.Join("testdata", ...))`.
- HTML fixtures cover DOMjudge **7.3.x** and **8.3.1**. Sanitize captures
  before committing; raw responses, session tokens, and personal data stay
  in ignored local locations.
- Don't generate fixtures from inside tests. They are inputs, not outputs.

## HTTP

- **Never** hit a real DOMjudge from a test.
- Use `httptest.NewServer` to mount handlers that return canned responses.
- Tests that exercise an `httptest.Server` are named `TestXxx_Integration`.

## Coverage targets (informal)

| Area                    | Target |
| ----------------------- | -----: |
| Parsers (HTML, CSV)     |  ≥ 80% |
| HTTP / web clients      |  ≥ 70% |
| CLI command handlers    | behavior and I/O contract tests |

Generate the report with `mise run cover`; output is `coverage.html`.

## Naming

- `TestXxx`             — pure unit tests, no `httptest.Server`.
- `TestXxx_Integration` — boots an `httptest.Server` or touches the
  filesystem (still hermetic — no real DOMjudge, no real SMTP).
- `Benchmark...`        — perf, run with `go test -bench=.` only when
  investigating a regression.

## Race detector

`mise run test-race` runs the full suite under `-race`. Run it before
submitting any PR that changes concurrent code.

## Contract testing (Phase 1.5+)

Contract tests pair a sanitized response fixture with its real client decoder
and consuming use-case. The same fixture is exercised at both layers so
response-shape drift is caught before it reaches user-visible workflows.
Capture is an authorized, separate operation; tests only read committed inputs.

Conventions:

- **Capture protocol:** `testdata/api/v4/CAPTURE_API.md` and
  `testdata/web/v7/CAPTURE.md` document fixture redaction. Use the maintained
  API/web PII scanner rather than old ad hoc grep checklists. Never
  recapture against production DOMjudge.
- **Privacy gate:** `mise run privacy-check` runs TruffleHog against the
  working tree and reachable git history, then runs the project-specific
  API fixture PII gates. TruffleHog is for secrets; the local PII gates
  remain responsible for known host, school, student, and course tokens.
- **Multipart contract tests** are a hybrid: instead of comparing
  decoder output against a fixture, they mount an `httptest.Server`
  whose handler parses the inbound multipart body with
  `mime/multipart.Reader` and asserts on field names, filenames, and
  byte contents. See `internal/client/api/v4/multipart_test.go` and
  `imports_test.go` for the canonical pattern.
- See `docs/phase1.5/CONTRACT.md` for the full protocol — fixture
  layout, sanitization rules, and the rule that the same JSON file is
  loaded by both the decoder test and the consuming use-case test.

## Desktop CLI integration

Installer tests use temporary bundle, home and state roots. Test observable
lifecycle transitions, foreign-file preservation, corrupt state, stale
revisions, no-replace creation, permission failures and broken targets.
Inspection must not create directories or execute discovered programs.
Handler tests cover cross-origin/Host/token rejection before filesystem
mutation and explicit disabled preview behavior. Run the entire affected
package, including existing Settings and template cases.

The Windows launcher is verified as a real subprocess: arguments including
empty strings, quotes and Unicode, stdin/stdout/stderr, cwd, environment and
exit codes must survive forwarding. Console cancellation additionally needs
a real Windows console. Cross-compilation is not native execution evidence.

Package smoke checks unpack artifacts, verify metadata/checksums and run
the packaged CLI's help, version and offline config generation. Run installed
commands with an isolated PATH and private installation roots; never use
the operator's real command directory as a test fixture.

Native acceptance covers macOS, Linux, Windows 11 ARM64, and Windows 10 x86_64.
VM tests use a standard user without Developer Mode; administrator CI jobs
alone cannot establish that no elevation is required. Store local VM paths,
credentials and captures only in ignored locations. Confirm legal terms and
security/account changes with the operator at the point of action.
