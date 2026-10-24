#!/usr/bin/env python3
"""Print the CHANGELOG.md section published as a release tag's notes.

A release tag must have the strict form vMAJOR.MINOR.PATCH and CHANGELOG.md
must contain a non-empty "## [MAJOR.MINOR.PATCH]" section for it. The release
workflow fails closed when either is missing, so every published release
carries reviewed changelog content.
"""

from __future__ import annotations

import argparse
import re
import sys
from pathlib import Path

TAG_PATTERN = re.compile(r"v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)")
FENCE_PATTERN = re.compile(r"^ {0,3}(`{3,}|~{3,})")
DEFAULT_CHANGELOG = Path(__file__).resolve().parent.parent / "CHANGELOG.md"


class ReleaseNotesError(Exception):
    """A tag or changelog that cannot produce release notes."""


def version_from_tag(tag: str) -> str:
    """Return MAJOR.MINOR.PATCH for a strict vMAJOR.MINOR.PATCH tag."""
    if TAG_PATTERN.fullmatch(tag) is None:
        raise ReleaseNotesError(
            f"invalid release tag {tag!r}: expected vMAJOR.MINOR.PATCH, e.g. v1.7.0"
        )
    return tag[1:]


def extract_section(changelog: str, version: str) -> str:
    """Return the body of the "## [version]" section, without its heading."""
    heading = re.compile(rf"^## \[{re.escape(version)}\](?:\s|$)")
    body: list[str] | None = None
    fence: str | None = None
    for line in changelog.splitlines():
        if fence is None and line.startswith("## "):
            if body is not None:
                break
            if heading.match(line):
                body = []
                continue
        if body is not None:
            body.append(line)
        match = FENCE_PATTERN.match(line)
        if match is not None:
            marker = match.group(1)
            if fence is None:
                fence = marker
            elif marker[0] == fence[0] and len(marker) >= len(fence):
                fence = None
    if body is None:
        raise ReleaseNotesError(f"CHANGELOG.md has no '## [{version}]' section")
    notes = "\n".join(body).strip()
    if not notes:
        raise ReleaseNotesError(f"CHANGELOG.md section '## [{version}]' is empty")
    return notes + "\n"


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("tag", help="release tag, vMAJOR.MINOR.PATCH")
    parser.add_argument("-o", "--output", type=Path, help="write notes to this file")
    parser.add_argument("--changelog", type=Path, default=DEFAULT_CHANGELOG)
    args = parser.parse_args(argv)
    try:
        version = version_from_tag(args.tag)
        notes = extract_section(args.changelog.read_text(encoding="utf-8"), version)
    except (ReleaseNotesError, OSError) as exc:
        print(f"release_notes.py: {exc}", file=sys.stderr)
        return 1
    if args.output is None:
        sys.stdout.write(notes)
    else:
        args.output.write_text(notes, encoding="utf-8")
    return 0


if __name__ == "__main__":
    sys.exit(main())
