# PR-2 — HTTP transport + API v4 client

**Status: merged**
**Branch: `feat/p1-api-v4` → `main`**

## Goal

Provide a typed Go client for DOMjudge 7.3.2's REST API v4 so that PR-4
(importer/exporter) and PR-7 (CLI submissions/contest-files) can read users,
teams, problems, submissions, judgements, and judgement types without ever
touching the web UI scraper.

## Files created

```
internal/client/httpx/httpx.go          HTTP transport: Basic Auth, TLS skip,
                                         no-redirect, configurable timeout
internal/client/httpx/httpx_test.go
internal/client/api/v4/errors.go        HTTPError + sentinel vars
internal/client/api/v4/errors_test.go
internal/client/api/v4/client.go        v4.Client + getJSON / getStream helpers
internal/client/api/v4/query.go         buildQuery (ids[]=a&ids[]=b encoding)
internal/client/api/v4/general.go       GeneralResource.Version
internal/client/api/v4/general_test.go
internal/client/api/v4/users.go         UsersResource.List / .Get
internal/client/api/v4/users_test.go
internal/client/api/v4/teams.go         TeamsResource.List / .Get
internal/client/api/v4/teams_test.go
internal/client/api/v4/problems.go      ProblemsResource.List / .Get
internal/client/api/v4/problems_test.go
internal/client/api/v4/submissions.go   SubmissionsResource.List/.Get/.Files/.SourceCode
internal/client/api/v4/submissions_test.go
internal/client/api/v4/judgements.go    JudgementsResource.List / .Get
internal/client/api/v4/judgements_test.go
internal/client/api/v4/judgement_types.go   JudgementTypesResource.List / .Get
internal/client/api/v4/judgement_types_test.go
testdata/api/v4/version.json
testdata/api/v4/users_list.json
testdata/api/v4/user.json
testdata/api/v4/teams_list.json
testdata/api/v4/team.json
testdata/api/v4/problems_list.json
testdata/api/v4/problem.json
testdata/api/v4/submissions_list.json
testdata/api/v4/submission.json
testdata/api/v4/source_code.json
testdata/api/v4/submission_files.zip
testdata/api/v4/judgements_list.json
testdata/api/v4/judgement.json
testdata/api/v4/judgement_types_list.json
testdata/api/v4/judgement_type.json
```

## Architecture

`httpx.Client` wraps `*http.Client` with:
- Per-request `Authorization: Basic <base64>` header injection
- Optional `TLS InsecureSkipVerify` (user-controlled, fixes Python
  api_client.py:48-55 constructor bug that silently dropped `disable_ssl`)
- No automatic redirects (matching Python API client)
- Configurable timeout (default 30 s)

`v4.Client` holds an `*httpx.Client` and exposes seven sub-resource fields
(`c.Users.List(ctx, opts)`). The `/api/v4` prefix is built once on
construction. Two internal helpers (`getJSON`, `getStream`) handle status
checking, body reading, and error wrapping.

## Behavioural deviations from Python

| Deviation | Python | Go |
|-----------|--------|----|
| Constructor bug | `api_client.py:48-55` drops `disable_ssl`/`timeout`/`limits` | All options applied correctly |
| TLS config | Silently ignored | `http.Transport.TLSClientConfig.InsecureSkipVerify` |
| Error types | `raise_for_status()` | `*HTTPError` with `errors.Is` sentinels |
| `Files` nil path | `submissions.py:58` panics | Returns `ErrInvalidArgument` immediately |

## TDD anchors

| Test | Location | What it locks |
|------|----------|---------------|
| `TestSubmissionsFiles_NilFilePath_DoesNotPanic` | `submissions_test.go` | Python `submissions.py:58` panic on `None` file_path |
| `TestClient_BasicAuthHeaderOnEveryRequest` | `httpx_test.go` | Auth header on every method |
| `TestClient_DisableSSLFlipsInsecureSkipVerify` | `httpx_test.go` | TLS config applied |
| `TestClient_ContextCancelled_AbortsRequest` | `httpx_test.go` | Context propagation |
| `TestClient_TimeoutHonoured` | `httpx_test.go` | Timeout config applied |

## Coverage

| Package | Coverage |
|---------|----------|
| `internal/client/httpx` | 84.2 % |
| `internal/client/api/v4` | 92.5 % |

Both exceed the ≥ 70 % target.

## Commit history

```
test(api/v4): add failing tests + fixtures for v4 client
feat(api/v4): http transport + typed json client for domjudge 7.3.2 api
docs(phase1): PR-2 api/v4 spec + status update
```

## Definition of done

- [x] `gofmt -s / go vet` clean
- [x] `golangci-lint run` 0 issues
- [x] `go test -race ./internal/client/...` green
- [x] Coverage ≥ 70 % on both packages
- [x] `TestSubmissionsFiles_NilFilePath_DoesNotPanic` present and passes
- [x] commit history: `test(...)` → `feat(...)` → `docs(...)`
- [x] No real DOMjudge hit from any test (all `httptest.NewServer`)
- [x] No `testify` or `ginkgo` imports
