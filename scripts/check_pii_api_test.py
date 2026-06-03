#!/usr/bin/env python3

from __future__ import annotations

import contextlib
import io
import tempfile
import unittest
import zipfile
from pathlib import Path

import check_pii_api


class CheckPIIAPITest(unittest.TestCase):
    SENSITIVE_OWNER = "jason" "xie"

    def test_clean_fixture_exits_zero(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            fixture = Path(tmp) / "clean.json"
            fixture.write_text(
                '{"email":"user1@example.org","ip":"203.0.113.10",'
                '"password":"REDACTED","_csrf_token":"TEST_CSRF_TOKEN"}\n',
                encoding="utf-8",
            )

            self.assertEqual([], check_pii_api.check_paths([fixture]))
            stdout = io.StringIO()
            with contextlib.redirect_stdout(stdout):
                self.assertEqual(0, check_pii_api.main([str(fixture)]))

    def test_dirty_fixtures_report_each_gate(self) -> None:
        cases = {
            1: f"https://domjudge7.{self.SENSITIVE_OWNER}.net/api/v4",
            2: "student 11446001",
            3: "\u9673\u5ba5\u8fb0",
            4: "198.51.100.7",
            5: "person@private.example",
            6: "Bearer abcdefghijklmnopqrstuvwxyz",
            7: "password: supersecret",
            8: "_csrf_token: abcdefghi",
            9: "Authorization: Basic dXNlcjpwYXNzd29yZA==",
        }
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            for gate_id, text in cases.items():
                fixture = root / f"dirty_{gate_id}.json"
                fixture.write_text(text, encoding="utf-8")
                with self.subTest(gate_id=gate_id):
                    findings = check_pii_api.check_paths([fixture])
                    self.assertTrue(
                        any(finding.gate_id == gate_id for finding in findings),
                        findings,
                    )
                    stderr = io.StringIO()
                    with contextlib.redirect_stderr(stderr):
                        self.assertEqual(1, check_pii_api.main([str(fixture)]))

    def test_zip_fixture_contents_are_scanned(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            fixture = Path(tmp) / "submission_files.zip"
            with zipfile.ZipFile(fixture, "w") as archive:
                archive.writestr("solution.cpp", "// contact: person@private.example\n")

            findings = check_pii_api.check_paths([fixture])

            self.assertTrue(any(finding.gate_id == 5 for finding in findings), findings)

    def test_web_fixture_suffixes_are_scanned(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            html = root / "page.html"
            headers = root / "response.headers"
            html.write_text('<a href="https://live-station.com/">station</a>\n', encoding="utf-8")
            headers.write_text("X-Contact: person@private.example\n", encoding="utf-8")

            findings = check_pii_api.check_paths([root])

            self.assertTrue(any(finding.gate_id == 1 for finding in findings), findings)
            self.assertTrue(any(finding.gate_id == 5 for finding in findings), findings)

    def test_cloudflare_email_outside_example_org_fails(self) -> None:
        findings = check_pii_api.scan_text(
            "inline.html",
            '<a class="__cf_email__" '
            'data-cfemail="12676177602352667761663f616673667b7d7c3c776a737f627e773c7d6075">'
            "[email protected]</a>",
        )

        self.assertTrue(any(finding.gate_id == 5 for finding in findings), findings)

    def test_cloudflare_email_at_example_org_passes(self) -> None:
        findings = check_pii_api.scan_text(
            "inline.html",
            '<a class="__cf_email__" '
            'data-cfemail="12676177602352776a737f627e773c7d6075">'
            "[email protected]</a>",
        )

        self.assertEqual([], findings)

    def test_extended_hostname_tlds_are_scanned(self) -> None:
        findings = check_pii_api.scan_text("inline.html", '<a href="https://something.me/">x</a>')

        self.assertTrue(any(finding.gate_id == 1 for finding in findings), findings)

    def test_allowed_public_host_passes(self) -> None:
        findings = check_pii_api.scan_text(
            "inline.html", '<a href="https://www.domjudge.org/">DOMjudge</a>'
        )

        self.assertEqual([], findings)

    def test_asset_filename_passes(self) -> None:
        findings = check_pii_api.scan_text("inline.html", '<link href="bootstrap.min.css">')

        self.assertEqual([], findings)


if __name__ == "__main__":
    unittest.main()
