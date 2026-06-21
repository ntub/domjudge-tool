# Desktop UX Improvements — Specification

> Status: All P1–P6 DONE.
>
> All improvements share the same injectable-function pattern introduced in P1:
> production code is wired in `cmd/desktop/`, tests use mocks,
> handlers live in `internal/desktop/handler/`.

---

## Completed

### P1 — Contest Dropdown (3 screens)

**Screens:** Download Scoreboard, Download Submissions, Import Problems

**Before:** `contest_id` text input, user types the ID manually.

**After:** `GET /api/fragment/contests` fetches real contests from the server
after the user picks a Server Profile. The `#contest-fragment` div is replaced
via Datastar SSE with a `<select>` whose default option matches the profile's
saved `ContestID`. Falls back to text input when no profile is selected or the
API is unreachable.

**Key files:**
- `internal/desktop/templates/fragments.templ` — `ContestSelectFragment`, `ContestInputFragment`
- `internal/desktop/handler/api_fragment.go` — `ContestsFetchFunc`, `RegisterAPIFragment`
- `cmd/desktop/run_fragment.go` — production `desktopFetchContests` (build tag: `desktop`)

---

## Pending

### P2 — Set Password: Username Searchable Dropdown

**Screen:** Set Password (`/set-password`)

**Problem:** The `username` field is a free-text input. Users must remember or
look up valid usernames.

**Before:**
```
Username: [ text input — e.g. user1 ]
```

**After:**
After a Server Profile is selected, load the user list from
`GET /api/v4/users` and display a searchable `<select>` (or a datalist
pattern) so the user can pick a username without knowing it by heart.

**New API endpoint:**
```
GET /api/fragment/users?required=true
```
- Reads `profileId` signal (same pattern as P1)
- Injectable: `UsersFetchFunc func(ctx, profile) ([]model.User, error)`
- On success → `UsernameSelectFragment` (replaces `#username-fragment`)
- On error / no profile → `UsernameInputFragment` (text fallback)

**Template changes:**
- `set_password.templ`: add `data-signals='{"profileId": ""}'` on `<form>`;
  add `data-bind:profile-id` + `data-on:change` on the profile `<select>`;
  replace `<input name="username">` with `@UsernameInputFragment(...)`.
- `fragments.templ`: add `UsernameSelectFragment` and `UsernameInputFragment`.

**Handler / wiring:**
- `api_fragment.go`: add `RegisterAPIFragmentUsers` (or extend `RegisterAPIFragment`).
- `cmd/desktop/run_fragment.go`: add `desktopFetchUsers` using `apiv4.New(...).Users.List(ctx)`.
- `cmd/e2e-server/main.go`: nop fixture returning 2–3 fake users.

**Opts struct:** no change — `SetPasswordOpts.Username string` stays as-is;
the form POSTs the selected username as the `username` field value.

---

### P3 — Download Submissions: Problem IDs Tag Multi-Select

**Screen:** Download Submissions (`/download-submissions`)

**Problem:** `problem_ids` is a comma-separated text input. Users must know
the exact problem short-names.

**Before:**
```
Problem IDs (optional): [ text input — P1, P2 … ]
```

**After:**
After a Server Profile **and** a Contest are selected, load the problem list
from `GET /api/v4/contests/{id}/problems` and display a tag-style multi-select.

**New API endpoint:**
```
GET /api/fragment/problems?required=false
```
- Reads `profileId` and `contestId` signals
- Injectable: `ProblemsFetchFunc func(ctx, profile, contestID string) ([]model.Problem, error)`
- On success → `ProblemsTagSelectFragment` (replaces `#problems-fragment`)
- On error / missing context → `ProblemsTextInputFragment` (comma-sep fallback)

**UI component (`ProblemsTagSelectFragment`):**
A `<select multiple>` with DaisyUI styling, or a custom tag-input component:
- Fetched problems shown as selectable options (short-name + name)
- Selected items rendered as dismissible DaisyUI `badge` tags below the select
- Hidden `<input type="hidden" name="problem_ids">` holds the comma-joined value
  updated via a small inline `<script>` or Datastar signal

**Template changes:**
- `download_submissions.templ`: add `contestId` to `data-signals`;
  update `data-on:change` on the contest `<select>` to also trigger
  `@get('/api/fragment/problems')`;
  replace `<input name="problem_ids">` with `@ProblemsTextInputFragment(...)`.
- `fragments.templ`: add `ProblemsTagSelectFragment` and `ProblemsTextInputFragment`.

**Opts struct:** `DownloadSubmissionsOpts.ProblemIDs []string` — no change.
The hidden input sends comma-separated values; the handler already splits them.

---

### P4 — Remove Users / Export Users: Tag Multi-Select from API

**Screens:** Remove Users (`/remove-users`), Export Users (`/export-users`)

#### Remove Users — Include / Exclude usernames

**Problem:** Both `include` and `exclude` are comma-separated text inputs.

**Before:**
```
Include (usernames to delete): [ text input — user1, user2 … ]
Exclude (usernames to keep):   [ text input — user3, user4 … ]
```

**After:**
After a Server Profile is selected, load the user list (reuses `UsersFetchFunc`
from P2) and display two independent tag multi-selects.

**New endpoint:** Reuses `GET /api/fragment/users` from P2.

**New fragment:** `UsernameTagSelectFragment(users []model.User, fieldName, labelText string)`
- Renders a `<select multiple>` + tag badges + hidden input
- `fieldName` is `"include"` or `"exclude"` so one component covers both fields
- `id` follows the pattern `#users-include-fragment` / `#users-exclude-fragment`

**Template changes — `remove_users.templ`:**
- Add `data-signals='{"profileId": ""}'` on `<form>`
- Add `data-bind:profile-id` + `data-on:change="@get('/api/fragment/users?field=include'), @get('/api/fragment/users?field=exclude')"` on the profile `<select>`
- Replace `<input name="include">` and `<input name="exclude">` with the corresponding fragments

**Opts struct:** `DeleteOpts.Include []string`, `DeleteOpts.Exclude []string` — no change.

#### Export Users — Filter by User IDs

**Problem:** `ids` is a comma-separated text input of numeric IDs.

**Before:**
```
Filter by User IDs (optional): [ text input — 1, 2, 3 … ]
Filter by Team ID (optional):  [ text input — 42 ]
```

**After:**
- `ids` → tag multi-select using `UsersFetchFunc` (display `username (id)`)
- `team_id` → single `<select>` using a new `TeamsFetchFunc` (display team name)

**New API endpoints:**
- `GET /api/fragment/users` — reused from P2
- `GET /api/fragment/teams` — new, returns `[]model.Team`

**Opts struct:** `ExportOpts.IDs []string`, `ExportOpts.TeamID string` — no change.

---

### P5 — Directory Picker for Output Directory Fields

**Screens:** Download Problems (`/download-problems`), Download Submissions (`/download-submissions`)

**Problem:** `out_dir` requires the user to type an absolute filesystem path.

**Before:**
```
Output Directory: [ text input — /path/to/export_problems ]  [Download]
```

**After:**
```
Output Directory: [ text input — /path/to/export_problems ]  [Browse…]  [Download]
```

Clicking **Browse…** opens the native OS folder picker (Finder on macOS) and
fills the text input with the selected path — no typing required.

**New API endpoint:**
```
GET /api/pick-directory?signal=outDir
```
- Server-side: `exec.Command("osascript", "-e", "POSIX path of (choose folder)")` on Darwin
- On Linux: `zenity --file-selection --directory` (if available), else 204 No Content
- On success: returns a Datastar SSE event that sets the `outDir` signal value
- On cancel / error: returns 204 (no-op; the input stays as-is)

**Injectable function:**
```go
// DirectoryPickerFunc opens a native folder picker and returns the selected path.
// Returns ("", nil) when the user cancels.
type DirectoryPickerFunc func(ctx context.Context) (string, error)
```

**Template changes — `download_problems.templ` and `download_submissions.templ`:**
- Add `outDir` to `data-signals` on `<form>`
- Add `data-bind:out-dir` on the `<input name="out_dir">`
- Add a `<button type="button" data-on:click="@get('/api/pick-directory?signal=outDir')">Browse…</button>`
  next to the input

**Handler:**
- `internal/desktop/handler/api_pick_directory.go`
- `RegisterPickDirectory(mux, picker DirectoryPickerFunc)`
- `cmd/desktop/run_fragment.go`: `desktopPickDirectory` using `osascript` / `zenity`
- `cmd/e2e-server/main.go`: nop — returns a hardcoded path like `/tmp/e2e-out`

**Platform support matrix:**

| OS      | Method                                      | Fallback            |
|---------|---------------------------------------------|---------------------|
| macOS   | `osascript -e 'POSIX path of (choose folder)'` | text input       |
| Linux   | `zenity --file-selection --directory`       | text input (204)    |
| Windows | PowerShell `FolderBrowserDialog` (TODO)     | text input (204)    |

---

### P6 — Tag Multi-Select Pattern for All Filter / Only / Exclude Fields

**Screens:** Download Problems, and any screen not covered by P3/P4 that has
comma-separated filter inputs.

This is a generalisation of the tag UI pattern introduced in P3 and P4.

**Fields covered:**

| Screen               | Field         | Type         | Data source               |
|----------------------|---------------|--------------|---------------------------|
| Download Problems    | `only`        | Problem IDs  | `ProblemsFetchFunc` (P3)  |
| Download Problems    | `exclude`     | Problem IDs  | `ProblemsFetchFunc` (P3)  |
| Download Submissions | `problem_ids` | Problem IDs  | `ProblemsFetchFunc` (P3)  |
| Remove Users         | `include`     | Usernames    | `UsersFetchFunc` (P2/P4)  |
| Remove Users         | `exclude`     | Usernames    | `UsersFetchFunc` (P2/P4)  |
| Export Users         | `ids`         | User IDs     | `UsersFetchFunc` (P2/P4)  |

**Download Problems note:** This screen has no explicit `contest_id` field —
it uses the profile's saved `ContestID`. To enable the problems tag-select,
either:
1. Add an optional contest override field (matches P1 pattern), **or**
2. Derive the contest from the profile and skip the contest picker step.

Option 2 is simpler and consistent with the existing behaviour where
"leave blank to use profile contest ID" is the stated default.

**Shared UI component — `TagMultiSelect`:**

```html
<!-- fragments.templ (sketch) -->
<div id="{fragment-id}">
  <select multiple class="select select-bordered w-full" id="{field}-options"
          data-on:change="syncTags('{field}')">
    for _, item := range items {
      <option value={item.ID}>{item.Label}</option>
    }
  </select>
  <div id="{field}-tags" class="flex flex-wrap gap-1 mt-1">
    <!-- rendered by syncTags JS -->
  </div>
  <input type="hidden" name="{fieldName}" id="{field}-hidden"/>
</div>
<script>
function syncTags(field) {
  const sel = document.getElementById(field + '-options');
  const hidden = document.getElementById(field + '-hidden');
  const tagBox = document.getElementById(field + '-tags');
  const selected = [...sel.selectedOptions];
  hidden.value = selected.map(o => o.value).join(',');
  tagBox.innerHTML = selected.map(o =>
    `<span class="badge badge-neutral gap-1">${o.text}
       <button type="button" onclick="deselectTag('${field}','${o.value}')">✕</button>
     </span>`
  ).join('');
}
function deselectTag(field, value) {
  const sel = document.getElementById(field + '-options');
  [...sel.options].forEach(o => { if (o.value === value) o.selected = false; });
  syncTags(field);
}
</script>
```

The `<script>` block is an inline helper scoped to the fragment. No external
JS dependency is added.

---

## Implementation Order

| Priority | Item | Blocker           | Effort |
|----------|------|-------------------|--------|
| P2       | Set Password username dropdown | none | S |
| P5       | Directory picker (Browse…)     | none | S |
| P3       | Download Submissions problem tag-select | P1 contest signal | M |
| P6       | Download Problems problem tag-select   | P3 ProblemsFetchFunc | M |
| P4       | Remove Users / Export Users tag-select | P2 UsersFetchFunc | M |

S = ~half day, M = ~1 day

---

## Shared Infrastructure Needed

```go
// New injectable types (api_fragment.go or a new api_fragment_users.go)
type UsersFetchFunc    func(ctx context.Context, p profile.ServerProfile) ([]model.User, error)
type TeamsFetchFunc    func(ctx context.Context, p profile.ServerProfile) ([]model.Team, error)
type ProblemsFetchFunc func(ctx context.Context, p profile.ServerProfile, contestID string) ([]model.Problem, error)
type DirectoryPickerFunc func(ctx context.Context) (string, error)
```

All new fragment endpoints follow the pattern established in P1:
- `GET /api/fragment/{resource}` reads Datastar signals → patches `#{resource}-fragment`
- Production wired in `cmd/desktop/run_fragment.go` using `apiv4.New(...)`
- E2E server uses nop fixtures
- Unit tests in `internal/desktop/handler/api_fragment_*_test.go`

---

## Out of Scope

- P2/P4: Team picker — teams are implicit in the Remove Users operation (all
  teams of deleted users are deleted). No team filter field exists currently.
- Pagination: assume contest/user/problem lists fit in a single response for
  the target DOMjudge instance sizes.
- Offline / cache: no client-side caching of API responses; each profile
  selection triggers a fresh fetch.
