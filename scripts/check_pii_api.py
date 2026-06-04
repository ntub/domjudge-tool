#!/usr/bin/env python3
"""Check sanitized DOMjudge API fixtures for PII-shaped leaks."""

from __future__ import annotations

import argparse
import ipaddress
import re
import sys
import zipfile
from dataclasses import dataclass
from pathlib import Path
from typing import Callable, Iterable


FIXTURE_SUFFIXES = {".json", ".txt", ".tsv", ".csv", ".zip", ".html", ".headers"}
MAX_EXCERPT = 180
SENSITIVE_HOST_TOKENS = ("domjudge7", "jason" "xie", "nt" "ub")
ALLOWED_PUBLIC_HOSTS = {"ccs-specs.icpc.io", "www.domjudge.org", "clics.ecs.baylor.edu"}
HOST_TLDS = (
    "com",
    "net",
    "org",
    "edu",
    "gov",
    "mil",
    "tw",
    "io",
    "dev",
    "local",
    "me",
    "co",
    "app",
    "cloud",
    "ai",
    "info",
    "biz",
    "xyz",
    "eu",
    "uk",
    "jp",
    "cn",
    "kr",
    "us",
    "ca",
    "au",
    "de",
    "fr",
)
CFEMAIL_PATTERN = re.compile(
    r"\bdata-cfemail\s*=\s*(?P<quote>[\"'])(?P<hex>[0-9A-Fa-f]+)(?P=quote)"
)
NON_EXAMPLE_EMAIL_PATTERN = re.compile(
    r"\b[A-Za-z0-9._%+-]+@(?!(?:example\.org)(?![A-Za-z0-9.-]))"
    r"(?:[A-Za-z0-9-]+\.)+[A-Za-z]{2,}(?![A-Za-z0-9.-])"
)


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


def _ipv4_allowed(match: re.Match[str]) -> bool:
    try:
        value = ipaddress.ip_address(match.group(0))
    except ValueError:
        return False
    return value in ipaddress.ip_network("192.0.2.0/24") or value in ipaddress.ip_network(
        "203.0.113.0/24"
    )


def _hostname_allowed(match: re.Match[str]) -> bool:
    host_match = match.group("host")
    if host_match is None:
        return False
    host = host_match.lower().rstrip(".")
    return host == "example.org" or host.endswith(".example.org") or host in ALLOWED_PUBLIC_HOSTS


def cf_decode(value: str) -> str:
    raw = bytes.fromhex(value)
    key = raw[0]
    return "".join(chr(char ^ key) for char in raw[1:])


def cfemail_findings(source: str, line_no: int, line: str) -> list[Finding]:
    findings: list[Finding] = []
    for match in CFEMAIL_PATTERN.finditer(line):
        try:
            decoded = cf_decode(match.group("hex"))
        except ValueError:
            findings.append(Finding(5, "non-example.org email", source, line_no, _excerpt(line)))
            continue
        if NON_EXAMPLE_EMAIL_PATTERN.search(decoded) or not decoded.lower().endswith(
            "@example.org"
        ):
            findings.append(Finding(5, "non-example.org email", source, line_no, _excerpt(line)))
    return findings


def mask_cfemail_values(line: str) -> str:
    return CFEMAIL_PATTERN.sub(
        lambda match: f"data-cfemail={match.group('quote')}REDACTED{match.group('quote')}",
        line,
    )


GATES = (
    Gate(
        1,
        "sensitive or raw hostname",
        re.compile(
            r"\b(?:" + "|".join(re.escape(token) for token in SENSITIVE_HOST_TOKENS) + r")\b"
            r"|(?i:(?:https?://)?(?P<host>(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+"
            r"(?:" + "|".join(HOST_TLDS) + r")|localhost)\b)",
            re.IGNORECASE,
        ),
        _hostname_allowed,
    ),
    Gate(2, "8+ digit student id", re.compile(r"(?<!\d)\d{8,}(?!\d)")),
    Gate(3, "CJK text", re.compile(r"[\u3400-\u4DBF\u4E00-\u9FFF\uF900-\uFAFF]+")),
    Gate(
        4,
        "real IPv4 address",
        re.compile(r"\b(?:\d{1,3}\.){3}\d{1,3}\b"),
        _ipv4_allowed,
    ),
    Gate(
        5,
        "non-example.org email",
        NON_EXAMPLE_EMAIL_PATTERN,
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
            r"(?i)\b(?:_csrf_token|csrf[_-]?token)\b"
            r"(?:[\"']?\s*[:=]\s*[\"']?|[^>\n]{0,200}\bvalue\s*=\s*[\"'])"
            r"(?P<value>[A-Za-z0-9_-]{8,})"
        ),
        _csrf_allowed,
    ),
    Gate(
        9,
        "basic credentials",
        re.compile(r"(?i)\bbasic\s+[A-Za-z0-9+/]{8,}={0,2}"),
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
        findings.extend(cfemail_findings(source, line_no, line))
        scan_line = mask_cfemail_values(line)
        for gate in GATES:
            for match in gate.pattern.finditer(scan_line):
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
