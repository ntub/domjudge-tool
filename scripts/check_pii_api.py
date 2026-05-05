#!/usr/bin/env python3
"""Check sanitized DOMjudge API fixtures for PII-shaped leaks."""

from __future__ import annotations

import argparse
import re
import sys
import zipfile
from dataclasses import dataclass
from pathlib import Path
from typing import Callable, Iterable


FIXTURE_SUFFIXES = {".json", ".txt", ".tsv", ".csv", ".zip"}
MAX_EXCERPT = 180
SENSITIVE_HOST_TOKENS = ("domjudge7", "jason" "xie", "nt" "ub")


@dataclass(frozen=True)
class Finding:
    gate_id: int
    gate_name: str
    source: str
    line: int
    excerpt: str


@dataclass(frozen=True)
class Gate:
    gate_id: int
    name: str
    pattern: re.Pattern[str]
    allowed: Callable[[re.Match[str]], bool] | None = None


def _password_allowed(match: re.Match[str]) -> bool:
    value = match.group("value").strip().strip("\"'")
    return value == "REDACTED" or value.startswith("TEST_")


def _csrf_allowed(match: re.Match[str]) -> bool:
    value = match.group("value").strip().strip("\"'")
    return value == "TEST_CSRF_TOKEN"


GATES = (
    Gate(
        1,
        "sensitive hostname or organization token",
        re.compile(
            r"\b(?:" + "|".join(re.escape(token) for token in SENSITIVE_HOST_TOKENS) + r")\b",
            re.IGNORECASE,
        ),
    ),
    Gate(2, "8-digit student id", re.compile(r"(?<!\d)\d{8}(?!\d)")),
    Gate(3, "CJK text", re.compile(r"[\u3400-\u4DBF\u4E00-\u9FFF\uF900-\uFAFF]+")),
    Gate(
        4,
        "private IPv4 address",
        re.compile(r"\b(?:10|172\.(?:1[6-9]|2\d|3[0-1])|192\.168)\.\d{1,3}\.\d{1,3}\b"),
    ),
    Gate(
        5,
        "non-example.org email",
        re.compile(
            r"\b[A-Za-z0-9._%+-]+@(?!(?:example\.org)(?![A-Za-z0-9.-]))"
            r"(?:[A-Za-z0-9-]+\.)+[A-Za-z]{2,}(?![A-Za-z0-9.-])"
        ),
    ),
    Gate(
        6,
        "bearer token",
        re.compile(r"(?i)\bbearer\s+[A-Za-z0-9_-]{20,}"),
    ),
    Gate(
        7,
        "password-like secret",
        re.compile(
            r"(?i)\bpassword\b[\"']?\s*[:=]\s*[\"']?(?P<value>[^\"'\s,}\]]{6,})"
        ),
        _password_allowed,
    ),
    Gate(
        8,
        "csrf token",
        re.compile(
            r"(?i)\b_csrf_token\b[\"']?\s*[:=]\s*[\"']?(?P<value>[A-Za-z0-9_-]{8,})"
        ),
        _csrf_allowed,
    ),
)


def fixture_files(paths: Iterable[Path]) -> Iterable[Path]:
    for path in paths:
        if path.is_dir():
            for child in sorted(path.rglob("*")):
                if child.is_file() and child.suffix.lower() in FIXTURE_SUFFIXES:
                    yield child
        elif path.is_file() and path.suffix.lower() in FIXTURE_SUFFIXES:
            yield path


def _excerpt(line: str) -> str:
    text = line.strip()
    if len(text) <= MAX_EXCERPT:
        return text
    return f"{text[:MAX_EXCERPT]}..."


def scan_text(source: str, text: str) -> list[Finding]:
    findings: list[Finding] = []
    for line_no, line in enumerate(text.splitlines() or [text], start=1):
        for gate in GATES:
            for match in gate.pattern.finditer(line):
                if gate.allowed is not None and gate.allowed(match):
                    continue
                findings.append(
                    Finding(gate.gate_id, gate.name, source, line_no, _excerpt(line))
                )
    return findings


def scan_file(path: Path) -> list[Finding]:
    if path.suffix.lower() != ".zip":
        return scan_text(str(path), path.read_text(encoding="utf-8", errors="replace"))

    findings: list[Finding] = []
    try:
        with zipfile.ZipFile(path) as archive:
            for info in archive.infolist():
                entry_source = f"{path}!{info.filename}"
                findings.extend(scan_text(f"{entry_source} (filename)", info.filename))
                if info.is_dir():
                    continue
                raw = archive.read(info)
                text = raw.decode("utf-8", errors="replace")
                findings.extend(scan_text(entry_source, text))
    except zipfile.BadZipFile:
        raw = path.read_bytes()
        findings.extend(scan_text(str(path), raw.decode("utf-8", errors="replace")))
    return findings


def check_paths(paths: Iterable[Path]) -> list[Finding]:
    findings: list[Finding] = []
    for path in fixture_files(paths):
        findings.extend(scan_file(path))
    return findings


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Run the Phase 1.5 API fixture PII gates.",
    )
    parser.add_argument("paths", nargs="+", type=Path, help="Fixture file or directory paths")
    return parser.parse_args(argv)


def main(argv: list[str] | None = None) -> int:
    args = parse_args(sys.argv[1:] if argv is None else argv)
    findings = check_paths(args.paths)
    if not findings:
        print(f"PII gates: {len(GATES)}/{len(GATES)} pass")
        return 0

    for finding in findings:
        print(
            f"{finding.source}:{finding.line}: gate {finding.gate_id} "
            f"({finding.gate_name}): {finding.excerpt}",
            file=sys.stderr,
        )
    print(f"PII gates: {len(findings)} finding(s)", file=sys.stderr)
    return 1


if __name__ == "__main__":
    raise SystemExit(main())
