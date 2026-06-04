# Architecture Decision Records

An ADR captures a single significant decision: **what** we chose, **why**,
and **what we accept by choosing it**. Future contributors read ADRs to
understand the shape of the codebase without having to re-derive it.

## When to write an ADR

Write one when:

- The decision is hard to reverse (module path, license, datastore choice).
- It affects multiple packages or phases.
- A reasonable engineer could disagree.
- We chose a non-default option and the next reader will ask why.

Don't write one for:

- Routine code style — that's `docs/CONVENTIONS.md`.
- Per-task implementation details — that's the PR description.
- Anything that would obviously be re-decided the same way every time.

## Lifecycle

1. **Proposed.** Open a PR adding `adr/NNNN-short-title.md` (next number,
   zero-padded to 4 digits). Status header reads `Proposed`.
2. **Accepted.** When the PR merges, the status flips to `Accepted` in the
   same commit, or in a follow-up housekeeping commit.
3. **Superseded.** Decisions get overturned by *new* ADRs. The new ADR
   notes which numbers it supersedes; the old ADR's status changes to
   `Superseded by NNNN`. We don't delete the old one.
4. **Deprecated.** A decision that is no longer relevant but isn't being
   replaced. Set status to `Deprecated` with a one-line reason.

## File layout

```
adr/
├── README.md         # this file
├── template.md       # copy this when creating a new ADR
├── 0001-go-module-path.md
└── NNNN-…
```

## Index

| ADR | Status | Title |
| --- | ------ | ----- |
| [0001](0001-go-module-path.md) | Accepted | Go module path |
| [0002](0002-tdd-collaboration-model.md) | Accepted | TDD collaboration model |
| [0003](0003-cobra-version-pin.md) | Accepted | Cobra version pin |
| [0004](0004-goquery-version-pin.md) | Accepted | goquery version pin |
| [0005](0005-python-quirk-preservation.md) | Accepted | Python quirk preservation |
| [0006](0006-api-first-with-scraper-fallback.md) | Accepted | API-first with scraper fallback |
| [0007](0007-scoreboard-cell-format-and-categories.md) | Accepted | Scoreboard cell format and categories |
| [0008](0008-domjudge-8.3.1-support-and-version-selection.md) | Proposed | DOMjudge 8.3.1 support and version selection |
| [0009](0009-api-v4-cross-version-response-compatibility.md) | Proposed | API v4 cross-version response compatibility |

## Style

- ATX headings.
- Use the template — don't invent your own structure.
- Wrap prose at ~80 columns.
- Link to related code, ADRs, and external sources.
- Past tense for the decision; present tense for consequences.
