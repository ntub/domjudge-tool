# 0007. Scoreboard API export matches Python cell format and spans all categories

- **Status:** Accepted
- **Date:** 2026-06-02
- **Deciders:** @project-maintainer

## Context

`download-scoreboard` fetches API JSON and reconstructs CSV through
`internal/scoreboard`. A single scoreboard request does not necessarily
contain every team: DOMjudge divides rankings by team-category `sortorder`
and defaults to the lowest section.

Complete jury exports need one request per populated sortorder, with
`allteams=true` to include hidden teams. Filtering by category or visibility
alone cannot recover omitted sections. A team may belong to multiple groups,
so its category must follow the section that produced the row.

The CSV keeps Python-compatible problem-cell text, while intentionally
extending the old export with a `Category` column and removing its scraped
summary row. These are separate compatibility choices: preserving the cell
format does not mean reproducing every legacy CSV artifact.

## Decision

The API export uses Python-compatible problem cells, covers all populated
category sections, records category provenance, and emits no summary row.

- **Cell format.** Solved cells with a time use
  `"<time>/<tries> try(ies)"`, where time is in minutes. Other attempted
  cells use `"<tries> try(ies)"`; untried cells are empty. Attempts include
  `num_judged + num_pending`, with singular/plural wording. A solved cell
  has at least one attempt; if its time is absent, only the attempt phrase
  is emitted.

- **All categories.** `Download` lists groups, enumerates the distinct
  `sortorder` values that contain teams, and fetches `/scoreboard` once per
  sortorder with `?sortorder=N&allteams=true`, concatenating the rows in
  ascending sortorder (matching the jury web view, where each sortorder is
  its own ranked section). `ScoreboardListOpts` has a `SortOrder *int`
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

- **Shared encoding.** CLI and Desktop use `scoreboard.WriteCSV`, whose
  CRLF line endings preserve the Python CSV dialect. Team display names
  retain the last-whitespace-token behavior shared with the legacy parser.

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
- **Gate category coverage behind a legacy format flag.** Rejected: the
  complete export is the selected default; maintaining another output
  schema is not part of this decision.

## Consequences

- Positive: the API export's per-problem cells match the Python output;
  every team category is covered; each row records its category.
- Negative: multiple rows can have `rank = 1`, because each sortorder is a
  separately ranked section. A multi-group team may appear in more than one
  section; consumers must use `Category` when interpreting the rank.
- Negative: `Download` issues one `/scoreboard` request per populated
  sortorder. Hidden-group exports require appropriate jury privileges.
- Compatibility: `Category` stays after `Score` and before problem columns;
  the absence of a summary row is intentional. Changes to this schema or
  cell grammar require an explicit compatibility decision.

## References

- [Current API and CSV contract](../phase1.5/CONTRACT.md).
- [CLI scoreboard options](../CLI.md#download-scoreboard).
- [Python deletion quirk](0005-python-quirk-preservation.md), a separate
  compatibility lock.
- [API-first routing](0006-api-first-with-scraper-fallback.md).
- [Category request/merge logic](../../internal/scoreboard/download.go#L19)
  (`internal/scoreboard/download.go:19`).
- [Cell formatting](../../internal/scoreboard/build.go#L150) and
  [shared CSV encoder](../../internal/scoreboard/csv.go#L9).
- [Legacy HTML parser](../../internal/scoreboard/parse.go), retained for
  parser compatibility tests rather than the current download path.
