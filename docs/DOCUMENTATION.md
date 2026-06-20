# How we write docs

Meta-rules for the `docs/` tree. Read this before adding a new doc.

## Where things go

| File                 | Purpose                                                       |
| -------------------- | ------------------------------------------------------------- |
| `README.md`          | Front door for new users. Install + quick start.              |
| `CLAUDE.md`          | Shared coding-agent instructions. `AGENTS.md` symlinks here.  |
| `CONTRIBUTING.md`    | How to set up and submit a change.                            |
| `CHANGELOG.md`       | Release history. Generated from Conventional Commits.         |
| `docs/ARCHITECTURE.md` | Layered design + package responsibilities.                  |
| `docs/CONVENTIONS.md`  | Go style + project additions.                               |
| `docs/TESTING.md`      | Test framework, fixtures, naming.                           |
| `docs/COMMITS.md`      | Conventional Commits cheat sheet.                           |
| `docs/ROADMAP.md`      | Phase 0/1/2/3 plan + carried-over notes.                    |
| `docs/PHASE1.md`       | Phase 1 plan + per-PR status (preserved history).           |
| `docs/CLI.md`          | Complete per-flag CLI reference (English, canonical).       |
| `docs/CLI.zh-TW.md`    | Traditional Chinese translation of `docs/CLI.md`.           |
| `docs/SNIPPETS.md`     | Copy-paste command examples (English).                      |
| `docs/SNIPPETS.zh-TW.md`| Traditional Chinese translation of `docs/SNIPPETS.md`.     |
| `docs/DOCUMENTATION.md`| This file.                                                  |
| `docs/adr/`            | Architecture Decision Records, one per file.                |

## When to update which doc

- **README.md** — only when a user-visible feature ships, or when install /
  setup steps change. Don't dump architecture into it.
- **CLAUDE.md** — when coding agents should be told something they currently
  guess wrong. Keep it short; defer detail to `docs/`. Keep `AGENTS.md` as a
  symlink to this file for Codex.
- **`docs/ARCHITECTURE.md`** — when a package is added, removed, or
  significantly reshaped.
- **`docs/CONVENTIONS.md`** — when a new style rule is agreed.
- **`docs/ROADMAP.md`** — at every phase boundary, or when forward-looking
  notes need to be captured so they aren't forgotten.
- **`docs/CLI.md` and `docs/CLI.zh-TW.md`** — whenever a command, flag,
  default, alias, or output format changes. Always update both files
  together to keep the translation in sync. The two files must remain
  structurally identical (same command order, same flag tables); translate
  prose, keep flag names and code examples byte-identical.
- **`docs/adr/NNNN-*.md`** — when an architectural decision is non-obvious,
  hard to reverse, or affects multiple packages.

## Style

- **Headings:** ATX (`#`, `##`), one H1 per file.
- **Code fences:** language tag is mandatory (`go`, `bash`, `json`, `toml`,
  `yaml`, `text`).
- **Links:** prefer relative links within the repo
  (`[architecture](docs/ARCHITECTURE.md)`); absolute URLs only for external
  references.
- **Line length:** wrap at ~80 columns where natural; don't fight Markdown
  rendering.
- **Tables:** allowed and encouraged for stable data; avoid for things that
  change every few weeks.
- **Tone:** declarative and present-tense. "The CLI loads…" not "The CLI
  will load…".

## ADRs

See `adr/README.md` for the lifecycle. The short version:

1. Copy `adr/template.md` to `adr/NNNN-short-title.md` (next number).
2. Fill in **Context**, **Decision**, **Consequences**.
3. Status starts at `Proposed`. Move to `Accepted` after merge.
4. To overturn an ADR, write a new ADR that supersedes it; mark the old one
   `Superseded by NNNN`.

## Things not to do

- Don't write multi-paragraph file headers in source code. Brief package
  comments only.
- Don't commit auto-generated docs (e.g. `godoc` HTML).
- Don't duplicate content between `README.md`, `CLAUDE.md`, and `docs/`. Pick
  one canonical home and link to it.
- Don't add diagrams as PNG/JPG. Use Mermaid in fenced blocks so diffs are
  readable.
