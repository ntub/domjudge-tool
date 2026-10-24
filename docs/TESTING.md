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

## Web UI and installer testing

Web UI tests use in-memory SQLite stores (`internal/webui/db.Open(":memory:")`),
mock operation functions, and `net/http/httptest`. Security tests cover
authentication enforcement (token vs basic vs none), loopback Host header
validation (DNS rebinding protection), CSRF cross-origin state-changing
request rejection, and security headers. Run the entire `internal/webui/...`
tree with `go test ./internal/webui/...`.

Installer tests verify `scripts/install.sh` and `scripts/install.ps1`:
- Test SHA-256 checksum verification against `checksums.txt` and abort on mismatch.
- Test download fallback (`curl` and `wget` on unix; PowerShell `Invoke-RestMethod` on Windows).
- Test environment overrides: `DOMJUDGE_TOOL_VERSION`, `DOMJUDGE_TOOL_INSTALL_DIR`, and `DOMJUDGE_TOOL_NO_MODIFY_PATH=1`.
- Verify user PATH configuration without requiring administrator privileges.
- Verify atomic binary installation and `<installed> version` execution.

Isolated preview tests run `cmd/e2e-server` with in-memory data for deterministic
manual check and screenshot validation via `scripts/update_user_guide.py check`.
