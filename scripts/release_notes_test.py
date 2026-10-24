#!/usr/bin/env python3

from __future__ import annotations

import contextlib
import io
import tempfile
import unittest
from pathlib import Path

import release_notes

CHANGELOG = """# Changelog

## [Unreleased]

- Pending work.

## [1.7.0] - 2026-10-24

Desktop release.

```text
## [1.6.0] inside a fence is content
```

### Fixes

- Fixed a thing.

## [1.6.0] - 2026-06-10

Older release.

## [1.5.0] - 2026-06-04

## [1.4.0] - 2026-06-03
"""


class VersionFromTagTest(unittest.TestCase):
    def test_accepts_strict_semver_tags(self) -> None:
        for tag, version in [("v1.7.0", "1.7.0"), ("v0.0.0", "0.0.0"), ("v10.20.30", "10.20.30")]:
            with self.subTest(tag=tag):
                self.assertEqual(version, release_notes.version_from_tag(tag))

    def test_rejects_other_tags(self) -> None:
        for tag in ["1.7.0", "v1.7", "v1.7.0-rc.1", "v01.7.0", "v1.7.0\n", "V1.7.0", "v1.7.0.1", ""]:
            with self.subTest(tag=tag):
                with self.assertRaises(release_notes.ReleaseNotesError):
                    release_notes.version_from_tag(tag)


class ExtractSectionTest(unittest.TestCase):
    def test_returns_only_the_requested_section(self) -> None:
        notes = release_notes.extract_section(CHANGELOG, "1.7.0")

        self.assertTrue(notes.startswith("Desktop release."))
        self.assertIn("## [1.6.0] inside a fence is content", notes)
        self.assertIn("- Fixed a thing.", notes)
        self.assertNotIn("Older release.", notes)
        self.assertNotIn("## [1.7.0]", notes)

    def test_last_section_runs_to_end_of_file(self) -> None:
        self.assertEqual("Older release.\n", release_notes.extract_section(CHANGELOG, "1.6.0"))

    def test_missing_section_fails(self) -> None:
        with self.assertRaisesRegex(release_notes.ReleaseNotesError, r"no '## \[1\.8\.0\]'"):
            release_notes.extract_section(CHANGELOG, "1.8.0")

    def test_version_prefix_does_not_match_longer_version(self) -> None:
        with self.assertRaises(release_notes.ReleaseNotesError):
            release_notes.extract_section("## [1.7.00] - 2026-10-24\n\nNotes.\n", "1.7.0")

    def test_empty_section_fails(self) -> None:
        with self.assertRaisesRegex(release_notes.ReleaseNotesError, "is empty"):
            release_notes.extract_section(CHANGELOG, "1.5.0")


class MainTest(unittest.TestCase):
    def test_writes_notes_to_output_file(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            changelog = Path(tmp) / "CHANGELOG.md"
            changelog.write_text(CHANGELOG, encoding="utf-8")
            output = Path(tmp) / "notes.md"

            code = release_notes.main(["v1.6.0", "--changelog", str(changelog), "-o", str(output)])

            self.assertEqual(0, code)
            self.assertEqual("Older release.\n", output.read_text(encoding="utf-8"))

    def test_invalid_tag_exits_nonzero_without_output(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            changelog = Path(tmp) / "CHANGELOG.md"
            changelog.write_text(CHANGELOG, encoding="utf-8")
            output = Path(tmp) / "notes.md"
            stderr = io.StringIO()

            with contextlib.redirect_stderr(stderr):
                code = release_notes.main(["v1.6", "--changelog", str(changelog), "-o", str(output)])

            self.assertEqual(1, code)
            self.assertFalse(output.exists())
            self.assertIn("invalid release tag", stderr.getvalue())


if __name__ == "__main__":
    unittest.main()
