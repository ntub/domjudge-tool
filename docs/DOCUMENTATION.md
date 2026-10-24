# How we write docs

Meta-rules for the `docs/` tree. Read this before adding a new doc.

## Where things go

| File                 | Purpose                                                       |
| -------------------- | ------------------------------------------------------------- |
| `README.md`          | Front door for new users. Install + quick start.              |
| `CLAUDE.md`          | Shared coding-agent instructions. `AGENTS.md` symlinks here.  |
| `CONTRIBUTING.md`    | How to set up and submit a change.                            |
| `CHANGELOG.md`       | Release history. Each `## [X.Y.Z]` section is that release's notes. |
| `docs/ARCHITECTURE.md` | Layered design + package responsibilities.                  |
| `docs/CONVENTIONS.md`  | Go style + project additions.                               |
| `docs/TESTING.md`      | Test framework, fixtures, naming.                           |
| `docs/COMMITS.md`      | Conventional Commits cheat sheet.                           |
| `docs/ROADMAP.md`      | Current milestones, remaining work, and compatibility locks. |
| `docs/phase1.5/CONTRACT.md` | Current API/fixture and transport compatibility contracts. |
| `docs/phase2/CONTRACT.md` | DOMjudge 8.3.1 decoder, form, and safety contracts.        |
| `docs/CLI.md`          | Complete per-flag CLI reference (English, canonical).       |
| `docs/CLI.zh-TW.md`    | Traditional Chinese translation of `docs/CLI.md`.           |
| `docs/SNIPPETS.md`     | Copy-paste command examples (English).                      |
| `docs/SNIPPETS.zh-TW.md`| Traditional Chinese translation of `docs/SNIPPETS.md`.     |
| `docs/USER_GUIDE.md`    | Illustrated Web UI and CLI workflows (English).             |
| `docs/USER_GUIDE.zh-TW.md` | Traditional Chinese translation of the user guide.       |
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
- **`docs/ROADMAP.md`** — when a milestone or remaining delivery requirement
  changes. Keep completed implementation diaries out of the active roadmap.
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

## Keep current information

- Remove completed per-PR checklists, one-off validation logs, superseded
  proposals, and duplicate phase plans when maintained references cover the
  current implementation. Do not create archive copies; Git retains history.
- Preserve live API/fixture contracts, compatibility locks, security
  decisions, and release history. A document's age alone does not make its
  contract obsolete.
- Repair links and source-comment references when deleting documents.
  Current behavior belongs in the architecture, references, and manuals;
  pending work belongs in the roadmap without claims of unexecuted validation.

## ADRs

See `adr/README.md` for the lifecycle. The short version:

1. Copy `adr/template.md` to `adr/NNNN-short-title.md` (next number).
2. Fill in **Context**, **Decision**, **Consequences**.
3. Status starts at `Proposed`. Move to `Accepted` after merge.
4. Record changed decisions in a new ADR and update the maintained references.
   Remove superseded or redundant records once their still-applicable
   constraints are covered. Recover historical rationale from Git rather than
   retaining contradictory documents.
5. Never renumber remaining ADRs or reuse retired numbers.

## Things not to do

- Don't write multi-paragraph file headers in source code. Brief package
  comments only.
- Don't commit auto-generated docs (e.g. `godoc` HTML).
- Don't duplicate content between `README.md`, `CLAUDE.md`, and `docs/`. Pick
  one canonical home and link to it.
- Don't add diagrams as PNG/JPG. Use Mermaid in fenced blocks so diffs are
  readable.

## Illustrated user guides and automated review

- `docs/USER_GUIDE.md` and `docs/USER_GUIDE.zh-TW.md` are the paired
  illustrated Web UI/CLI manuals. Keep section order, commands, image
  paths, and safety guidance aligned; translate explanatory prose.
- `scripts/update_user_guide.py` owns the production workflow:
  `capture` builds and starts the isolated web UI preview, captures
  sanitized screenshots with a locally installed Chromium browser, and
  stops only the process it started; `check` validates the manuals and
  their local image/link references; `update` runs a documentation-only
  AI review. No production server or SMTP access is needed.
- Screenshots belong in `docs/images/user-guide/`. Use only the seeded
  preview with reserved example data; never copy live-validation captures
  or anything from an operator's profile database into the manuals.
- The Lefthook pre-commit documentation command inspects staged feature
  changes after formatting and linting. Documentation-only changes do not
  invoke an AI model. The agent reviews an isolated staged snapshot, not
  ignored files, local credentials, or unstaged source edits.
- The default reviewer is the installed, authenticated Claude Code CLI.
  It has read-only tools confined to the snapshot, no shell or network
  tools, no project hooks/plugins/MCP integrations, and returns structured
  proposed Markdown updates. Never bypass agent permission checks.
- Only `README.md`, both user guides, and both CLI references may be
  changed by the updater. Validate output paths and document structure
  before applying anything. Preserve pre-existing local documentation
  edits and refuse conflicting updates rather than overwriting them.
- AI updates are never staged or committed automatically. When generated
  changes need review, the hook stops the commit and asks the author to
  inspect and stage them. Missing tools/authentication, invalid output,
  timeouts, or failed checks fail visibly; none counts as a successful
  documentation review.
- AI review sends the selected, privacy-checked source snapshot and
  documentation to the configured provider and may incur usage charges.
  Keep credentials out of tracked source. Run `mise run privacy-check` before
  publishing manual text or screenshots.
