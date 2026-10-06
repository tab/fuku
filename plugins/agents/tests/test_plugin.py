"""Structure check of the fuku agents plugin and the proof that scripts/api.sh keeps the token out of sight"""

import http.server
import json
import os
import pathlib
import re
import shutil
import socket
import subprocess
import tempfile
import threading
import unittest

PLUGIN = pathlib.Path(__file__).resolve().parents[1]
ROOT = PLUGIN.parents[1]
SKILL = PLUGIN / "skills" / "fuku"
SCRIPT = SKILL / "scripts" / "api.sh"

CLAUDE_MANIFEST = PLUGIN / ".claude-plugin" / "plugin.json"
CODEX_MANIFEST = PLUGIN / ".codex-plugin" / "plugin.json"
CLAUDE_MARKETPLACE = ROOT / ".claude-plugin" / "marketplace.json"
CODEX_MARKETPLACE = ROOT / ".agents" / "plugins" / "marketplace.json"

LINK = re.compile(r"\]\(([^)\s]+)\)")
GUARD_WORDS = [
    "control.py",
    "discover",
    "revision",
    "FUKU_API_TOKEN",
    "runtime.api",
    "config.api",
    "--since",
    "--summary",
]

BASE = """services:
  api:
    token: fake-decoy-service
auth:
  token: fake-decoy-top
server:
  listen: "127.0.0.1:9876"
  other:
    token: fake-decoy-other
  auth:
    # token: fake-decoy-comment
    token: fake-base # trailing comment
"""


def frontmatter(text):
    """Returns the top-level keys of a Markdown frontmatter, folded block values joined into one line"""
    if not text.startswith("---\n") or "\n---\n" not in text[4:]:
        return {}

    fields = {}
    key = None
    for line in text[4:].split("\n---\n", 1)[0].splitlines():
        match = re.match(r"^([A-Za-z_][\w-]*):\s*(.*)$", line)
        if match:
            key, value = match.groups()
            fields[key] = "" if value in (">", ">-", "|", "|-") else value.strip()
        elif key and line.startswith(" "):
            fields[key] = (fields[key] + " " + line.strip()).strip()

    return fields


def broken_links(markdown):
    """Returns the relative links of a Markdown file whose target does not exist"""
    targets = (link.split("#", 1)[0] for link in LINK.findall(markdown.read_text(encoding="utf-8")))

    return [
        target
        for target in targets
        if target and not re.match(r"^[a-z]+:", target) and not (markdown.parent / target).exists()
    ]


class ManifestTests(unittest.TestCase):
    def test_json_files_parse(self):
        for path in [CLAUDE_MANIFEST, CODEX_MANIFEST, CLAUDE_MARKETPLACE, CODEX_MARKETPLACE]:
            with self.subTest(path=str(path.relative_to(ROOT))):
                self.assertIsInstance(json.loads(path.read_text(encoding="utf-8")), dict)

    def test_plugin_manifests_share_identity(self):
        claude = json.loads(CLAUDE_MANIFEST.read_text(encoding="utf-8"))
        codex = json.loads(CODEX_MANIFEST.read_text(encoding="utf-8"))

        self.assertEqual("fuku", claude["name"])
        self.assertEqual("fuku", codex["name"])
        self.assertEqual(claude["version"], codex["version"])

    def test_marketplace_entries_point_to_plugin(self):
        claude = json.loads(CLAUDE_MARKETPLACE.read_text(encoding="utf-8"))["plugins"]
        codex = json.loads(CODEX_MARKETPLACE.read_text(encoding="utf-8"))["plugins"]
        entries = [
            ("claude", claude[0]["name"], claude[0]["source"], len(claude)),
            ("codex", codex[0]["name"], codex[0]["source"]["path"], len(codex)),
        ]

        for host, name, source, count in entries:
            with self.subTest(host=host):
                self.assertEqual(1, count)
                self.assertEqual("fuku", name)
                self.assertEqual(PLUGIN, (ROOT / source).resolve())

    def test_codex_asset_paths_exist(self):
        codex = json.loads(CODEX_MANIFEST.read_text(encoding="utf-8"))
        values = [codex["skills"]] + list(codex["interface"].values())
        paths = [value for value in values if isinstance(value, str) and value.startswith("./")]

        self.assertGreater(len(paths), 0)
        for path in paths:
            with self.subTest(path=path):
                self.assertTrue((PLUGIN / path).exists())


class SkillTests(unittest.TestCase):
    def test_one_skill_ships(self):
        skills = sorted((ROOT / "plugins").rglob("SKILL.md"))

        self.assertEqual([SKILL / "SKILL.md"], skills)

    def test_frontmatter_names_the_skill(self):
        fields = frontmatter((SKILL / "SKILL.md").read_text(encoding="utf-8"))

        self.assertEqual("fuku", fields.get("name"))
        self.assertTrue(fields.get("description"))

    def test_relative_links_resolve(self):
        for markdown in [SKILL / "SKILL.md"] + sorted((SKILL / "references").glob("*.md")):
            with self.subTest(file=markdown.name):
                self.assertEqual([], broken_links(markdown))

    def test_script_is_executable(self):
        self.assertTrue(SCRIPT.is_file())
        self.assertTrue(os.access(SCRIPT, os.X_OK))

    def test_guard_words_absent(self):
        files = [path for path in SKILL.rglob("*") if path.is_file()]

        for path in files:
            text = path.read_text(encoding="utf-8")
            for word in GUARD_WORDS:
                with self.subTest(file=str(path.relative_to(SKILL)), word=word):
                    self.assertNotIn(word, text)

    def test_log_reads_never_follow(self):
        lines = [line for line in (SKILL / "SKILL.md").read_text(encoding="utf-8").splitlines() if "fuku logs" in line]

        self.assertGreater(len(lines), 0)
        for line in lines:
            with self.subTest(line=line):
                self.assertIn("--no-follow", line)
                self.assertIn("--no-ui", line)


class StructureCheckTests(unittest.TestCase):
    def test_frontmatter_rejects_missing_fields(self):
        cases = [
            ("no frontmatter", "# fuku\n"),
            ("no name", "---\ndescription: Runs fuku\n---\n"),
            ("empty description", "---\nname: fuku\ndescription: >-\n---\n"),
        ]

        for name, text in cases:
            with self.subTest(case=name):
                fields = frontmatter(text)
                self.assertFalse(fields.get("name") == "fuku" and fields.get("description"))

    def test_broken_link_is_found(self):
        with tempfile.TemporaryDirectory() as directory:
            markdown = pathlib.Path(directory) / "SKILL.md"
            markdown.write_text("[ok](SKILL.md) [web](https://getfuku.sh/) [gone](references/missing.md)\n")

            self.assertEqual(["references/missing.md"], broken_links(markdown))


class Listener(http.server.BaseHTTPRequestHandler):
    requests = []

    def answer(self):
        Listener.requests.append((self.command, self.path, self.headers.get("Authorization")))
        body = b'{"ok":true}'
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    do_GET = answer
    do_POST = answer

    def log_message(self, *args):
        pass


class IPv6Server(http.server.ThreadingHTTPServer):
    address_family = socket.AF_INET6


class ScriptTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Listener)
        cls.url = "http://127.0.0.1:%d" % cls.server.server_address[1]
        threading.Thread(target=cls.server.serve_forever, daemon=True).start()

    @classmethod
    def tearDownClass(cls):
        cls.server.shutdown()
        cls.server.server_close()

    def setUp(self):
        Listener.requests.clear()

    def run_script(self, files, args, env=None):
        with tempfile.TemporaryDirectory() as directory:
            for name, content in files.items():
                (pathlib.Path(directory) / name).write_text(content)

            return subprocess.run(
                [str(SCRIPT)] + [arg.format(url=self.url) for arg in args],
                cwd=directory,
                env=dict(os.environ, **{key: value.format(dir=directory) for key, value in (env or {}).items()}),
                capture_output=True,
                text=True,
                timeout=30,
            )

    def test_sends_the_effective_token(self):
        cases = [
            ("base only with decoys", {"fuku.yaml": BASE}, [], "Bearer fake-base"),
            (
                "override wins",
                {"fuku.yaml": BASE, "fuku.override.yml": "server:\n  auth:\n    token: 'fake-over''ride'\n"},
                [],
                "Bearer fake-over'ride",
            ),
            (
                "override without token keeps base",
                {"fuku.yaml": BASE, "fuku.override.yaml": 'server:\n  listen: "127.0.0.1:9900"\n'},
                [],
                "Bearer fake-base",
            ),
            (
                "double quoted",
                {"fuku.yaml": 'server:\n    auth:\n        token: "fake-dq:with#hash" # c\n'},
                [],
                "Bearer fake-dq:with#hash",
            ),
            ("yml base", {"fuku.yml": "server:\n  auth:\n    token: fake-yml\n"}, [], "Bearer fake-yml"),
            (
                "config reads one file",
                {
                    "custom.yaml": "server:\n  auth:\n    token: fake-explicit\n",
                    "fuku.yaml": BASE,
                    "fuku.override.yaml": "server:\n  auth:\n    token: fake-ignored\n",
                },
                ["--config", "custom.yaml"],
                "Bearer fake-explicit",
            ),
            (
                "config path with a backslash",
                {"a\\tb.yaml": "server:\n  auth:\n    token: fake-backslash\n"},
                ["--config", "a\\tb.yaml"],
                "Bearer fake-backslash",
            ),
            (
                "single quote in a comment",
                {"fuku.yaml": "server:\n  auth:\n    token: 'fake-sq' # don't\n"},
                [],
                "Bearer fake-sq",
            ),
            (
                "double quote in a comment",
                {"fuku.yaml": 'server:\n  auth:\n    token: "fake-dq" # say "hi"\n'},
                [],
                "Bearer fake-dq",
            ),
            (
                "crlf base with a double-quoted token",
                {"fuku.yaml": 'server:\r\n  auth:\r\n    token: "fake-crlf"\r\n'},
                [],
                "Bearer fake-crlf",
            ),
            (
                "double-quoted server key in the override",
                {"fuku.yaml": BASE, "fuku.override.yaml": '"server":\n  auth:\n    token: fake-override\n'},
                [],
                "Bearer fake-override",
            ),
            (
                "single-quoted auth key in the override",
                {"fuku.yaml": BASE, "fuku.override.yaml": "server:\n  'auth':\n    token: fake-override\n"},
                [],
                "Bearer fake-override",
            ),
            (
                "double-quoted token key in the override",
                {"fuku.yaml": BASE, "fuku.override.yaml": 'server:\n  auth:\n    "token": fake-override\n'},
                [],
                "Bearer fake-override",
            ),
            (
                "quoted sibling keeps its own token",
                {"fuku.yaml": 'server:\n  auth:\n    token: fake-real\n  "other":\n    token: fake-other\n'},
                [],
                "Bearer fake-real",
            ),
            (
                "leading document marker",
                {"fuku.yaml": "---\n" + BASE},
                [],
                "Bearer fake-base",
            ),
            (
                "list item under another top-level key",
                {"fuku.yaml": BASE + "hooks:\n- token: fake-list\n"},
                [],
                "Bearer fake-base",
            ),
            (
                "top-level list of mappings with server keys",
                {"fuku.yaml": "hooks:\n- server:\n    auth:\n      token: fake-list\n" + BASE},
                [],
                "Bearer fake-base",
            ),
            (
                "nested list of mappings with server keys",
                {"fuku.yaml": "services:\n  api:\n    items:\n    - server:\n        auth:\n          token: fake-list\n" + BASE},
                [],
                "Bearer fake-base",
            ),
            (
                "anchor on auth keeps the child token",
                {"fuku.yaml": "server:\n  auth: &a\n    token: fake-anchor\n"},
                [],
                "Bearer fake-anchor",
            ),
        ]

        for name, files, flags, header in cases:
            with self.subTest(case=name):
                Listener.requests.clear()

                result = self.run_script(files, flags + ["POST", "{url}/api/v1/services/x/restart"])

                self.assertEqual(0, result.returncode, result.stderr)
                self.assertEqual([("POST", "/api/v1/services/x/restart", header)], Listener.requests)
                self.assertTrue(result.stdout.endswith("\n200\n"))
                self.assertNotIn("fake-", result.stdout + result.stderr)

    def test_sends_nothing_without_a_readable_token(self):
        cases = [
            ("override nulls token", {"fuku.yaml": BASE, "fuku.override.yaml": "server:\n  auth:\n    token: null\n"}),
            ("override empties token", {"fuku.yaml": BASE, "fuku.override.yaml": "server:\n  auth:\n    token: # unset\n"}),
            ("override nulls auth", {"fuku.yaml": BASE, "fuku.override.yaml": "server:\n  auth:\n  listen: x\n"}),
            ("override nulls server", {"fuku.yaml": BASE, "fuku.override.yaml": "server: ~\n"}),
            ("no config", {"fuku.override.yaml": "server:\n  auth:\n    token: fake-orphan\n"}),
            ("no token", {"fuku.yaml": 'server:\n  listen: "127.0.0.1:9876"\n'}),
            ("block scalar", {"fuku.yaml": "server:\n  auth:\n    token: |\n      fake-block\n"}),
            ("escaped double quote", {"fuku.yaml": 'server:\n  auth:\n    token: "fake\\"esc"\n'}),
            ("crlf override nulls token", {"fuku.yaml": BASE, "fuku.override.yaml": "server:\r\n  auth:\r\n    token: ~\r\n"}),
        ]

        for name, files in cases:
            with self.subTest(case=name):
                Listener.requests.clear()

                result = self.run_script(files, ["GET", "{url}/api/v1/status"])

                self.assertEqual(1, result.returncode)
                self.assertEqual([], Listener.requests)
                self.assertTrue(result.stderr.startswith("api.sh: "))
                self.assertNotIn("fake-", result.stdout + result.stderr)

    def test_refuses_unreadable_forms(self):
        cases = [
            ("flow mapping on auth", {"fuku.yaml": "server:\n  auth: {token: fake-flow}\n"}),
            ("flow mapping on server", {"fuku.yaml": "server: {auth: {token: fake-flow}}\n"}),
            ("alias on auth", {"fuku.yaml": "server:\n  auth: *a\n"}),
            ("text after a single-quoted token", {"fuku.yaml": "server:\n  auth:\n    token: 'fake-sq' extra\n"}),
            ("text after a double-quoted token", {"fuku.yaml": 'server:\n  auth:\n    token: "fake-dq" extra\n'}),
            ("multi-line plain token", {"fuku.yaml": "server:\n  auth:\n    token: fake-first\n      second\n"}),
            ("list item beside auth", {"fuku.yaml": "server:\n  auth:\n    token: fake-real\n  - token: fake-list\n"}),
            ("stray line in server", {"fuku.yaml": "server:\n  auth:\n    token: fake-real\n  stray\n"}),
            ("escaped quoted key", {"fuku.yaml": '"ser\\x76er":\n  auth:\n    token: fake-escaped\n'}),
            ("complex key", {"fuku.yaml": "? server\n: auth:\n    token: fake-complex\n"}),
            ("merge key in auth", {"fuku.yaml": "x: &a\n  token: fake-merge\nserver:\n  auth:\n    <<: *a\n"}),
            ("indented root", {"fuku.yaml": "  server:\n    auth:\n      token: fake-indented\n"}),
            ("indented override root", {"fuku.yaml": BASE, "fuku.override.yaml": "  server:\n    auth:\n      token: fake-indented\n"}),
            ("second document", {"fuku.yaml": BASE + "---\nserver:\n  auth:\n    token: fake-second\n"}),
        ]

        for name, files in cases:
            with self.subTest(case=name):
                Listener.requests.clear()

                result = self.run_script(files, ["GET", "{url}/api/v1/status"])

                self.assertEqual(1, result.returncode)
                self.assertEqual([], Listener.requests)
                self.assertIn("cannot read", result.stderr)
                self.assertNotIn("fake-", result.stdout + result.stderr)

    def test_ignores_curlrc(self):
        cases = [
            ("verbose", "verbose\n"),
            ("trace to stdout", "trace-ascii = -\n"),
        ]

        for name, curlrc in cases:
            with self.subTest(case=name):
                Listener.requests.clear()

                result = self.run_script(
                    {"fuku.yaml": BASE, ".curlrc": curlrc},
                    ["GET", "{url}/api/v1/status"],
                    {"HOME": "{dir}", "CURL_HOME": "{dir}"},
                )

                self.assertEqual(0, result.returncode, result.stderr)
                self.assertEqual(1, len(Listener.requests))
                self.assertNotIn("fake-", result.stdout + result.stderr)

    def test_sends_one_request_for_a_glob_url(self):
        cases = [
            ("braces", "{url}/api/v1/services/{{a,b}}"),
            ("brackets", "{url}/api/v1/services/[1-2]"),
        ]

        for name, url in cases:
            with self.subTest(case=name):
                Listener.requests.clear()

                result = self.run_script({"fuku.yaml": BASE}, ["GET", url])

                self.assertEqual(0, result.returncode, result.stderr)
                self.assertEqual(1, len(Listener.requests))

    def test_reaches_ipv6_loopback(self):
        try:
            server = IPv6Server(("::1", 0), Listener)
        except OSError as error:
            self.skipTest("no IPv6 loopback on this machine: %s" % error)
        self.addCleanup(server.server_close)
        self.addCleanup(server.shutdown)
        threading.Thread(target=server.serve_forever, daemon=True).start()
        url = "http://[::1]:%d/api/v1/status" % server.server_address[1]

        result = self.run_script({"fuku.yaml": BASE}, ["GET", url])

        self.assertEqual(0, result.returncode, result.stderr)
        self.assertEqual([("GET", "/api/v1/status", "Bearer fake-base")], Listener.requests)

    def test_token_stays_off_curl_argv_and_environment(self):
        curl = shutil.which("curl")

        with tempfile.TemporaryDirectory() as directory:
            folder = pathlib.Path(directory)
            (folder / "fuku.yaml").write_text(BASE)
            (folder / "bin").mkdir()
            shim = folder / "bin" / "curl"
            shim.write_text(
                '#!/bin/sh\nprintf "%%s\\n" "$@" > "%s/argv.txt"\nenv > "%s/env.txt"\nexec "%s" "$@"\n'
                % (directory, directory, curl)
            )
            shim.chmod(0o755)

            result = subprocess.run(
                [str(SCRIPT), "GET", self.url + "/api/v1/status"],
                cwd=directory,
                env={"PATH": "%s:%s" % (folder / "bin", os.environ["PATH"])},
                capture_output=True,
                text=True,
                timeout=30,
            )
            argv = (folder / "argv.txt").read_text()
            environment = (folder / "env.txt").read_text()

        self.assertEqual(0, result.returncode, result.stderr)
        self.assertIn(self.url + "/api/v1/status", argv)
        self.assertNotIn("fake-", argv)
        self.assertFalse("fake-" in environment, "the token reached curl's environment")
        self.assertEqual([("GET", "/api/v1/status", "Bearer fake-base")], Listener.requests)

    def test_refuses_bad_input(self):
        cases = [
            ("method DELETE", ["DELETE", "{url}/api/v1/status"]),
            ("lowercase method", ["get", "{url}/api/v1/status"]),
            ("remote host", ["GET", "http://example.invalid:9876/api/v1/status"]),
            ("https", ["GET", "https://127.0.0.1:9876/api/v1/status"]),
            ("user info", ["GET", "http://127.0.0.1:1@127.0.0.2:9876/api/v1/status"]),
            ("look-alike host", ["GET", "http://localhost.example.invalid:9876/api/v1/status"]),
            ("no path", ["GET", "{url}"]),
            ("missing url", ["GET"]),
        ]

        for name, args in cases:
            with self.subTest(case=name):
                Listener.requests.clear()

                result = self.run_script({"fuku.yaml": BASE}, args)

                self.assertEqual(2, result.returncode)
                self.assertEqual([], Listener.requests)
                self.assertNotIn("fake-", result.stdout + result.stderr)


if __name__ == "__main__":
    unittest.main()
