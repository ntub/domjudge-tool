# Commit messages

This project uses **[Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/)**.
The `commit-msg` lefthook hook enforces the format locally; CI re-checks.
`git-cliff` reads commits to generate `CHANGELOG.md`, so getting this right
matters.

## Format

```
<type>(<scope>)?: <subject>

<body>            # optional, blank line above

<footer>          # optional, blank line above
```

- **type** — one of: `feat`, `fix`, `docs`, `style`, `refactor`, `perf`,
  `test`, `build`, `ci`, `chore`, `revert`.
- **scope** — optional package or area, e.g. `cli`, `web/v7`, `importer`.
- **subject** — imperative mood, no trailing period, ≤ 72 chars.
- **body** — wrap at ~72 chars; explain the *why*.
- **footer** — `Closes #123`, `Co-Authored-By: …`, etc.

## Breaking changes

Mark them either with `!` after the type/scope, or a `BREAKING CHANGE:`
footer. **Both is fine, but the `!` is what shows up in the changelog.**

```
feat(cli)!: rename --user to --username

BREAKING CHANGE: --user is removed; use --username.
```

## Type cheat-sheet

| Type       | When to use                                                 | In changelog?            |
| ---------- | ----------------------------------------------------------- | ------------------------ |
| `feat`     | New user-visible capability                                 | Features                 |
| `fix`      | Bug fix                                                     | Fixes                    |
| `perf`     | Performance improvement, no behaviour change                | Performance              |
| `refactor` | Internal restructure, no behaviour change                   | Refactor                 |
| `docs`     | Documentation only                                          | Documentation            |
| `test`     | Tests only                                                  | Testing                  |
| `build`    | Build system, dependencies, release tooling                 | Build                    |
| `ci`       | CI configuration                                            | CI                       |
| `chore`    | Misc (e.g. version bumps, repo housekeeping)                | Chore                    |
| `style`    | Whitespace / formatting only                                | (filtered)               |
| `revert`   | Revert a previous commit                                    | Reverts                  |

## Examples

```
feat(importer): support BOM-prefixed CSV input

Strips a leading UTF-8 BOM (﻿) from the first column header so
spreadsheet exports from Excel can be imported directly.

Closes #42
```

```
fix(web/v7): tighten path sanitization for problem export

Replace single-character space substitution with a path-safe filter that
also strips slashes and reserved characters, fixing a directory traversal
hole in --output.

Closes #47
```

```
chore: bootstrap project (phase 0)
```

## Anti-patterns

- "wip", "fix stuff", "update files" — rejected by the hook and useless in
  the changelog.
- Multiple unrelated changes squashed into one commit. Split them.
- Including the issue title verbatim — paraphrase.
- Putting the *what* in the subject and the *why* nowhere. Use the body.
