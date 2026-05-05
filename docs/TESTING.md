# Testing

The testing strategy for `domjudge-tool` is deliberately small. The goal is
reliable signal, not assertion gymnastics.

## Framework

- **Standard library `testing`** + `testing/httptest`.
- **No testify, no ginkgo** in Phase 0. Reduces dependency surface; we can
  revisit if assertion verbosity becomes painful in Phase 1+.

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
- HTML fixtures are captured from real DOMjudge **7.3.2** and (Phase 2)
  **8.3.1** instances. Commit them verbatim, with a sanitization pass to
  strip session tokens and personal data.
- Don't generate fixtures from inside tests. They are inputs, not outputs.

## HTTP

- **Never** hit a real DOMjudge from a test.
- Use `httptest.NewServer` to mount handlers that return canned responses.
- Tests that exercise an `httptest.Server` are named `TestXxx_Integration`.

## Coverage targets (informal — not failing CI in Phase 0)

| Area                    | Target |
| ----------------------- | -----: |
| Parsers (HTML, CSV)     |  ≥ 80% |
| HTTP / web clients      |  ≥ 70% |
| CLI command handlers    | smoke-test only |

Generate the report with `just cover`; opens at `coverage.html`.

## Naming

- `TestXxx`             — pure unit tests, no `httptest.Server`.
- `TestXxx_Integration` — boots an `httptest.Server` or touches the
  filesystem (still hermetic — no real DOMjudge, no real SMTP).
- `Benchmark...`        — perf, run with `go test -bench=.` only when
  investigating a regression.

## Race detector

`just test-race` runs the full suite under `-race`. Run it before
submitting any PR that changes concurrent code.

## Contract testing (Phase 1.5+)

The Phase 1.5 API client adds a third test category beyond unit and
`_Integration` tests: **contract tests**. A contract test pairs a real
captured response from a live DOMjudge station — sanitized and stored
under `testdata/api/v4/<endpoint>/<case>.json` — with a Go decoder or
downstream transform (importer, scoreboard reconstructor, …). The same
fixture is consumed by both the client-side decoder and the use-case
that builds on top of it, so a server-shape change shows up at the
client layer first instead of leaking into a use-case test where the
root cause is harder to spot.

Conventions:

- **Capture protocol:** `testdata/api/v4/CAPTURE_API.md` documents the
  PII redaction rules and the script in `scripts/`. The 8 grep gates
  used for HTML fixtures (`testdata/web/v7/CAPTURE.md`) apply
  unchanged. Never re-capture against production DOMjudge — see CLAUDE.md.
- **Privacy gate:** `just privacy-check` runs TruffleHog against the
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
