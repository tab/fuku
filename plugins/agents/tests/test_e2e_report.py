"""Offline self-checks of the sweep: exit codes, pass rates, the outage policy, medians, outputs, guards and secrets"""

import contextlib
import glob
import importlib.util
import io
import json
import os
import pathlib
import re
import shutil
import sys
import tempfile
import unittest

E2E = pathlib.Path(__file__).resolve().parents[1] / "e2e"

if "e2e_runner" not in sys.modules:
    spec = importlib.util.spec_from_file_location("e2e_runner", E2E / "runner.py")
    sys.modules[spec.name] = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(sys.modules[spec.name])
runner = sys.modules["e2e_runner"]
report = runner.report

SCENARIO = {
    "prompt": "Start the default profile.",
    "profile": "default",
    "setup": [],
    "checks": ["run_detached_once"],
    "budget": {"tool_calls": 5, "input_tokens": 1000, "seconds": 60, "usd": 1},
}


def canned(tool_calls=1, input_tokens=100, seconds=1.0, usd=0.1, output="Running detached · pid 1 · 2 services",
           **fields):
    """Returns a host result that passes the test scenario unless the fields say otherwise"""
    calls = [{"tool": "shell", "command": "fuku run default -d", "output": output, "error": False}]
    totals = {"tool_calls": tool_calls, "turns": 2, "input_tokens": input_tokens, "cached_input_tokens": 0,
              "output_tokens": 10, "usd": usd, "seconds": seconds, "model": "fake-model", "host_version": "fake 1.0"}

    return runner.AgentResult(**{"calls": calls, "totals": totals, "skill_loaded": True, **fields})


class CannedAdapter:
    """Stand-in adapter that returns prepared results in turn and records the secrets and deadline it was given"""

    def __init__(self, results):
        self.results = list(results)
        self.seen = []

    def __enter__(self):
        return self

    def __exit__(self, *_):
        return False

    def run(self, project, prompt, env, budget=None, timeout=None, secrets=()):
        self.seen.append({"timeout": timeout, "secrets": list(secrets)})
        result = self.results.pop(0)
        return result(secrets) if callable(result) else result


def sweep(test, scenarios, environ, adapters):
    """Runs main against temp scenario files and a temp HOME, returning the exit code, stdout, stderr and report"""
    directory = pathlib.Path(tempfile.mkdtemp(prefix="fuku-fake-sweep-"))
    test.addCleanup(shutil.rmtree, directory)
    for scenario_id, body in scenarios.items():
        (directory / f"{scenario_id}.json").write_text(json.dumps(body), encoding="utf-8")
    home = directory / "home"
    home.mkdir()
    stdout, stderr = io.StringIO(), io.StringIO()
    env = {"FUKU_BIN": sys.executable, "HOME": str(home), "PATH": os.environ["PATH"], **environ}

    with contextlib.redirect_stdout(stdout), contextlib.redirect_stderr(stderr):
        code = runner.main(directory, env, adapters)

    match = re.search(r"^report: (.+)$", stdout.getvalue(), re.M)
    path = pathlib.Path(match.group(1)) if match else None
    if path:
        test.addCleanup(shutil.rmtree, path.parent)

    return code, stdout.getvalue(), stderr.getvalue(), path


class ClassifyTest(unittest.TestCase):
    def test_host_errors(self):
        cases = [
            ("unreadable events, then a crash",
             canned(host_error="unreadable events: line 1: not a Codex event; codex exited 2: boom",
                    host_error_kind="crash"), ("failed", "unreadable")),
            ("crash", canned(host_error="codex exited 2: boom", host_error_kind="crash"), ("failed", "crash")),
            ("outage", canned(host_error="API error 529: Overloaded", host_error_kind="api"), ("outage", "api")),
            ("login", canned(host_error="Codex is not logged in", host_error_kind="login"), ("login", "login")),
            ("no host error", canned(), ("passed", None)),
        ]

        for name, result, expected in cases:
            with self.subTest(name):
                record = report.classify("codex", "T1", 1, runner.RunResult({"teardown": None}, result))

                self.assertEqual(expected, (record["status"], record["host_error_kind"]))


class SweepTest(unittest.TestCase):
    def test_usage_errors(self):
        cases = [
            ("unknown host", {"HOST": "gemini"}, "HOST must be one of"),
            ("zero repeats", {"REPEATS": "0"}, "REPEATS must be a whole number"),
            ("word repeats", {"REPEATS": "three"}, "REPEATS must be a whole number"),
            ("unknown scenario", {"SCENARIO": "S9"}, "unknown scenario: S9"),
        ]

        for name, environ, message in cases:
            with self.subTest(name):
                code, _, stderr, path = sweep(self, {"T1": SCENARIO}, environ, {})

                self.assertEqual(2, code)
                self.assertIn(message, stderr)
                self.assertIsNone(path)

    def test_empty_variables_mean_defaults(self):
        options = runner.sweep_options({"HOST": "", "SCENARIO": "", "REPEATS": ""})

        self.assertEqual((["claude", "codex"], [], 3), options)

    def test_pass_rates_and_exit_codes(self):
        cases = [
            ("all pass", [canned(), canned()], 0, "pass", 1.0),
            ("one run fails", [canned(), canned(skill_loaded=False)], 1, "fail", 0.5),
            ("host crash fails", [canned(), canned(host_error="boom", host_error_kind="crash")], 1, "fail", 0.5),
        ]

        for name, results, exit_code, status, rate in cases:
            with self.subTest(name):
                adapters = {"claude": lambda r=results: CannedAdapter(r)}

                code, _, _, path = sweep(self, {"T1": SCENARIO}, {"HOST": "claude", "REPEATS": "2"}, adapters)

                summary = json.loads(path.read_text())["summary"]["claude/T1"]
                self.assertEqual(exit_code, code)
                self.assertEqual((status, rate), (summary["status"], summary["pass_rate"]))

    def test_outage_stops_the_host(self):
        results = [canned(), canned(host_error="API error 429: limit", host_error_kind="api"), canned(), canned()]
        adapter = CannedAdapter(results)

        code, _, stderr, path = sweep(self, {"T1": SCENARIO, "T2": SCENARIO}, {"HOST": "claude", "REPEATS": "2"},
                                      {"claude": lambda: adapter})

        data = json.loads(path.read_text())
        self.assertEqual(1, code)
        self.assertEqual(2, len(adapter.seen))
        self.assertEqual(("pass", 1.0, 1, 1), tuple(data["summary"]["claude/T1"][k] for k in
                                                     ("status", "pass_rate", "completed", "outages")))
        self.assertEqual(("not run", None), (data["summary"]["claude/T2"]["status"],
                                             data["summary"]["claude/T2"]["pass_rate"]))
        self.assertIn("host outage, the remaining runs were skipped", stderr)

    def test_missing_login_stops_the_host(self):
        results = [canned(), canned(host_error="Codex is not logged in", host_error_kind="login"), canned()]
        adapter = CannedAdapter(results)

        code, _, stderr, path = sweep(self, {"T1": SCENARIO, "T2": SCENARIO}, {"HOST": "codex", "REPEATS": "2"},
                                      {"codex": lambda: adapter})

        data = json.loads(path.read_text())
        self.assertEqual(1, code)
        self.assertEqual(2, len(adapter.seen))
        self.assertEqual(("pass", 1.0, 1, 0), tuple(data["summary"]["codex/T1"][k] for k in
                                                     ("status", "pass_rate", "completed", "outages")))
        self.assertEqual("not run", data["summary"]["codex/T2"]["status"])
        self.assertIn("codex is not logged in; run `codex login`", stderr)

    def test_adapter_error_fails_the_sweep(self):
        class Broken:
            def __enter__(self):
                raise runner.codex.AdapterError("the installed skill differs from the working tree: fuku/SKILL.md")

            def __exit__(self, *_):
                return False

        code, _, stderr, path = sweep(self, {"T1": SCENARIO}, {"HOST": "codex", "REPEATS": "1"}, {"codex": Broken})

        self.assertEqual(1, code)
        self.assertIn("runner: codex: the installed skill differs from the working tree: fuku/SKILL.md", stderr)
        self.assertEqual("not run", json.loads(path.read_text())["summary"]["codex/T1"]["status"])

    def test_medians_and_largest_outputs(self):
        results = [canned(tool_calls=1, input_tokens=100, seconds=3.0, usd=0.3, output="a" * 10),
                   canned(tool_calls=3, input_tokens=300, seconds=1.0, usd=None, output="b" * 30),
                   canned(tool_calls=2, input_tokens=900, seconds=2.0, usd=0.1, output="c" * 20)]

        code, _, _, path = sweep(self, {"T1": SCENARIO}, {"HOST": "claude"}, {"claude": lambda: CannedAdapter(results)})

        data = json.loads(path.read_text())
        medians = data["summary"]["claude/T1"]["medians"]
        largest = [(item["bytes"], item["run"]) for item in data["largest_fuku_outputs"]]
        self.assertEqual(0, code)
        self.assertEqual({"tool_calls": 2, "input_tokens": 300, "output_tokens": 10, "seconds": 2.0, "usd": 0.2},
                         medians)
        self.assertEqual([(30, 2), (20, 3), (10, 1)], largest)

    def test_over_budget_versus_timed_out(self):
        cases = [
            ("over budget", canned(tool_calls=9), {"over_budget": True, "timed_out": False}, "budget:tool_calls"),
            ("timed out", canned(timed_out=True), {"over_budget": False, "timed_out": True}, "time_limit"),
        ]

        for name, result, flags, failed in cases:
            with self.subTest(name):
                adapter = CannedAdapter([result])

                code, _, _, path = sweep(self, {"T1": SCENARIO}, {"HOST": "claude", "REPEATS": "1"},
                                         {"claude": lambda: adapter})

                run = json.loads(path.read_text())["summary"]["claude/T1"]["runs"][0]
                self.assertEqual(1, code)
                self.assertEqual(flags, {k: run["flags"][k] for k in flags})
                self.assertEqual([failed], list(run["failed"]))
                self.assertEqual(120, adapter.seen[0]["timeout"])

    def test_report_holds_no_secret(self):
        def leak(secrets):
            result = canned(output=f"token {secrets[0]}")
            result.calls[0]["command"] = f"fuku run default -d # {secrets[0]}"
            result.answer = f"The token is {secrets[0]}"
            return result

        adapter = CannedAdapter([leak])

        code, stdout, _, path = sweep(self, {"T1": SCENARIO}, {"HOST": "claude", "REPEATS": "1"},
                                      {"claude": lambda: adapter})

        token = adapter.seen[0]["secrets"][0]
        written = "".join(p.read_text() for p in path.parent.iterdir())
        failed = json.loads(path.read_text())["summary"]["claude/T1"]["runs"][0]["failed"]
        answer = json.loads((path.parent / "claude-T1-1.json").read_text())["answer"]
        self.assertEqual(1, code)
        self.assertIn("no_token", failed)
        self.assertNotIn(token, written + stdout)
        self.assertIn("<redacted>", written)
        self.assertEqual("The token is <redacted>", answer)

    def test_answer_is_capped(self):
        adapter = CannedAdapter([canned(answer="a" * 4000 + "b")])

        code, _, _, path = sweep(self, {"T1": SCENARIO}, {"HOST": "claude", "REPEATS": "1"},
                                 {"claude": lambda: adapter})

        run = json.loads((path.parent / "claude-T1-1.json").read_text())
        self.assertEqual(0, code)
        self.assertEqual("a" * 4000, run["answer"])
        self.assertNotIn("answer", json.loads(path.read_text())["summary"]["claude/T1"]["runs"][0])

    def test_login_rotation_is_reported(self):
        adapter = CannedAdapter([canned(login_rotated="Codex refreshed its login during the run")])

        code, _, stderr, _ = sweep(self, {"T1": SCENARIO}, {"HOST": "codex", "REPEATS": "1"},
                                   {"codex": lambda: adapter})

        self.assertEqual(0, code)
        self.assertIn("codex T1 #1: Codex refreshed its login during the run", stderr)

    def test_no_temp_dir_is_left(self):
        before = set(glob.glob(os.path.join(tempfile.gettempdir(), "fuku-agents-e2e-*")))

        sweep(self, {"T1": SCENARIO}, {"HOST": "claude", "REPEATS": "1"}, {"claude": lambda: CannedAdapter([canned()])})

        self.assertEqual(before, set(glob.glob(os.path.join(tempfile.gettempdir(), "fuku-agents-e2e-*"))))


class GuardTest(unittest.TestCase):
    def test_guards(self):
        def plant_fuku(home):
            (home / ".codex" / "plugins" / "cache" / "fuku").mkdir(parents=True)

        def change_config(home):
            (home / ".codex" / "config.toml").write_text("model = 'other'\n", encoding="utf-8")

        def add_plugin(home):
            path = home / ".claude" / "plugins" / "installed_plugins.json"
            path.write_text(json.dumps({"version": 2, "plugins": {"a@m": [], "fuku@fuku": []}}), encoding="utf-8")

        def bump_version(home):
            path = home / ".claude" / "plugins" / "installed_plugins.json"
            path.write_text(json.dumps({"version": 2, "plugins": {"a@m": [{"version": "9.9.9"}]}}), encoding="utf-8")

        def edit_cached_file(home):
            path = home / ".claude" / "plugins" / "cache" / "m" / "a" / "1.0" / "SKILL.md"
            path.write_text("tampered\n", encoding="utf-8")

        def cache_fuku(home):
            (home / ".claude" / "plugins" / "cache" / "fuku" / "fuku").mkdir(parents=True)

        def retarget_link(home):
            (home / ".claude" / "plugins" / "cache" / "m" / "b").mkdir()
            (home / ".claude" / "plugins" / "cache" / "m" / "current").unlink()
            (home / ".claude" / "plugins" / "cache" / "m" / "current").symlink_to("b")

        def lock_cached_plugin(home):
            path = home / ".claude" / "plugins" / "cache" / "m" / "a"
            path.chmod(0)
            self.addCleanup(path.chmod, 0o755)

        def lock_cached_file(home):
            path = home / ".claude" / "plugins" / "cache" / "m" / "a" / "1.0" / "SKILL.md"
            path.chmod(0)
            self.addCleanup(path.chmod, 0o644)

        cases = [
            ("unchanged", lambda home: None, []),
            ("planted fuku path", plant_fuku, ["~/.codex/plugins fuku paths: added ['cache/fuku'], removed []"]),
            ("changed config", change_config, ["~/.codex/config.toml changed"]),
            ("new plugin id", add_plugin,
             ["~/.claude/plugins: added [], removed [], changed ['installed_plugins.json']"]),
            ("changed metadata value", bump_version,
             ["~/.claude/plugins: added [], removed [], changed ['installed_plugins.json']"]),
            ("edited cached file", edit_cached_file,
             ["~/.claude/plugins: added [], removed [], changed ['cache/m/a/1.0/SKILL.md']"]),
            ("new cached plugin", cache_fuku,
             ["~/.claude/plugins: added ['cache/fuku', 'cache/fuku/fuku'], removed [], changed []"]),
            ("retargeted link", retarget_link,
             ["~/.claude/plugins: added ['cache/m/b'], removed [], changed ['cache/m/current']"]),
            ("unreadable dir", lock_cached_plugin,
             ["~/.claude/plugins: added [], removed ['cache/m/a/1.0', 'cache/m/a/1.0/SKILL.md'], "
              "changed ['cache/m/a']"]),
            ("locked cached file", lock_cached_file,
             ["~/.claude/plugins: added [], removed [], changed ['cache/m/a/1.0/SKILL.md']"]),
        ]

        for name, change, expected in cases:
            with self.subTest(name):
                home = pathlib.Path(tempfile.mkdtemp(prefix="fuku-fake-home-"))
                self.addCleanup(shutil.rmtree, home)
                (home / ".codex" / "plugins").mkdir(parents=True)
                (home / ".codex" / "config.toml").write_text("model = 'm'\n", encoding="utf-8")
                (home / ".claude" / "plugins").mkdir(parents=True)
                (home / ".claude" / "plugins" / "installed_plugins.json").write_text(
                    json.dumps({"version": 2, "plugins": {"a@m": [{"version": "0.1.0"}]}}), encoding="utf-8")
                (home / ".claude" / "plugins" / "cache" / "m" / "a" / "1.0").mkdir(parents=True)
                (home / ".claude" / "plugins" / "cache" / "m" / "a" / "1.0" / "SKILL.md").write_text(
                    "name: a\n", encoding="utf-8")
                (home / ".claude" / "plugins" / "cache" / "m" / "current").symlink_to("a")
                before = report.guard_snapshot(home)
                change(home)

                changes = report.compare(before, report.guard_snapshot(home))

                self.assertEqual(expected, changes)

    def test_missing_files_are_a_baseline(self):
        home = pathlib.Path(tempfile.mkdtemp(prefix="fuku-fake-home-"))
        self.addCleanup(shutil.rmtree, home)

        snapshot = report.guard_snapshot(home)

        self.assertEqual([], report.compare(snapshot, report.guard_snapshot(home)))
        self.assertIsNone(snapshot["~/.codex/config.toml"])

    def test_guard_failure_fails_the_sweep(self):
        home = pathlib.Path(tempfile.mkdtemp(prefix="fuku-fake-home-"))
        self.addCleanup(shutil.rmtree, home)

        def install(secrets):
            (home / ".codex" / "plugins" / "fuku").mkdir(parents=True)
            return canned()

        adapter = CannedAdapter([install])

        code, _, stderr, _ = sweep(self, {"T1": SCENARIO}, {"HOST": "codex", "REPEATS": "1", "HOME": str(home)},
                                   {"codex": lambda: adapter})

        self.assertEqual(1, code)
        self.assertIn("guard: ~/.codex/plugins fuku paths: added ['fuku'], removed []", stderr)

if __name__ == "__main__":
    unittest.main()
