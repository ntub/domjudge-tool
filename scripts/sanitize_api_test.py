#!/usr/bin/env python3

from __future__ import annotations

import json
import tempfile
import unittest
from pathlib import Path

import sanitize_api


class SanitizeAPITest(unittest.TestCase):
    SENSITIVE_OWNER = "jason" "xie"
    SENSITIVE_SCHOOL = "nt" "ub"

    def test_sanitize_tree_rewrites_json_pii_and_preserves_layout(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            raw = root / "raw"
            out = root / "out"
            (raw / "contest_2").mkdir(parents=True)
            sample = {
                "url": f"https://domjudge7.{self.SENSITIVE_OWNER}.net/api/v4/users?email=learner@{self.SENSITIVE_SCHOOL}.edu.tw",
                "homepage": "http://onlinejudge." + self.SENSITIVE_SCHOOL + ".edu.tw/path?q=1",
                "remote_ip": "203.0.113.10",
                "ip": "192.168.1.1",
                "last_ip": "10.0.0.5",
                "ipv6": "2001:0:fe00::123",
                "contest_time": "1:00:05",
                "email": f"learner@{self.SENSITIVE_SCHOOL}.edu.tw",
                "name": "\u9673\u5ba5\u8fb0",
                "course": "1140202\u56db\u6280\u8cc7\u7ba1\u4e00\u7532",
                "student": "11446001",
                "_csrf_token": "9KZ3abcdEfg",
                "phone": "+886 2 1234 5678",
                "affiliation": "NT" "UB",
                "last_login_time": "2023-05-01T10:00:00Z",
                "first_login_time": "2023-01-01T08:00:00Z",
                "password_hash": "$2y$abcdefghijklmnopqrstuv",
                "api_key": "abcdef0123456789abcdef",
                "contest_title": "ICPC2024 Final",
                "notes": "Bearer abcdefghijklmnopqrstuvwxyz and _csrf_token: abcdefghi",
            }
            (raw / "contest_2" / "sample.json").write_text(
                json.dumps(sample),
                encoding="utf-8",
            )
            (raw / "contest_2" / "submission_files.zip").write_bytes(b"zip bytes")

            count = sanitize_api.sanitize_tree(raw, out)

            self.assertEqual(2, count)
            got = json.loads((out / "contest_2" / "sample.json").read_text(encoding="utf-8"))
            self.assertEqual(
                "https://example.org/api/v4/users?email=user1@example.org",
                got["url"],
            )
            self.assertEqual("http://example.org/path?q=1", got["homepage"])
            self.assertEqual("192.0.2.1", got["remote_ip"])
            self.assertEqual("192.0.2.1", got["ip"])
            self.assertEqual("192.0.2.1", got["last_ip"])
            self.assertEqual("2001:db8::1", got["ipv6"])
            self.assertEqual("1:00:05", got["contest_time"])
            self.assertEqual("user1@example.org", got["email"])
            self.assertEqual("Sample001", got["name"])
            self.assertEqual("course001", got["course"])
            self.assertEqual("student001", got["student"])
            self.assertEqual("TEST_CSRF_TOKEN", got["_csrf_token"])
            self.assertEqual("+1-555-0100", got["phone"])
            self.assertEqual("Sample University", got["affiliation"])
            self.assertEqual("2026-01-01T00:00:00+00:00", got["last_login_time"])
            self.assertEqual("2026-01-01T00:00:00+00:00", got["first_login_time"])
            self.assertEqual("REDACTED", got["password_hash"])
            self.assertEqual("REDACTED", got["api_key"])
            self.assertEqual("ContestX", got["contest_title"])
            self.assertEqual("Bearer REDACTED and _csrf_token: TEST_CSRF_TOKEN", got["notes"])
            self.assertEqual(
                b"zip bytes",
                (out / "contest_2" / "submission_files.zip").read_bytes(),
            )


if __name__ == "__main__":
    unittest.main()
