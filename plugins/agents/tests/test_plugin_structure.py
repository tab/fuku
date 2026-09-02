from __future__ import annotations

import importlib.util
import json
import os
import pathlib
import re
import subprocess
import sys
import tempfile
import unittest


PLUGIN_ROOT = pathlib.Path(__file__).parents[1]
REPOSITORY_ROOT = PLUGIN_ROOT.parents[1]
HOOK_PATH = PLUGIN_ROOT / "hooks" / "instances.py"
HOOK_SPEC = importlib.util.spec_from_file_location("instances", HOOK_PATH)
assert HOOK_SPEC is not None and HOOK_SPEC.loader is not None
instances = importlib.util.module_from_spec(HOOK_SPEC)
HOOK_SPEC.loader.exec_module(instances)


class PluginStructureTests(unittest.TestCase):
    def test_codex_manifest(self):
        manifest_path = PLUGIN_ROOT / ".codex-plugin" / "plugin.json"
        manifest = json.loads(manifest_path.read_text(encoding="utf-8"))

        self.assertEqual("fuku", manifest["name"])
        self.assertEqual("./skills/", manifest["skills"])
        self.assertTrue((PLUGIN_ROOT / "skills" / "fuku" / "SKILL.md").is_file())

    def test_codex_marketplace_points_to_plugin(self):
        marketplace_path = REPOSITORY_ROOT / ".agents" / "plugins" / "marketplace.json"
        marketplace = json.loads(marketplace_path.read_text(encoding="utf-8"))
        entry = next(item for item in marketplace["plugins"] if item["name"] == "fuku")

        self.assertEqual("fuku", marketplace["name"])
        self.assertEqual("fuku", marketplace["interface"]["displayName"])
        self.assertEqual("./plugins/agents", entry["source"]["path"])
        self.assertEqual("AVAILABLE", entry["policy"]["installation"])

    def test_claude_marketplace_points_to_plugin(self):
        marketplace_path = REPOSITORY_ROOT / ".claude-plugin" / "marketplace.json"
        marketplace = json.loads(marketplace_path.read_text(encoding="utf-8"))
        entry = next(item for item in marketplace["plugins"] if item["name"] == "fuku")

        self.assertEqual("fuku", marketplace["name"])
        self.assertEqual("./plugins/agents", entry["source"])

    def test_claude_manifest_matches_codex_identity(self):
        codex_manifest = json.loads(
            (PLUGIN_ROOT / ".codex-plugin" / "plugin.json").read_text(encoding="utf-8")
        )
        claude_manifest = json.loads(
            (PLUGIN_ROOT / ".claude-plugin" / "plugin.json").read_text(encoding="utf-8")
        )

        self.assertEqual("fuku", claude_manifest["name"])
        self.assertEqual(codex_manifest["name"], claude_manifest["name"])
        self.assertEqual(codex_manifest["version"], claude_manifest["version"])
        self.assertEqual(codex_manifest["homepage"], claude_manifest["homepage"])

    def test_manifest_homepage_points_at_a_published_page(self):
        manifest = json.loads(
            (PLUGIN_ROOT / ".claude-plugin" / "plugin.json").read_text(encoding="utf-8")
        )
        page = REPOSITORY_ROOT / "docs" / "src" / "pages" / "plugins" / "agents.astro"

        self.assertEqual("https://getfuku.sh/plugins/agents/", manifest["homepage"])
        self.assertTrue(page.is_file())

    def test_skill_has_required_frontmatter(self):
        skill = (PLUGIN_ROOT / "skills" / "fuku" / "SKILL.md").read_text(encoding="utf-8")

        self.assertTrue(skill.startswith("---\n"))
        self.assertIn("\nname: fuku\n", skill)
        self.assertIn("\ndescription:", skill)
        self.assertIn("Never print a whole fuku config file", skill)
        self.assertIn("Never extract `server.auth.token`", skill)
        self.assertNotIn("[TODO:", skill)

    def test_skill_reference_links_exist(self):
        skill_path = PLUGIN_ROOT / "skills" / "fuku" / "SKILL.md"
        skill = skill_path.read_text(encoding="utf-8")
        references = re.findall(r"\]\((references/[^)]+)\)", skill)

        self.assertGreaterEqual(len(references), 2)
        for reference in references:
            with self.subTest(reference=reference):
                self.assertTrue((skill_path.parent / reference).is_file())

    def test_packaged_openapi_matches_canonical_contract(self):
        canonical = REPOSITORY_ROOT / "spec" / "openapi.yaml"
        packaged = PLUGIN_ROOT / "skills" / "fuku" / "references" / "openapi.yaml"

        self.assertEqual(
            canonical.read_bytes(),
            packaged.read_bytes(),
            "Run make generate:agents-plugin after changing spec/openapi.yaml",
        )

    def test_control_reference_links_to_packaged_openapi(self):
        reference = (
            PLUGIN_ROOT / "skills" / "fuku" / "references" / "control-api.md"
        ).read_text(encoding="utf-8")

        self.assertIn("](openapi.yaml)", reference)

    def test_claude_commands_are_packaged(self):
        names = sorted(path.stem for path in (PLUGIN_ROOT / "commands").glob("*.md"))

        self.assertEqual(["logs", "restart", "status"], names)

    def test_claude_commands_declare_a_description(self):
        for path in sorted((PLUGIN_ROOT / "commands").glob("*.md")):
            with self.subTest(command=path.name):
                text = path.read_text(encoding="utf-8")

                self.assertTrue(text.startswith("---\n"))
                self.assertIn("\ndescription:", text)

    def test_session_start_hook_is_registered(self):
        hooks = json.loads((PLUGIN_ROOT / "hooks" / "hooks.json").read_text(encoding="utf-8"))
        matchers = [entry["matcher"] for entry in hooks["hooks"]["SessionStart"]]
        commands = [
            hook["command"]
            for entry in hooks["hooks"]["SessionStart"]
            for hook in entry["hooks"]
        ]

        self.assertEqual(["startup|resume|clear|compact|fork"], matchers)
        self.assertTrue(any("hooks/instances.py" in command for command in commands))
        self.assertTrue((PLUGIN_ROOT / "hooks" / "instances.py").is_file())

    def test_session_start_hook_is_silent_outside_a_project(self):
        with tempfile.TemporaryDirectory(dir="/tmp") as directory:
            result = subprocess.run(
                [sys.executable, str(PLUGIN_ROOT / "hooks" / "instances.py")],
                cwd=directory,
                env={**os.environ, "CLAUDE_PROJECT_DIR": directory},
                capture_output=True,
                text=True,
                timeout=30,
                check=False,
            )

        self.assertEqual(0, result.returncode)
        self.assertEqual("", result.stdout.strip())

    def test_session_start_context_uses_the_authenticated_project_instance(self):
        class Control:
            @staticmethod
            def discover(explicit, timeout):
                return {
                    "socket_profiles": ["minimal"],
                    "api": {"reachable": True, "base_url": "http://localhost:1235"},
                    "instance": {
                        "status": {"phase": "running", "profile": "minimal"},
                        "services": [{"name": "api", "status": "running"}],
                    },
                }

        context = instances.describe(Control)

        self.assertIn("This project's fuku API is running", context)
        self.assertIn("profile 'minimal'", context)
        self.assertIn("api (running)", context)

    def test_session_start_context_does_not_claim_machine_wide_sockets(self):
        class Control:
            @staticmethod
            def discover(explicit, timeout):
                return {
                    "socket_profiles": ["minimal"],
                    "other_socket_profiles": ["another-project"],
                    "api": {"reachable": False, "reason": "server.listen is not set"},
                }

        context = instances.describe(Control)

        self.assertIn("No running fuku API was confirmed for this project", context)
        self.assertIn("This project has live log sockets for profiles: minimal", context)
        self.assertNotIn("another-project", context)
        self.assertNotIn("already running", context)

    def test_codex_manifest_does_not_claim_the_claude_components(self):
        manifest = json.loads(
            (PLUGIN_ROOT / ".codex-plugin" / "plugin.json").read_text(encoding="utf-8")
        )

        self.assertEqual("./skills/", manifest["skills"])
        self.assertNotIn("commands", manifest)
        self.assertNotIn("hooks", manifest)

    def test_skill_requires_attaching_before_starting(self):
        skill = (PLUGIN_ROOT / "skills" / "fuku" / "SKILL.md").read_text(encoding="utf-8")

        self.assertIn("## Attach before starting", skill)
        self.assertIn("control.py discover", skill)
        self.assertIn("refuses to start beside an instance already serving this project", skill)
        self.assertIn("other_socket_profiles", skill)
        self.assertRegex(
            skill,
            r"identifies an instance before it authenticates, so this project's token is never offered to\s+an API belonging to another directory",
        )

    def test_skill_reads_logs_without_a_follower(self):
        skill = (PLUGIN_ROOT / "skills" / "fuku" / "SKILL.md").read_text(encoding="utf-8")
        command = (PLUGIN_ROOT / "commands" / "logs.md").read_text(encoding="utf-8")

        self.assertIn("control.py logs --tail", skill)
        self.assertIn("Never start a log follower to collect evidence", skill)
        self.assertIn("control.py logs --tail", command)
        self.assertIn("Never stream logs with a follower process", command)

    def test_packaged_contract_documents_the_log_endpoints(self):
        spec = (PLUGIN_ROOT / "skills" / "fuku" / "references" / "openapi.yaml").read_text(encoding="utf-8")

        self.assertIn("/logs:", spec)
        self.assertIn("/services/{id}/logs:", spec)
        self.assertIn("operationId: listLogs", spec)
        self.assertIn("operationId: listServiceLogs", spec)

    def test_packaged_contract_documents_the_instance_identity(self):
        spec = (PLUGIN_ROOT / "skills" / "fuku" / "references" / "openapi.yaml").read_text(encoding="utf-8")

        self.assertIn("    Live:", spec)
        self.assertIn('$ref: "#/components/schemas/Live"', spec)
        self.assertRegex(spec, r"required: \[status, product, instance, project\]")
        self.assertRegex(spec, r"required: \[version, instance, project, profile, phase, uptime, services\]")

    def test_product_name_is_lowercase_in_plugin_text(self):
        roots = [
            REPOSITORY_ROOT / ".agents" / "plugins",
            REPOSITORY_ROOT / ".claude-plugin",
            PLUGIN_ROOT,
        ]
        suffixes = {".json", ".md", ".py", ".yaml", ".yml"}

        for root in roots:
            for path in root.rglob("*"):
                if path.is_file() and path.suffix in suffixes:
                    with self.subTest(path=path):
                        product_name = "Fu" + "ku"
                        self.assertNotRegex(path.read_text(encoding="utf-8"), rf"\b{product_name}\b")


if __name__ == "__main__":
    unittest.main()
