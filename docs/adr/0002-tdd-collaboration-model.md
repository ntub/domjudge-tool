# 0002. TDD + Codex-implements / Claude-reviews collaboration model

- **Status:** Accepted
- **Date:** 2026-05-04
- **Deciders:** @project-maintainer

## Context

The Python CLI (`ntub-inc/domjudge-tool-cli` 0.3.0) had zero automated
tests, three known bugs, and one quirky behaviour the user base now
depends on. Migrating it to Go was the right moment to grow coverage from
zero rather than port the same fragile state.

Two questions needed answers before the first Phase 1 PR:

1. **Methodology**: write tests before or after code?
2. **Collaboration**: who writes code, who reviews, and how do they
   interact without a GitHub PR workflow?

Constraints at decision time:

- The project owner (@project-maintainer) wanted AI-assisted implementation but
  was explicit about reviewing every PR for correctness.
- The rewrite covers eight sequentially-dependent PRs; a heavyweight
  review cycle (GitHub PR + CI round trips) would slow iteration.
- Each PR carries a formal spec (`docs/phase1/PR-N-<slug>.md`) that
  must exist *before* the code is written — specs are the single source
  of truth for the review.

## Decision

**Test-Driven Development** (red → green → refactor) is mandatory for
every public function added in Phase 1. **Codex** drafts specs and
implements code; **Claude** drafts specs (when Codex-initiated) and
reviews diffs against the spec's tests + edge cases + docs checklist.
Reviews happen locally on `feat/p1-<slug>` branches; there are no GitHub
Pull Requests in Phase 1.

### Workflow per PR

1. **Spec**: Codex (or Claude) writes `docs/phase1/PR-N-<slug>.md`
   just-in-time before implementation.
2. **Review**: Claude reviews the spec against `PHASE1.md` and approves
   (or requests changes).
3. **Implementation**: Codex implements on `feat/p1-<slug>` following
   strict red → green → refactor commit cadence.
4. **Review**: Claude reviews the diff against the spec; findings go in
   `PR-N-review.md`. Approved branches merge to `main` via squash +
   `--no-ff`.

### Why TDD?

- Zero-test migration is a known risk vector: a test-after workflow
  tempts implementers to write tests that "prove" the current code
  rather than drive the design.
- The three Python bugs (`__init__.py:21`, `_problems.py:32`,
  `submissions.py:58`) were all caught and fixed by writing the failing
  test first — the bugs would have silently survived a test-after port.
- The Python quirk (`web/v7.py:161`) is locked by a named regression
  test (`TestDeleteUsers_PythonInvertedFilter_LocksKnownQuirk`) that
  prevents future "fixes".

### Alternatives considered

- **Test-after.** Rejected: high risk of tautological tests that pass
  vacuously; no forcing function to catch the known Python bugs.
- **GitHub Pull Requests.** Rejected: round-trip latency (push → CI →
  review comment → push again) would dominate a single-developer
  repo with no public contributors. Local branch review is faster and
  sufficient for the Phase 1 team of two (Codex + Claude).
- **No formal collaboration model** (Codex implements everything,
  Claude is silent). Rejected: Claude's review caught bugs and
  inconsistencies (see PR-4/PR-5 review logs) that Codex missed.

## Consequences

- **Positive**: Every public function has at least one test that drove
  its design. Coverage is tracked per package with minimums (≥ 70 % for
  CLI packages, ≥ 80 % for parsers).
- **Positive**: Python bugs are caught and locked by regression tests
  before Phase 1 ships.
- **Negative**: Slower initial implementation compared to test-after;
  each function requires two commits (red + green) instead of one.
- **Follow-up**: Phase 2 reuses this model for the v8 scraper port. If
  the project grows a public contributor base, replace local branch
  review with GitHub PRs while keeping TDD mandatory.

## References

- `docs/PHASE1.md` §"TDD discipline applied to every PR"
- `docs/PHASE1.md` §"TDD anchors — locked golden tests"
- `docs/phase1/README.md §Workflow`
- Phase 1 PR review logs: `PR-4-review.md` (implicit), `PR-5-review.md`
