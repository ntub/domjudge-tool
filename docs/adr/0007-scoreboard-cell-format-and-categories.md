# 0007. Scoreboard API export matches Python cell format and spans all categories

- **Status:** Accepted
- **Date:** 2026-06-02
- **Deciders:** @project-maintainer

## Context

`download-scoreboard` is API-first since Phase 1.5 (ADR 0006): it fetches
`/contests/{cid}/scoreboard` JSON and reconstructs the CSV via
`internal/scoreboard.Build`. The stated PR-2 goal was output
*byte-identical* to the Phase 1 Python scraper.

Validating the Go export against a live DOMjudge 7.3.x contest and against a
Phase 1 CSV produced by the Python CLI surfaced four defects:

1. **Score-cell format regression.** `Build.formatScoreCell` emits a solved
   cell as `"<num_judged> <time>"` (e.g. `1 91`). The Python/`Parse` format
   is `"<time>/<num_judged> try(ies)"` (e.g. `91/1 try`) for solved,
   `"<tries> try(ies)"` (e.g. `5 tries`) for tried-but-unsolved, and `""`
   for untried. The order is reversed and the `try`/`tries` word is
   missing. `build_test.go` asserts the wrong values (`"1 30"`, `"1 42"`),
   so the regression is locked by a test. Ground truth from the API: for a
   problem `solved` with `num_judged=1, time=91` the cell must render
   `91/1 try`; for `num_judged=1, !solved` it renders `1 try`; for
   `num_judged=0` it renders empty. `time` is in minutes; `tries` =
   `num_judged` (+ `num_pending` while a verdict is pending).

2. **Hidden / non-default-sortorder categories are dropped.** `Download`
   calls `/scoreboard` with empty options. DOMjudge segments the scoreboard
   by team-category `sortorder` and, absent a `sortorder` query parameter,
   returns only the lowest sortorder. A visible participant category
   (sortorder 0) is returned, but a hidden category — such as the built-in
   `System` group at a higher sortorder — is not. Live probing confirmed
   the only call that returns such teams is `?sortorder=N&allteams=true`
   (`allteams` requires jury privileges and "also shows invisible teams");
   filtering by `category` or `public` alone returns 0 rows for a hidden
   category. So the missing teams cannot be recovered by
   `public`/`category`/`allteams` alone — `sortorder` must be enumerated.

3. **No category provenance.** The CSV does not record which team category
   (group) a row belongs to, so a multi-category export is ambiguous.

4. **Spurious summary row.** `Build` appends a `"%d///%d"` summary row
   replicating a Python scraping quirk (`parse.go` `parseSummary`). The
   operator considers this a Phase 1 bug, not a feature.

## Decision

We corrected the API export to match the Python cell format, to span all
team categories, and to record category provenance, and we removed the
summary row.

- **Cell format.** `formatScoreCell` returns `"<time>/<tries> try(ies)"`
  for solved, `"<tries> try(ies)"` for tried-but-unsolved (`tries =
  num_judged + num_pending`), and `""` for untried, with correct
  `try`/`tries` pluralisation. This **restores** Phase 1 / Python parity.
  Unlike ADR 0005 — which preserves a Python *bug* operators depend on —
  this corrects a Go-only *divergence from* Python; the locked
  `build_test.go` expectations are updated to the correct format.

- **All categories.** `Download` lists groups, enumerates the distinct
  `sortorder` values that contain teams, and fetches `/scoreboard` once per
  sortorder with `?sortorder=N&allteams=true`, concatenating the rows in
  ascending sortorder (matching the jury web view, where each sortorder is
  its own ranked section). `ScoreboardListOpts` gains a `SortOrder *int`
  field. Each merged row's category is resolved against the group that
  matches the sortorder section it came from (not blindly
  `team.GroupIDs[0]`), so teams that belong to several groups are labelled
  and hidden-filtered correctly.

- **Category column.** A `Category` column is inserted **after `Score`**
  and before the problem columns, sourced from the group resolved above
  (falling back to the team's first group when no sortorder context is
  available). Final header: `Rank, TeamAffiliation, TeamName, SolvedCount,
  Score, Category, <problem labels…>`.

- **Default-on with opt-out.** All categories (including hidden ones) are
  exported by default. `--exclude-hidden` (default `false`) drops rows
  whose matching category group has `hidden = true`.

- **No summary row.** `Build` no longer appends the summary row.

### Alternatives considered

- **Keep byte-identical-to-Python (no Category, exclude hidden groups).**
  Rejected: the operator explicitly asked for complete category coverage
  and a provenance column.
- **Zero-fill the missing teams from `/teams`.** Rejected: the API already
  returns them with correct (zero) scores via `?sortorder=N&allteams=true`;
  synthesising rows would duplicate logic and risk drift from the API.
- **Derive Category from `organization` instead of `group`.** Rejected:
  organisation ≠ category. They can share the same string (e.g. when an
  organisation and a category are named identically) by coincidence, but in
  multi-group contests they diverge, and built-in/system teams may have no
  organisation at all.
- **Gate the new behaviour behind a `--legacy` format flag.** Rejected for
  now: the operator wants the new format as the default; a compatibility
  flag can be added later if a consumer needs the old shape.

## Consequences

- Positive: the API export's per-problem cells match the Python output;
  every team category is covered; each row records its category.
- Negative: the CSV schema changes — a new `Category` column, multiple
  `rank = 1` rows (one per sortorder section), and no summary row. This is
  an intentional, operator-requested divergence from strict Phase 1
  byte-parity, so it contradicts PR-2's "no user-visible change" claim and
  **must** be called out in release notes. `Download` now issues one
  `/scoreboard` request per populated sortorder instead of one.
- Follow-ups:
  - A cross-path equivalence test (`Build` output vs `Parse` output on
    equivalent fixtures) guards the cell format against future drift.
  - Update `docs/phase1.5/SPEC.md` and `README.md` for the new column and
    `--exclude-hidden` flag.

## References

- `docs/phase1.5/PR-2-scoreboard.md` — original (partly stale) scoreboard
  API spec.
- ADR 0005 — Python quirk preservation (contrast: preserve vs correct).
- ADR 0006 — API-first with scraper fallback.
- `internal/scoreboard/parse.go` — the locked Python cell format this
  restores parity with.
- Ground truth: a Phase 1 CSV from the Python CLI, plus live DOMjudge 7.3.x
  scoreboard API probing (`/scoreboard?sortorder=N&allteams=true`).
