#!/usr/bin/env python3
"""Sanitize raw DOMjudge API fixtures before committing them."""

from __future__ import annotations

import argparse
import ipaddress
import json
import re
import shutil
import sys
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any
from urllib.parse import urlsplit, urlunsplit


STABLE_LOGIN_TIME = "2026-01-01T00:00:00+00:00"
TEST_IPV4 = "192.0.2.1"
TEST_IPV6 = "2001:db8::1"
TEST_CSRF_TOKEN = "TEST_CSRF_TOKEN"
REDACTED = "REDACTED"
SAMPLE_UNIVERSITY = "Sample University"
CONTEST_TITLE = "ContestX"
SAMPLE_PHONE = "+1-555-0100"
SENSITIVE_DOMAIN_TOKENS = ("domjudge7", "jason" "xie", "nt" "ub")

EMAIL_RE = re.compile(
    r"\b[A-Za-z0-9._%+-]+@(?:[A-Za-z0-9-]+\.)+[A-Za-z]{2,}(?![A-Za-z0-9.-])"
)
URL_RE = re.compile(r"https?://[^\s\"'<>]+")
SENSITIVE_DOMAIN_RE = re.compile(
    r"\b(?:[A-Za-z0-9-]+\.)*(?:"
    + "|".join(re.escape(token) for token in SENSITIVE_DOMAIN_TOKENS)
    + r")"
    r"(?:\.[A-Za-z0-9-]+)+\b",
    re.IGNORECASE,
)
IPV4_RE = re.compile(
    r"\b(?:(?:25[0-5]|2[0-4]\d|1?\d?\d)\.){3}"
    r"(?:25[0-5]|2[0-4]\d|1?\d?\d)\b"
)
IPV6_CANDIDATE_RE = re.compile(
    r"(?<![\w:])(?:[0-9A-Fa-f]{0,4}:){2,}[0-9A-Fa-f]{0,4}(?![\w:])"
)
COURSE_RE = re.compile(
    r"(?<!\d)\d{7}\u56db\u6280"
    r"(?:\u8cc7\u7ba1|\u884c\u92b7|\u4f01\u61c9|\u6578\u96fb|\u96fb\u8cc7)+"
    r"\u4e00[\u7532\u4e59\u4e19]"
)
STUDENT_ID_RE = re.compile(r"(?<!\d)\d{8}(?!\d)")
CJK_RE = re.compile(r"[\u3400-\u4DBF\u4E00-\u9FFF\uF900-\uFAFF]+")
CSRF_INLINE_RE = re.compile(
    r"(?i)(\b_csrf_token\b[\"']?\s*[:=]\s*[\"']?)([A-Za-z0-9_-]{8,})"
)
BEARER_RE = re.compile(r"(?i)\bbearer\s+[A-Za-z0-9_-]{20,}")
PHONE_RE = re.compile(r"(?<!\w)(?:\+?\d[\d .()/-]{7,}\d)(?!\w)")
CONTEST_RE = re.compile(
    r"\b(?:ICPC\d{4}|\d+OpenCV\d+|OpenCV\d+|Contest\s*\d{2,4})\b",
    re.IGNORECASE,
)

REAL_AFFILIATIONS = (
    "NT" "UB",
    "nt" "ub.edu.tw",
    "National Taipei University of Business",
    "\u570b\u7acb\u53f0\u5317\u5546\u696d\u5927\u5b78",
    "\u570b\u7acb\u81fa\u5317\u5546\u696d\u5927\u5b78",
)

LOGIN_TIME_KEYS = {"last_login_time", "first_login_time"}
IP_KEYS = {"ip", "last_ip"}
CSRF_KEYS = {"_csrf_token", "csrf_token"}
SECRET_KEY_PARTS = (
    "api_key",
    "apikey",
    "bearer",
    "password",
    "secret",
    "session",
    "token",
)
AFFILIATION_KEYS = {
    "affiliation",
    "affiliation_name",
    "organization",
    "organization_name",
    "formal_name",
}
CONTEST_KEYS = {"contest", "contest_name", "contest_title", "title", "shortname"}


@dataclass
class PlaceholderState:
    email: dict[str, str] = field(default_factory=dict)
    cjk: dict[str, str] = field(default_factory=dict)
    course: dict[str, str] = field(default_factory=dict)
    student: dict[str, str] = field(default_factory=dict)

    def placeholder(self, seen: dict[str, str], raw: str, prefix: str, width: int = 0) -> str:
        if raw not in seen:
            n = len(seen) + 1
            suffix = f"{n:0{width}d}" if width else str(n)
            seen[raw] = f"{prefix}{suffix}"
        return seen[raw]


class Sanitizer:
    def __init__(self) -> None:
        self.placeholders = PlaceholderState()

    def sanitize_data(self, value: Any, key: str | None = None) -> Any:
        if isinstance(value, dict):
            return {k: self.sanitize_data(v, k) for k, v in value.items()}
        if isinstance(value, list):
            return [self.sanitize_data(v, key) for v in value]
        if isinstance(value, str):
            return self.sanitize_string_value(value, key)
        return value

    def sanitize_string_value(self, value: str, key: str | None = None) -> str:
        normalized_key = (key or "").lower()

        if normalized_key in LOGIN_TIME_KEYS:
            return STABLE_LOGIN_TIME
        if normalized_key in IP_KEYS:
            return TEST_IPV4 if value else value
        if normalized_key in CSRF_KEYS or "csrf" in normalized_key:
            return TEST_CSRF_TOKEN if value else value
        if any(part in normalized_key for part in SECRET_KEY_PARTS):
            return REDACTED if value else value
        if normalized_key in AFFILIATION_KEYS and self._contains_real_affiliation(value):
            return SAMPLE_UNIVERSITY
        if normalized_key in CONTEST_KEYS and CONTEST_RE.search(value):
            return CONTEST_TITLE

        return self.sanitize_string(value)

    def sanitize_string(self, value: str) -> str:
        value = URL_RE.sub(self._sanitize_url, value)
        value = EMAIL_RE.sub(self._replace_email, value)
        value = SENSITIVE_DOMAIN_RE.sub("example.org", value)
        value = self._replace_affiliation_text(value)
        value = COURSE_RE.sub(self._replace_course, value)
        value = STUDENT_ID_RE.sub(self._replace_student, value)
        value = CSRF_INLINE_RE.sub(lambda match: f"{match.group(1)}{TEST_CSRF_TOKEN}", value)
        value = BEARER_RE.sub("Bearer REDACTED", value)
        value = CONTEST_RE.sub(CONTEST_TITLE, value)
        value = IPV6_CANDIDATE_RE.sub(self._replace_ipv6, value)
        value = IPV4_RE.sub(TEST_IPV4, value)
        value = PHONE_RE.sub(self._replace_phone, value)
        value = CJK_RE.sub(self._replace_cjk, value)
        return value

    def _sanitize_url(self, match: re.Match[str]) -> str:
        raw = match.group(0)
        parsed = urlsplit(raw)
        if not parsed.netloc:
            return raw
        return urlunsplit((parsed.scheme, "example.org", parsed.path, parsed.query, parsed.fragment))

    def _replace_email(self, match: re.Match[str]) -> str:
        raw = match.group(0)
        if raw not in self.placeholders.email:
            self.placeholders.email[raw] = f"user{len(self.placeholders.email) + 1}@example.org"
        return self.placeholders.email[raw]

    def _replace_course(self, match: re.Match[str]) -> str:
        return self.placeholders.placeholder(self.placeholders.course, match.group(0), "course", 3)

    def _replace_student(self, match: re.Match[str]) -> str:
        return self.placeholders.placeholder(self.placeholders.student, match.group(0), "student", 3)

    def _replace_cjk(self, match: re.Match[str]) -> str:
        return self.placeholders.placeholder(self.placeholders.cjk, match.group(0), "Sample", 3)

    def _replace_ipv6(self, match: re.Match[str]) -> str:
        raw = match.group(0)
        try:
            parsed = ipaddress.ip_address(raw)
        except ValueError:
            return raw
        if parsed.version != 6:
            return raw
        return TEST_IPV6

    def _replace_phone(self, match: re.Match[str]) -> str:
        raw = match.group(0)
        if IPV4_RE.fullmatch(raw):
            return raw
        return SAMPLE_PHONE

    def _contains_real_affiliation(self, value: str) -> bool:
        lowered = value.lower()
        return any(item.lower() in lowered for item in REAL_AFFILIATIONS)

    def _replace_affiliation_text(self, value: str) -> str:
        for item in REAL_AFFILIATIONS:
            value = re.sub(re.escape(item), SAMPLE_UNIVERSITY, value, flags=re.IGNORECASE)
        return value


def sanitize_json_file(src: Path, dst: Path, sanitizer: Sanitizer) -> None:
    with src.open("r", encoding="utf-8") as fh:
        data = json.load(fh)
    sanitized = sanitizer.sanitize_data(data)
    dst.parent.mkdir(parents=True, exist_ok=True)
    with dst.open("w", encoding="utf-8") as fh:
        json.dump(sanitized, fh, ensure_ascii=False, indent=2)
        fh.write("\n")


def copy_fixture_file(src: Path, dst: Path) -> None:
    dst.parent.mkdir(parents=True, exist_ok=True)
    shutil.copyfile(src, dst)


def sanitize_tree(raw_dir: Path, out_dir: Path) -> int:
    if not raw_dir.exists():
        raise FileNotFoundError(f"raw fixture directory does not exist: {raw_dir}")
    if not raw_dir.is_dir():
        raise NotADirectoryError(f"raw fixture path is not a directory: {raw_dir}")

    sanitizer = Sanitizer()
    written = 0
    for src in sorted(path for path in raw_dir.rglob("*") if path.is_file()):
        rel = src.relative_to(raw_dir)
        dst = out_dir / rel
        if src.suffix.lower() == ".json":
            sanitize_json_file(src, dst, sanitizer)
        else:
            copy_fixture_file(src, dst)
        written += 1
    return written


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Sanitize raw DOMjudge API fixtures into a commit-safe fixture tree.",
    )
    parser.add_argument("raw_dir", type=Path, help="Directory containing raw API fixtures")
    parser.add_argument("out_dir", type=Path, help="Directory for sanitized fixtures")
    return parser.parse_args(argv)


def main(argv: list[str] | None = None) -> int:
    args = parse_args(sys.argv[1:] if argv is None else argv)
    try:
        count = sanitize_tree(args.raw_dir, args.out_dir)
    except (OSError, json.JSONDecodeError) as exc:
        print(f"sanitize_api.py: {exc}", file=sys.stderr)
        return 1
    print(f"sanitized {count} fixture file(s) into {args.out_dir}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
