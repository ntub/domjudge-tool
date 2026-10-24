# Architecture Decision Records

An ADR captures a significant, active decision: **what** we chose, **why**,
and **what we accept by choosing it**. This directory explains the current
system and its compatibility constraints; it is not a development diary.

## When to write an ADR

Write one when:

- The decision is hard to reverse, such as a datastore or transport choice.
- It affects multiple packages or user-facing compatibility contracts.
- A reasonable engineer could disagree.
- We chose a non-default option and the next reader will ask why.

Don't write one for:

- Routine code style — see [conventions](../CONVENTIONS.md).
- Per-task plans, spike logs, review transcripts, or completed bootstrap work.
- Anything that would obviously be re-decided the same way every time.

## Lifecycle

1. **Proposed.** Add `NNNN-short-title.md` using the next unused number,
   zero-padded to four digits. A proposal may already be implemented in the
   working tree; its status does not establish that it has merged.
2. **Accepted.** Mark a decision accepted when its approving change merges.
   Keep its context, constraints, and implementation description current.
3. **Retired.** Delete obsolete, superseded, or no-longer-useful records.
   Preserve any still-active compatibility or security rationale in a
   retained decision or the current reference documentation, and repair
   links in the same cleanup.

Keep only active decisions and proposals. Do not retain contradictory pins,
completed plans, tombstones, or archive copies for historical value. Removed
records are recoverable from Git history.

ADR numbers are stable identifiers. Never renumber retained files or reuse
a retired ID. A replacement decision identifies the prior numbers where
that relationship explains the current contract; it does not require the
retired documents to remain in this tree.

Decision status, implementation, acceptance evidence, and release status are
separate facts. In particular, local native acceptance does not establish
that an artifact CI workflow ran or that a public release exists.

## File layout

```text
adr/
├── README.md
├── template.md
├── 0005-python-quirk-preservation.md
└── NNNN-short-title.md
```

## Index

| ADR | Status | Title |
| --- | ------ | ----- |
| [0005](0005-python-quirk-preservation.md) | Accepted | Python quirk preservation |
| [0006](0006-api-first-with-scraper-fallback.md) | Accepted | API-first with scraper fallback |
| [0007](0007-scoreboard-cell-format-and-categories.md) | Accepted | Scoreboard cell format and categories |
| [0008](0008-domjudge-8.3.1-support-and-version-selection.md) | Accepted | DOMjudge 8.3.1 support and version selection |
| [0009](0009-api-v4-cross-version-response-compatibility.md) | Accepted | API v4 cross-version response compatibility |
| [0010](0010-problem-package-contest-scoping-and-import.md) | Accepted | Problem package contest scoping and import |
| [0011](0011-import-problems-upsert.md) | Accepted | Import problems upsert |
| [0012](0012-phase3-local-datastore.md) | Accepted | Desktop local datastore: GORM and SQLite |
| [0013](0013-phase3-frontend-stack.md) | Accepted | Desktop frontend: Wails, templ, and Datastar |
| [0015](0015-security-runtime-and-dependency-upgrades.md) | Accepted | Security runtime and dependency upgrades |
| [0016](0016-desktop-bundled-cli.md) | Proposed | Desktop-bundled CLI and per-user command integration |
| [0017](0017-mise-toolchain-and-tag-release.md) | Proposed | mise toolchain, tasks, and tag-only release pipeline |

## Style

- ATX headings.
- Use the template — don't invent your own structure.
- Wrap prose at ~80 columns.
- Link to related code, ADRs, and external sources.
- Use present tense for current decisions and consequences.
