#!/usr/bin/env python3
"""Behavioral tests for the documentation producer; no external AI or browser."""

from __future__ import annotations

import contextlib
import io
import json
import os
import socket
import struct
import subprocess
import sys
import tempfile
import textwrap
import unittest
import zlib
from pathlib import Path
from unittest.mock import patch

import update_user_guide as guide


FAKE_PROVIDER = r'''
import json
import pathlib
import sys
import time

config_path = pathlib.Path(__file__).with_suffix(".json")
config = json.loads(config_path.read_text())
args = sys.argv[1:]
required = ["--print", "--safe-mode", "--restricted", "--strict-mcp-config",
            "--disable-slash-commands", "--no-session-persistence"]
assert all(flag in args for flag in required), args
for flag, value in {
    "--tools": "Read,Glob,Grep", "--allowedTools": "Read,Glob,Grep",
    "--permission-mode": "dontAsk", "--permission-prompts": "none",
    "--output-format": "json", "--mcp-config": '{"mcpServers":{}}',
}.items():
    assert args[args.index(flag) + 1] == value, (flag, args)
assert not any("bypass" in arg.lower() or "skip-permissions" in arg for arg in args)
schema = json.loads(args[args.index("--json-schema") + 1])
assert schema["properties"]["updates"]["items"]["properties"]["path"]["enum"]
root = pathlib.Path.cwd()
assert not (root / ".git").exists()
for name, expected in config.get("expected", {}).items():
    assert (root / name).read_text() == expected, name
for name in config.get("absent", []):
    assert not (root / name).exists(), name
prompt = sys.stdin.read()
assert "untrusted data" in prompt and "Do not run Git" in prompt
pathlib.Path(config["record"]).write_text(json.dumps({
    "cwd": str(root), "args": args,
    "changes": json.loads((root / "REVIEW_CHANGES.json").read_text()),
}))
mode = config.get("mode", "success")
if mode == "timeout":
    time.sleep(30)
if mode == "error":
    print("authentication unavailable", file=sys.stderr)
    sys.exit(7)
if mode == "malformed":
    print("not JSON")
    sys.exit(0)
response = {
    "type": "result", "subtype": "success", "is_error": False,
    "permission_denials": [],
    "structured_output": config.get("proposal", {"summary": "Current.", "updates": []}),
}
response.update(config.get("envelope", {}))
print(json.dumps(response))
'''


class RepositoryTest(unittest.TestCase):
    """Exercise snapshots, provider subprocesses, and writes using disposable Git."""

    def setUp(self) -> None:
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.base = Path(self.temp.name)
        self.root = self.base / "repo"
        self.root.mkdir()
        self.git_env = os.environ.copy()
        self.git_env.update({
            "GIT_CONFIG_NOSYSTEM": "1", "GIT_CONFIG_GLOBAL": os.devnull,
            "GIT_AUTHOR_NAME": "Example", "GIT_COMMITTER_NAME": "Example",
            "GIT_AUTHOR_EMAIL": "author@example.org", "GIT_COMMITTER_EMAIL": "author@example.org",
        })
        self.git("init", "--quiet", "--template=")
        self.put(".gitignore", "tmp/\n.env\ninternal/ignored.go\n")
        self.put("internal/feature.go", "package feature\n// original\n")
        self.put("README.md", "# Example\n\n[Guide](docs/USER_GUIDE.md).\n")
        self.put("CONTRIBUTING.md", "# Contributing\n")
        commands = "```bash\ndomjudge-tool --help\n```\n"
        images = "\n".join(f"![Preview](images/user-guide/{slug}.png)" for slug, _ in guide.SCREENS)
        self.put("docs/USER_GUIDE.md", "# User guide\n\n## Use\n\n" + commands + "\n" + images + "\n")
        self.put("docs/USER_GUIDE.zh-TW.md", "# 使用手冊\n\n## 使用\n\n" + commands + "\n" + images + "\n")
        self.put("docs/CLI.md", "# CLI\n\n## `help`\n\n" + commands)
        self.put("docs/CLI.zh-TW.md", "# 指令\n\n## `help`\n\n" + commands)
        for slug, _ in guide.SCREENS:
            self.put(f"docs/images/user-guide/{slug}.png", png())
        self.git("add", ".")
        self.git("-c", "core.hooksPath=/dev/null", "commit", "--quiet", "-m", "initial")
        self.bin = self.base / "bin"
        self.bin.mkdir()
        self.provider = self.executable("claude", FAKE_PROVIDER)
        self.record = self.base / "provider-record.json"
        self.configure_provider()
        self.scanner_record = self.base / "scanner-record.json"
        self.executable("trufflehog", textwrap.dedent(f"""\
            import json
            import pathlib
            import sys
            args = sys.argv[1:]
            assert '--no-verification' in args and '--fail' in args
            assert '--fail-on-scan-errors' in args
            root = pathlib.Path(args[args.index('filesystem') + 1])
            assert not (root / '.git').exists()
            pathlib.Path({str(self.scanner_record)!r}).write_text(json.dumps(args))
        """))
        self.environment = patch.dict(os.environ, {"PATH": str(self.bin) + os.pathsep + os.environ["PATH"]})
        self.environment.start()
        self.addCleanup(self.environment.stop)

    def put(self, name: str, content: str | bytes) -> Path:
        path = self.root / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(content.encode() if isinstance(content, str) else content)
        return path

    def git(self, *args: str) -> bytes:
        return subprocess.run(
            ["git", "-C", str(self.root), *args], env=self.git_env,
            check=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
        ).stdout

    def executable(self, name: str, content: str) -> Path:
        path = self.bin / name
        path.write_text(f"#!{sys.executable}\n" + content, encoding="utf-8")
        path.chmod(0o755)
        return path

    def configure_provider(self, **values: object) -> None:
        config: dict[str, object] = {"record": str(self.record)}
        config.update(values)
        self.provider.with_suffix(".json").write_text(json.dumps(config), encoding="utf-8")

    def invoke(self, command: str = "hook", *args: str) -> tuple[int, str]:
        output = io.StringIO()
        with contextlib.redirect_stdout(output), contextlib.redirect_stderr(output):
            status = guide.main([
                command, "--root", str(self.root), "--agent", str(self.provider),
                "--timeout", "5", *args,
            ])
        return status, output.getvalue()

    def stage_feature(self, name: str = "internal/feature.go") -> None:
        self.put(name, "package feature\n// staged feature\n")
        self.git("add", "--", name)

    def test_docs_only_skips_provider_and_privacy_dependency(self) -> None:
        self.put("README.md", "# Example\n\nDocumentation only.\n")
        self.git("add", "README.md")
        with patch.object(guide, "find_executable", side_effect=AssertionError("must not find tools")):
            status, output = self.invoke()
        self.assertEqual(0, status, output)
        self.assertIn("No staged user-facing changes", output)
        self.assertFalse(self.record.exists())

    def test_index_snapshot_excludes_unstaged_ignored_and_sensitive_data(self) -> None:
        self.stage_feature()
        self.put("internal/feature.go", "package feature\n// unstaged private content\n")
        self.put("internal/untracked.go", "package private\n")
        self.put("internal/ignored.go", "package ignored\n")
        self.put("tmp/private.md", "not for the provider\n")
        self.put(".claude/settings.json", '{"hooks": {}}\n')
        self.put("internal/credentials.json", '{"password":"REDACTED"}\n')
        self.git("add", "-f", "internal/ignored.go", ".claude/settings.json", "internal/credentials.json")
        self.configure_provider(
            expected={"internal/feature.go": "package feature\n// staged feature\n"},
            absent=["internal/untracked.go", "internal/ignored.go", "tmp", ".claude", "internal/credentials.json"],
        )
        before = self.git("ls-files", "--stage", "-z")
        raw_index = (self.root / ".git/index").read_bytes()
        status, output = self.invoke()
        self.assertEqual(0, status, output)
        self.assertEqual(before, self.git("ls-files", "--stage", "-z"))
        self.assertEqual(raw_index, (self.root / ".git/index").read_bytes())
        self.assertIn("unstaged private content", (self.root / "internal/feature.go").read_text())
        self.assertTrue(self.scanner_record.exists())
        self.assertFalse(Path(json.loads(self.record.read_text())["cwd"]).exists())
        manifest = json.loads(self.record.read_text())["changes"]
        self.assertNotIn("package ignored", manifest["diff"])
        self.assertNotIn("internal/ignored.go", manifest["changed_paths"])

    def test_local_and_global_ignore_rules_exclude_source_and_diff(self) -> None:
        self.stage_feature()
        global_config = self.base / "gitconfig"
        global_excludes = self.base / "global-ignore"
        global_excludes.write_text("config/global-site.json\n")
        global_config.write_text(f"[core]\n\texcludesFile = {global_excludes}\n")
        self.put(".git/info/exclude", "config/local-site.json\n")
        for name in ("config/local-site.json", "config/global-site.json"):
            self.put(name, '{"privateExample":"never disclose this local setting"}\n')
            self.git("add", "-f", "--", name)
        self.configure_provider(absent=["config/local-site.json", "config/global-site.json"])
        before = self.git("ls-files", "--stage", "-z")
        with patch.dict(os.environ, {"GIT_CONFIG_GLOBAL": str(global_config)}):
            status, output = self.invoke()
        self.assertEqual(0, status, output)
        manifest = json.loads(self.record.read_text())["changes"]
        self.assertNotIn("never disclose this local setting", manifest["diff"])
        self.assertNotIn("config/local-site.json", manifest["changed_paths"])
        self.assertNotIn("config/global-site.json", manifest["changed_paths"])
        self.assertEqual(before, self.git("ls-files", "--stage", "-z"))

    def test_update_reads_current_tracked_source_not_the_index(self) -> None:
        self.stage_feature()
        self.put("internal/feature.go", "package feature\n// current tracked source\n")
        self.configure_provider(expected={"internal/feature.go": "package feature\n// current tracked source\n"})
        before = self.git("ls-files", "--stage", "-z")
        status, output = self.invoke("update")
        self.assertEqual(0, status, output)
        self.assertEqual(before, self.git("ls-files", "--stage", "-z"))

    def test_deleted_feature_triggers_review(self) -> None:
        self.git("rm", "internal/feature.go")
        status, output = self.invoke()
        self.assertEqual(0, status, output)
        changes = json.loads(self.record.read_text())["changes"]
        self.assertIn("internal/feature.go", changes["changed_paths"])
        self.assertIn("-// original", changes["diff"])

    def test_renamed_feature_and_newline_filename_are_nul_safe(self) -> None:
        name = "docs/renamed\nfeature.md"
        self.git("mv", "internal/feature.go", name)
        self.stage_feature("internal/new\ncommand.go")
        status, output = self.invoke()
        self.assertEqual(0, status, output)
        paths = json.loads(self.record.read_text())["changes"]["changed_paths"]
        self.assertIn("internal/feature.go", paths)
        self.assertIn(name, paths)
        self.assertIn("internal/new\ncommand.go", paths)

    def test_valid_proposal_stops_hook_without_staging(self) -> None:
        self.stage_feature()
        proposed = "# Example\n\nUpdated usage.\n"
        self.configure_provider(proposal={"summary": "Update usage.", "updates": [{"path": "README.md", "content": proposed}]})
        before = self.git("ls-files", "--stage", "-z")
        status, output = self.invoke()
        self.assertEqual(1, status, output)
        self.assertEqual(proposed, (self.root / "README.md").read_text())
        self.assertEqual(before, self.git("ls-files", "--stage", "-z"))
        self.assertIn("inspect", output.lower())
        self.assertIn("stage", output.lower())

    def test_invalid_proposals_never_partially_write(self) -> None:
        self.stage_feature()
        original = (self.root / "README.md").read_bytes()
        bad_updates = [
            {"path": "internal/feature.go", "content": "# Not documentation\n"},
            {"path": "../README.md", "content": "# Traversal\n"},
            {"path": "docs/../README.md", "content": "# Traversal\n"},
            {"path": "/README.md", "content": "# Absolute\n"},
            {"path": "docs/CLI.md", "content": ""},
            {"path": "docs/CLI.md", "content": "# Broken\n\n```bash\nunclosed\n"},
            {"path": "docs/CLI.md", "content": "# Broken\n\n[Missing](absent.md)\n"},
        ]
        for bad in bad_updates:
            with self.subTest(proposal=bad):
                self.configure_provider(proposal={"summary": "Invalid.", "updates": [
                    {"path": "README.md", "content": "# Updated\n"}, bad,
                ]})
                status, output = self.invoke()
                self.assertNotEqual(0, status, output)
                self.assertEqual(original, (self.root / "README.md").read_bytes())

    def test_invalid_schema_and_provider_envelopes_fail_closed(self) -> None:
        self.stage_feature()
        cases = [
            {"proposal": {"summary": "Wrong type.", "updates": {}}},
            {"proposal": {"summary": "Extra.", "updates": [], "extra": True}},
            {"envelope": {"permission_denials": [{"tool_name": "Read"}]}},
            {"envelope": {"is_error": True}},
            {"envelope": {"subtype": "error_max_budget_usd"}},
            {"mode": "malformed"},
        ]
        for case in cases:
            with self.subTest(case=case):
                self.configure_provider(**case)
                status, output = self.invoke()
                self.assertNotEqual(0, status, output)

    def test_preexisting_document_edits_are_preserved(self) -> None:
        self.stage_feature()
        self.put("README.md", "# Example\n\nMy local unfinished notes.\n")
        self.configure_provider(proposal={"summary": "Change.", "updates": [
            {"path": "README.md", "content": "# Example\n\nAI proposal.\n"},
        ]})
        before = self.git("ls-files", "--stage", "-z")
        status, output = self.invoke()
        self.assertNotEqual(0, status, output)
        self.assertIn("local", output.lower())
        self.assertIn("My local unfinished notes", (self.root / "README.md").read_text())
        self.assertEqual(before, self.git("ls-files", "--stage", "-z"))

    def test_symlink_source_and_document_targets_are_rejected(self) -> None:
        self.stage_feature()
        outside = self.base / "outside.md"
        outside.write_text("# Private\n")
        link = self.root / "internal/link.go"
        link.symlink_to(outside)
        self.git("add", "internal/link.go")
        status, output = self.invoke()
        self.assertNotEqual(0, status, output)
        self.assertIn("symlink", output.lower())
        self.assertFalse(self.record.exists())
        self.git("rm", "--force", "internal/link.go")
        (self.root / "README.md").unlink()
        (self.root / "README.md").symlink_to(outside)
        status, output = self.invoke()
        self.assertNotEqual(0, status, output)
        self.assertEqual("# Private\n", outside.read_text())

    def test_provider_failure_and_timeout_surface_without_writes(self) -> None:
        self.stage_feature()
        original = (self.root / "README.md").read_bytes()
        for mode, expected in [("error", "authentication unavailable"), ("timeout", "timed out")]:
            with self.subTest(mode=mode):
                self.configure_provider(mode=mode)
                status, output = self.invoke("hook", "--timeout", "0.2" if mode == "timeout" else "5")
                self.assertNotEqual(0, status, output)
                self.assertIn(expected, output)
                self.assertEqual(original, (self.root / "README.md").read_bytes())

    def test_missing_provider_and_privacy_failures_do_not_pass_feature_commit(self) -> None:
        self.stage_feature()
        status, output = self.invoke("hook", "--agent", str(self.base / "absent-claude"))
        self.assertNotEqual(0, status, output)
        self.executable("trufflehog", "import sys\nprint('privacy scanner rejected source', file=sys.stderr)\nsys.exit(183)\n")
        status, output = self.invoke()
        self.assertNotEqual(0, status, output)
        self.assertIn("privacy scanner rejected source", output)
        self.assertFalse(self.record.exists())

    def test_missing_privacy_scanner_stops_before_provider(self) -> None:
        self.stage_feature()
        original = guide.find_executable

        def find(name: str) -> str:
            if name == "trufflehog":
                raise guide.GuideError("Required executable not found: trufflehog")
            return original(name)

        with patch.object(guide, "find_executable", side_effect=find):
            status, output = self.invoke()
        self.assertNotEqual(0, status, output)
        self.assertIn("trufflehog", output)
        self.assertFalse(self.record.exists())

    def test_shadow_ignore_repository_cannot_mutate_an_alternate_index(self) -> None:
        self.stage_feature()
        alternate = self.base / "alternate-index"
        alternate.write_bytes((self.root / ".git/index").read_bytes())
        before = alternate.read_bytes()
        with patch.dict(os.environ, {"GIT_INDEX_FILE": str(alternate)}):
            status, output = self.invoke()
        self.assertEqual(0, status, output)
        self.assertEqual(before, alternate.read_bytes())

    def test_literal_pathspecs_do_not_disclose_ignored_neighbor_changes(self) -> None:
        self.stage_feature("internal/[draft]*.go")
        self.put("internal/ignored.go", "package ignored\n// private ignored change\n")
        self.git("add", "-f", "internal/ignored.go")
        status, output = self.invoke()
        self.assertEqual(0, status, output)
        manifest = json.loads(self.record.read_text())["changes"]
        self.assertIn("internal/[draft]*.go", manifest["changed_paths"])
        self.assertNotIn("private ignored change", manifest["diff"])

    def test_check_validates_paired_structure_images_and_links(self) -> None:
        guide.check_documents(self.root)
        bad_cases = [
            ("docs/USER_GUIDE.zh-TW.md", "# Guide\n\n## One\n\n### Extra\n"),
            ("README.md", "# Example\n\n```\nmissing language\n```\n"),
            ("README.md", "# Example\n\n[Broken](missing.md)\n"),
            ("README.md", "# Example\n\n[Broken][absent]\n"),
            ("README.md", "# Example\n\n[Broken](docs/USER_GUIDE.md#missing-anchor)\n"),
            ("README.md", ""),
            ("docs/USER_GUIDE.md", "# Guide\n\n```\nmissing language\n```\n"),
            ("docs/USER_GUIDE.md", "# Guide\n\n[Broken](missing.md)\n"),
            ("docs/USER_GUIDE.md", "# Guide\n\n[Broken][absent]\n"),
            ("docs/USER_GUIDE.md", "# Guide\n\n[Broken](../README.md#missing-anchor)\n"),
            ("docs/USER_GUIDE.md", ""),
        ]
        for name, text in bad_cases:
            with self.subTest(name=name, text=text):
                before = (self.root / name).read_bytes()
                self.put(name, text)
                with self.assertRaises(guide.GuideError):
                    guide.check_documents(self.root)
                self.put(name, before)
        (self.root / "docs/images/user-guide/profiles.png").write_bytes(b"")
        with self.assertRaises(guide.GuideError):
            guide.check_documents(self.root)

    def test_check_resolves_real_reference_links_line_fragments_and_code_literals(self) -> None:
        self.put("README.md", (
            "# Example\n\n[Heading](docs/USER_GUIDE.md#use)\n"
            "[Source](internal/feature.go#L1-L2)\n[Guide][manual]\n"
            "`[literal](absent.md)`\n\n[manual]: docs/USER_GUIDE.md\n"
        ))
        guide.check_documents(self.root)


class CaptureTest(unittest.TestCase):
    """Own a real fixture process while substituting only compiler/browser binaries."""

    def setUp(self) -> None:
        with socket.socket() as probe:
            probe.bind(("127.0.0.1", 0))
            self.address = probe.getsockname()
        self.address_patch = patch.object(guide, "PREVIEW_ADDRESS", self.address)
        self.address_patch.start()
        self.addCleanup(self.address_patch.stop)

    def test_owned_server_and_browser_profile_cleanup_on_browser_failure(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "tmp").mkdir()
            binary = root / "server"
            binary.write_text(f"#!{sys.executable}\n" + textwrap.dedent(f'''\
                import http.server
                import os
                import pathlib
                class Handler(http.server.BaseHTTPRequestHandler):
                    def do_GET(self):
                        self.send_response(200)
                        self.end_headers()
                        self.wfile.write(b'<!DOCTYPE html><title>Servers</title>')
                    def log_message(self, *args):
                        pass
                pathlib.Path('server.pid').write_text(str(os.getpid()))
                http.server.HTTPServer({self.address!r}, Handler).serve_forever()
            '''))
            binary.chmod(0o755)
            browser = root / "browser"
            record = root / "browser-profile.txt"
            browser.write_text(f"#!{sys.executable}\n" + textwrap.dedent(f'''\
                import pathlib
                import sys
                for arg in sys.argv[1:]:
                    if arg.startswith('--user-data-dir='):
                        pathlib.Path({str(record)!r}).write_text(arg.split('=', 1)[1])
                print('browser failed deliberately', file=sys.stderr)
                sys.exit(3)
            '''))
            browser.chmod(0o755)
            processes: list[subprocess.Popen[bytes]] = []
            original_popen = subprocess.Popen

            def record_popen(*args: object, **kwargs: object) -> subprocess.Popen[bytes]:
                process = original_popen(*args, **kwargs)
                processes.append(process)
                return process

            with patch.object(guide, "build_preview", return_value=binary), patch.object(
                guide, "require_ignored_tmp", return_value=None,
            ), patch.object(guide.subprocess, "Popen", side_effect=record_popen):
                with self.assertRaisesRegex(guide.GuideError, "browser failed deliberately"):
                    guide.capture(root, browser=str(browser))
            self.assertTrue(processes)
            self.assertTrue(all(process.poll() is not None for process in processes))
            self.assertTrue(record.exists())
            self.assertFalse(Path(record.read_text()).exists())
            self.assertEqual([], list((root / "tmp").iterdir()))
            self.assertFalse((root / "docs/images/user-guide/profiles.png").exists())

    def test_occupied_capture_port_is_not_reused_or_terminated(self) -> None:
        with socket.socket() as listener:
            listener.bind(self.address)
            listener.listen()
            with tempfile.TemporaryDirectory() as tmp:
                with patch.object(guide, "build_preview", side_effect=AssertionError("must not build")):
                    with self.assertRaisesRegex(guide.GuideError, str(self.address[1])):
                        guide.capture(Path(tmp))
            self.assertGreaterEqual(listener.fileno(), 0)


def png() -> bytes:
    """Create a small valid, correctly sized PNG without a third-party image package."""
    def chunk(kind: bytes, data: bytes) -> bytes:
        return struct.pack(">I", len(data)) + kind + data + struct.pack(">I", zlib.crc32(kind + data))

    rows = (b"\x00" + b"\x00\x00\x00" * 1280) * 1000
    return (
        b"\x89PNG\r\n\x1a\n"
        + chunk(b"IHDR", struct.pack(">IIBBBBB", 1280, 1000, 8, 2, 0, 0, 0))
        + chunk(b"IDAT", zlib.compress(rows))
        + chunk(b"IEND", b"")
    )


if __name__ == "__main__":
    unittest.main()
