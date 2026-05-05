# PR-1 — Foundation layer (`util` + `model` + `config`)

## Goal

Establish Layer 1 of the architecture: pure data types and helpers with
no business I/O beyond config file reads. Ship the three TDD anchors
that lock backward compatibility against the Python reference.

## Files created

### Source

| Path | Role |
|------|------|
| `internal/util/password/password.go` | `Generate(length, charset)` |
| `internal/util/csvio/csvio.go` | `StripBOM`, `NewReader`, `ReadAll`, `Write` |
| `internal/util/pathx/pathx.go` | `SafeFilename` |
| `internal/model/{affiliation,user,team,problem,submission,judgement,judgement_type}.go` | Pydantic-equivalent structs |
| `internal/model/doc.go` | Package doc |
| `internal/config/config.go` | `DomServerClient`, `Load`, `Save`, `MarshalCanonical`, `Validate` |

### Tests

| Path | TDD anchor? |
|------|-------------|
| `internal/util/password/password_test.go` | (unit only) |
| `internal/util/csvio/csvio_test.go` | `TestImportTemplateGolden_HeadersByteIdentical` |
| `internal/util/pathx/pathx_test.go` | `TestPathSafe_RejectsTraversalAndOSReservedChars` |
| `internal/model/model_test.go` | round-trip per type |
| `internal/config/config_test.go` | `TestDomServerClient_RoundTrip_Golden` |

### Fixtures

| Path | Origin |
|------|--------|
| `testdata/config/domserver_canonical.json` | Synthetic; canonical Go output, no real credentials |
| `testdata/config/domserver_minimal.json` | Synthetic; only required fields |
| `testdata/config/domserver_unknown_field.json` | Synthetic; exercises unknown-field warning |
| `testdata/config/domserver_invalid.json` | Synthetic; malformed JSON |
| `testdata/importer/template_python_golden.csv` | Byte-identical copy of Python `templates/csv/import-users-teams.csv` |
| `testdata/importer/with_bom.csv` | Synthetic; same content with UTF-8 BOM prefix |
| `testdata/model/*.json` | Synthetic; one per type, in canonical Go shape |

## Tests written first (RED)

Committed in `a179615 test(foundation): add failing tests + golden fixtures`.

| Test | Asserts |
|------|---------|
| `TestDomServerClient_RoundTrip_Golden` | Load + MarshalCanonical of `domserver_canonical.json` reproduces the exact bytes — locks snake_case JSON tags, field order, `null`-not-omitted for Optional, 4-space indent, no trailing newline. |
| `TestLoad_FillsDefaultsForMinimal` | When `affiliation_country`, `version`, `api_version` are omitted, `Load` substitutes the Python defaults (`"TWN"`, `"7.3.2"`, `"v4"`). |
| `TestLoad_AcceptsUnknownFieldAndWarns` | Captures `slog` output via a custom handler; asserts the warning names the unknown field. |
| `TestLoad_ReturnsErrorForMalformedJSON` | Returns a non-nil error on invalid JSON. |
| `TestLoad_ReturnsErrorForMissingFile` | Wraps `os.ErrNotExist` so `errors.Is` works. |
| `TestSave_WritesCanonicalAndRoundTrips` | `Save → ReadFile` matches the canonical golden bytes. |
| `TestSave_RejectsNil` | Returns error rather than panicking. |
| `TestValidate` | Required-field check matrix (host / username / password). |
| `TestImportTemplateGolden_HeadersByteIdentical` | The Python CSV template parses with the locked header order. |
| `TestReadAll_StripsBOMFromHeader` | `with_bom.csv` parses successfully. |
| `TestStripBOM` (table) | Strip / preserve / empty / partial-prefix behaviour. |
| `TestReadAll_RejectsHeaderMismatchOrder` | Wrong column order yields `ErrHeaderMismatch` naming the offender. |
| `TestReadAll_RejectsHeaderMismatchCount` | Wrong column count yields `ErrHeaderMismatch`. |
| `TestReadAll_EmptyInputErrors` | Empty input returns a non-nil error. |
| `TestWrite_RoundTrip` | `Write → ReadAll` reproduces input rows. |
| `TestPathSafe_RejectsTraversalAndOSReservedChars` | `../etc/passwd` produces no `/` or `..`; each of `/\:*?"<>|` is replaced. |
| `TestSafeFilename` (table) | Spaces, unicode, leading/trailing trim, NUL/DEL, only-reserved input. |
| `TestGenerate_DefaultLength10AsciiLettersDigits` | Defaults match Python `gen_password()`. |
| `TestGenerate_HonorsCustomLength` | Length passed through (1, 5, 32, 128). |
| `TestGenerate_HonorsCustomCharset` | Charset passed through (`AB` → only A/B). |
| `TestGenerate_RejectsNegativeLength` | Returns `ErrInvalidLength`. |
| `TestGenerate_RejectsEmptyCharset` | Returns `ErrEmptyCharset` (improvement over Python). |
| `TestGenerate_ZeroLengthSubstitutesDefault` | length=0 → DefaultLength. |
| `TestGenerate_NegativeLengthBeforeEmptyCharset` | Negative-length error precedes charset error. |
| `TestGenerate_HandlesUTF8Charset` | rune count, not byte count. |
| `TestX_RoundTripGoldenJSON` (one per model type) | Canonical fixture round-trips byte-identically. |
| `TestUser_OptionalFieldsAreNullableNotOmitted` | All Optional fields keep their key with `null` value. |
| `TestCreateUser_FromUser` / `TestCreateUser_FromUser_HonoursOverrides` | Equivalent of `CreateUser.from_user(user, **kwargs)`. |
| `TestTeam_PreservesGroupIDsOrder` | `group_ids` stay in input order. |

Implementation committed in `c0cbdfe feat(foundation): util + model + config with backward-compat anchors`.

## Edge cases covered

- **Empty / single / multi-rune charsets** in `password.Generate`.
- **UTF-8 BOM present / absent / partial** in `csvio.StripBOM` and `NewReader`.
- **Header drift** in `csvio.ReadAll`: wrong column count, wrong column order, missing header (empty input).
- **Traversal** (`../`) in `pathx.SafeFilename` — every byte of the
  Windows-reserved set neutralised.
- **Optional-then-null** vs. **omitted** distinction in
  `internal/model` — pointer types without `omitempty` keep keys present.
- **Missing config file** wraps `os.ErrNotExist`.
- **Malformed JSON** returns a wrapped error.
- **Unknown field** is accepted but logged via `slog.Warn`.
- **Nil pointer to `Save`** returns an error rather than panicking.
- **`json.Encoder` trailing newline** is stripped to match
  `json.MarshalIndent` so the canonical golden round-trips byte-exact.

## Definition of done

- [x] All RED tests fail at HEAD~1 (verified via `go test ./...` against
  the stub commit).
- [x] All tests pass at HEAD with `go test -race ./... -count=1`.
- [x] Coverage ≥ 80 % for parsers (config 91.2 %, csvio 85.3 %, model
  100 %, pathx 100 %, password 93.3 %).
- [x] `go vet ./...` clean.
- [x] No `init()` outside registry wiring (none exist).
- [x] No `fmt.Print*` from library code.
- [x] Every exported symbol has a doc comment; package-level doc comment
  on every package.
- [x] No real DOMjudge URLs, credentials, or PII in any fixture.
- [x] Commit history shows `test(...): add failing tests` BEFORE
  `feat(...): implement` (TDD audit).

## Cross-references

- Python source: `domjudge_tool_cli/{utils/password.py, models/*.py,
  models/domserver.py, templates/csv/import-users-teams.csv,
  commands/problems/_problems.py:32, commands/problems/__init__.py:21}`.
- Architecture: `docs/ARCHITECTURE.md` Layer 1 packages.
- Related ADRs: none in this PR (architectural decisions captured in
  PR-8: ADRs 0002–0005).
- Conventions: `docs/CONVENTIONS.md`, `docs/TESTING.md`,
  `docs/COMMITS.md`.

## Conventional Commit (merge)

```
feat(foundation): util + model + config with backward-compat anchors
```

## Notes for downstream PRs

- PR-2 (API v4 client) imports `internal/model` and `internal/config`.
- PR-3 (web v7) imports `internal/util/pathx` for filename safety on
  problem ZIP downloads, and the model types.
- PR-4 (importer) imports `internal/util/csvio` and `internal/model`.
- PR-6 (CLI) imports `internal/config` for `Load` and `Save`.
