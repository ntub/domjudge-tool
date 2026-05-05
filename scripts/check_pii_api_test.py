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
                '{"email":"user1@example.org","ip":"192.0.2.1",'
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
            4: "192.168.1.1",
            5: "person@private.example",
            6: "Bearer abcdefghijklmnopqrstuvwxyz",
            7: "password: supersecret",
            8: "_csrf_token: abcdefghi",
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


if __name__ == "__main__":
    unittest.main()
