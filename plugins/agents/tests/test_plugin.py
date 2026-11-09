"""Structure check of the fuku agents plugin"""

import json
import pathlib
import re
import tempfile
import unittest

PLUGIN = pathlib.Path(__file__).resolve().parents[1]
ROOT = PLUGIN.parents[1]
SKILL = PLUGIN / "skills" / "fuku"
VERSION = "0.2.0"

CLAUDE_MANIFEST = PLUGIN / ".claude-plugin" / "plugin.json"
CODEX_MANIFEST = PLUGIN / ".codex-plugin" / "plugin.json"
MCP_CONFIG = PLUGIN / ".mcp.json"
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
    "api.sh",
    "control-api",
    "fuku run ",
    "fuku stop ",
    "fuku logs ",
    "fuku doctor ",
]
TOOLS = ["doctor", "run", "stop", "service"]


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
        for path in [CLAUDE_MANIFEST, CODEX_MANIFEST, MCP_CONFIG, CLAUDE_MARKETPLACE, CODEX_MARKETPLACE]:
            with self.subTest(path=str(path.relative_to(ROOT))):
                self.assertIsInstance(json.loads(path.read_text(encoding="utf-8")), dict)

    def test_plugin_manifests_share_identity(self):
        claude = json.loads(CLAUDE_MANIFEST.read_text(encoding="utf-8"))
        codex = json.loads(CODEX_MANIFEST.read_text(encoding="utf-8"))

        self.assertEqual("fuku", claude["name"])
        self.assertEqual("fuku", codex["name"])
        self.assertEqual(VERSION, claude["version"])
        self.assertEqual(VERSION, codex["version"])

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
        values = [codex["skills"], codex["mcpServers"]] + list(codex["interface"].values())
        paths = [value for value in values if isinstance(value, str) and value.startswith("./")]

        self.assertGreater(len(paths), 0)
        for path in paths:
            with self.subTest(path=path):
                self.assertTrue((PLUGIN / path).exists())

    def test_mcp_config_launches_fuku(self):
        servers = json.loads(MCP_CONFIG.read_text(encoding="utf-8"))["mcpServers"]

        self.assertEqual({"fuku": {"command": "fuku", "args": ["mcp"]}}, servers)

    def test_codex_manifest_bundles_the_mcp_config(self):
        codex = json.loads(CODEX_MANIFEST.read_text(encoding="utf-8"))

        self.assertEqual("./.mcp.json", codex["mcpServers"])


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

    def test_skill_ships_only_markdown(self):
        files = sorted(str(path.relative_to(SKILL)) for path in SKILL.rglob("*") if path.is_file())

        self.assertEqual(["SKILL.md", "references/configuration.md"], files)

    def test_guard_words_absent(self):
        files = [path for path in SKILL.rglob("*") if path.is_file()]

        for path in files:
            text = path.read_text(encoding="utf-8")
            for word in GUARD_WORDS:
                with self.subTest(file=str(path.relative_to(SKILL)), word=word):
                    self.assertNotIn(word, text)

    def test_skill_names_the_tools(self):
        text = (SKILL / "SKILL.md").read_text(encoding="utf-8")

        for tool in TOOLS:
            with self.subTest(tool=tool):
                self.assertIn("`%s`" % tool, text)

    def test_skill_stays_on_mcp_tools(self):
        text = (SKILL / "SKILL.md").read_text(encoding="utf-8")

        self.assertIn("only through the fuku MCP tools", text)
        self.assertIn("Never run `fuku`, `curl` or the REST API from the shell", text)


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


if __name__ == "__main__":
    unittest.main()
