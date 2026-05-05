# 0004. goquery pinned at v1.10.3 for Go 1.23 compatibility

- **Status:** Accepted
- **Date:** 2026-05-04
- **Deciders:** @project-maintainer

## Context

The web scraper (`internal/client/web/v7`) parses DOMjudge jury HTML
pages. The Python equivalent used `BeautifulSoup`; the Go port uses
`goquery`, which provides a jQuery-like API over Go's `html` package.

The same `go 1.23.0` module directive constraint that affected Cobra
(ADR-0003) applies here:

- **goquery v1.10.3** is the latest release that declares `go 1.22`
  and is therefore compatible with a `go 1.23` module.
- **goquery v1.12.0** and later bumped the Go directive to `go 1.25`,
  which would force our module to `go 1.25` as well.

The constraint is tracked **separately from ADR-0003** because:

1. The unpin trigger is independent: goquery's parser internals (HTML5
   tokeniser, `cascadia` selector engine) change faster than Cobra's
   command-wiring API. A minor goquery update might affect HTML
   parsing behaviour in ways that require fixture updates even if the
   API surface is unchanged.
2. The `cascadia` v1.3.3 transitive dependency follows the same
   `go 1.22` constraint; upgrading goquery to v1.12+ also upgrades
   `cascadia`, which could silently alter CSS selector matching.

## Decision

goquery is pinned at **v1.10.3** in `go.mod`. The pin is explicit and
will be reassessed independently of ADR-0003.

```
require (
    github.com/PuerkitoBio/goquery v1.10.3
    github.com/andybalholm/cascadia v1.3.3 // transitive
)
```

### Alternatives considered

- **goquery v1.12.x (latest at decision time).** Rejected: requires
  `go 1.25`; see Context.
- **`golang.org/x/net/html` directly** (no goquery). Rejected: the
  resulting code is significantly more verbose and harder to maintain
  than the selector-based approach; the team reviewed the PR-3 scraper
  and found goquery's API a good fit for the DOMjudge HTML structure.
- **`htmlquery` (XPath-based).** Rejected: CSS selectors (via
  goquery) are more familiar and more consistent with the JavaScript
  tooling used in Phase 3 (TanStack, shadcn/ui).

## Consequences

- **Positive**: `go 1.23.0` directive stays stable; scraper tests stay
  reproducible against the pinned parser version.
- **Negative**: goquery v1.12+ parser improvements (if any) unavailable
  until the pin is lifted.
- **Unpin trigger**: When the `go` directive is raised to ≥ 1.25,
  unpin goquery independently of Cobra. Before unpinning, re-run all
  scraper tests against the current `testdata/web/v7/` fixtures and
  confirm the HTML output is byte-identical. If fixtures need updating,
  re-run the 8 PII grep gates from `testdata/web/v7/CAPTURE.md`.

## References

- `go.mod` — current `require` stanza.
- `CLAUDE.md` §"Tech stack" — "Pinned at v1.10.3".
- `docs/PHASE1.md` §"Deviations" §2 — Go directive stays at 1.23.
- `testdata/web/v7/CAPTURE.md` — HTML fixture capture and PII gates.
- ADR-0003 — Cobra is in the same situation; tracked separately.
