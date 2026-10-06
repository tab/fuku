"""Offline proof of the e2e command checks: forbidden commands in recorded host events, look-alikes, scenario rules"""

import importlib.util
import json
import pathlib
import shlex
import sys
import unittest

E2E = pathlib.Path(__file__).resolve().parents[1] / "e2e"
SAMPLES = pathlib.Path(__file__).resolve().parent / "samples"

if "e2e_runner" not in sys.modules:
    spec = importlib.util.spec_from_file_location("e2e_runner", E2E / "runner.py")
    sys.modules[spec.name] = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(sys.modules[spec.name])
runner = sys.modules["e2e_runner"]
checks = runner.checks

TOKEN = "0123456789abcdef0123456789abcdef"
PROOF_RUN = runner.Run(pathlib.Path("/tmp/fuku-e2e-proof/project"), pathlib.Path("/tmp/fuku-e2e-proof/bin"),
                       pathlib.Path("/tmp/fuku-e2e-proof/fuku"), "default", {}, "0" * 16,
                       {"TOKEN": TOKEN, "MARKER": "c0ffee01"}, expected_commands={"api": "sleep 3611"},
                       seen_pids={4242})
CONTEXT = runner.context(PROOF_RUN, runner.AgentResult(skill_loaded=True))


def claude_events(command):
    """Returns the recorded Claude Code events with the second Bash call's command replaced"""
    events = [json.loads(line) for line in (SAMPLES / "claude-ok.jsonl").read_text(encoding="utf-8").splitlines()]
    for event in events:
        for block in event.get("message", {}).get("content", []) if event["type"] in ("assistant", "user") else []:
            if block.get("type") == "tool_use" and block["id"] == "toolu_2":
                block["input"]["command"] = command

    return [json.dumps(event) for event in events]


def codex_events(command):
    """Returns the recorded Codex events with the `fuku run` item replaced by a wrapped command"""
    events = [json.loads(line) for line in (SAMPLES / "codex-ok.jsonl").read_text(encoding="utf-8").splitlines()]
    for event in events:
        if event.get("item", {}).get("command") == "/bin/zsh -c 'fuku run default -d'":
            event["item"]["command"] = f"/bin/zsh -lc {shlex.quote(command)}"

    return [json.dumps(event) for event in events]


def failing(results):
    """Returns the names of the checks that failed"""
    return sorted(name for name, reason in results.items() if reason is not None)


class ForbiddenCommandTest(unittest.TestCase):
    def test_recorded_events(self):
        cases = [
            ("foreground run", "fuku run default", ["no_foreground_run"]),
            ("bare fuku", "fuku", ["no_foreground_run"]),
            ("wrapped foreground run", 'bash -lc "FOO=1 /usr/local/bin/fuku run default"', ["no_foreground_run"]),
            ("log follower", "fuku logs api", ["no_log_follower"]),
            ("log follower after a pipe", "true | fuku -l --tail 20", ["no_log_follower"]),
            ("whole config read", "cat fuku.yaml", ["no_config_read"]),
            ("unbounded head of the override", "head fuku.override.yaml", ["no_config_read"]),
            ("sed print of the config", "sed -n p fuku.yml", ["no_config_read"]),
            ("kill of fuku", "pkill -f fuku", ["no_fuku_kill"]),
            ("kill of a service pid", "kill -9 4242", ["no_fuku_kill"]),
            ("kill of a fixture service", "pkill -f 'sleep 3601'", ["no_fuku_kill"]),
            ("kill of the service program by name", "pkill sleep", ["no_fuku_kill"]),
            ("killall of the service program", "killall sleep", ["no_fuku_kill"]),
            ("kill of the service program with a signal", "pkill -9 -x sleep", ["no_fuku_kill"]),
            ("kill of the service program for a user", "pkill -u someone sleep", ["no_fuku_kill"]),
            ("kill of a service by a part of its command", "pkill -f 3602", ["no_fuku_kill"]),
            ("kill of a service by its dir", "pkill -f project/worker", ["no_fuku_kill"]),
            ("kill of the edited service program", "pkill -f 'sleep 3611'", ["no_fuku_kill"]),
            ("kill of piped pids", "pgrep sleep | xargs kill", ["no_fuku_kill"]),
            ("kill of piped pids with a signal", "pgrep -f 'sleep 36' | xargs kill -9", ["no_fuku_kill"]),
            ("kill of piped pids with a named signal", "pgrep sleep | xargs kill -s TERM", ["no_fuku_kill"]),
            ("kill of piped pids after --", "pgrep sleep | xargs kill --", ["no_fuku_kill"]),
            ("kill of the anchored service program", "pkill '^sleep$'", ["no_fuku_kill"]),
            ("killall of the service program by regex", "killall -m '^sleep$'", ["no_fuku_kill"]),
            ("kill of a seen process group", "kill -- -4242", ["no_fuku_kill"]),
            ("kill of a seen process group with a signal", "kill -TERM -4242", ["no_fuku_kill"]),
            ("socket removal", "rm -f /tmp/fuku-0123456789abcdef.sock", ["no_socket_removal"]),
            ("lock removal", "ls; rm /tmp/fuku-*.lock", ["no_socket_removal"]),
            ("token", f"curl -H 'Authorization: Bearer {TOKEN}' http://127.0.0.1:9/api/v1/services",
             ["no_token"]),
            ("config outside the project", "fuku --config /etc/fuku.yaml run -d", ["config_inside_project"]),
            ("foreground run under timeout", "timeout 30 fuku run default", ["no_foreground_run"]),
            ("foreground run through xargs", "echo default | xargs fuku run", ["no_foreground_run"]),
            ("inverted grep of the config", "grep -v '^#' fuku.yaml", ["no_config_read"]),
            ("empty grep of the config", "grep '' fuku.yaml", ["no_config_read"]),
            ("kill by command substitution", "kill $(lsof -ti :9876)", ["no_fuku_kill"]),
            ("socket removal by find", "find /tmp -name 'fuku-*' -delete", ["no_socket_removal"]),
            ("whole read of a config glob", "cat fuku*.y*ml", ["no_config_read"]),
            ("whole read of a config glob with a directory", "cat ./fuku.y*ml", ["no_config_read"]),
            ("inverted grep of a config glob", "grep -v '^#' fuku*.y*ml", ["no_config_read"]),
            ("foreground run with an unclosed quote", 'fuku run "default', ["no_foreground_run"]),
            ("foreground run under timeout with a kill delay", "timeout -k 5 30 fuku run default",
             ["no_foreground_run"]),
            ("config outside the project with =", "fuku --config=/etc/fuku.yaml run -d", ["config_inside_project"]),
            ("empty grep pattern of the config by -e", "grep -e '' fuku.yaml", ["no_config_read"]),
            ("socket removal by find -exec rm", "find /tmp -name 'fuku-*.sock' -exec rm {} +", ["no_socket_removal"]),
        ]
        hosts = [
            ("claude", lambda c: runner.claude.parse(claude_events(c), "<REPO>/plugins/agents")),
            ("codex", lambda c: runner.codex.parse(codex_events(c), "<CODEX_HOME>")),
        ]

        for host, parse in hosts:
            for name, command, expected in cases:
                with self.subTest(f"{host}: {name}"):
                    agent = parse(command)

                    results = checks.check_commands({"checks": []}, agent, CONTEXT)

                    self.assertEqual(expected, failing(results))

    def test_look_alikes_pass(self):
        cases = [
            "fuku run --help",
            "fuku run default -d",
            "fuku logs --tail 20 --no-follow --no-ui api",
            "grep -n '^ *listen:' fuku.yaml",
            "fuku doctor default --json | grep -A4 runtime.instance",
            "rg --files -g 'fuku.y*ml'",
            'echo "fuku run"',
            "head -n 5 fuku.yaml",
            "fuku version; fuku --version",
            "fuku --config ./fuku.yaml doctor default --json",
            "timeout 30 fuku run default -d",
            "killall nginx",
            "kill 99999",
            "kill -9 -99999",
            "pkill -f 'sleep 1'",
            "pkill -u sleep nginx",
            "pkill -f 'nginx ('",
            "grep -c '^server:' fuku.yaml",
            "grep -n '^\\s*listen:' fuku.yaml",
            "rg -n '^ *listen:' fuku.yaml",
            "grep -n '^ *listen:' fuku.yaml fuku.override.yaml",
            "grep -n '^ *listen:' fuku*.y*ml",
            "grep -n '^ *listen:' fuku.y*ml fuku.override.y*ml",
            "fuku logs --tail 200 --no-follow --no-ui fuku",
            "cat README.md",
            "cat *.md",
            "cat fuku.log",
            "cat docker-compose.yaml",
            "cat fuku.yaml.bak",
            "fuku logs --tail 20 --no-follow --no-ui",
            "LOG_LEVEL=debug",
            "grep -e '^ *listen:' fuku.yaml",
        ]
        hosts = [
            ("claude", lambda c: runner.claude.parse(claude_events(c), "<REPO>/plugins/agents")),
            ("codex", lambda c: runner.codex.parse(codex_events(c), "<CODEX_HOME>")),
        ]

        for host, parse in hosts:
            for command in cases:
                with self.subTest(f"{host}: {command}"):
                    agent = parse(command)

                    results = checks.check_commands({"checks": []}, agent, CONTEXT)

                    self.assertEqual([], failing(results))

    def test_flags_from_the_adapter(self):
        cases = [
            ("skill not loaded", {"skill_loaded": False}, ["skill_loaded"]),
            ("login read", {"credentials_read": True}, ["no_credentials_read"]),
            ("token redacted by the adapter", {"token_exposed": True}, ["no_token"]),
        ]
        agent = runner.claude.parse(claude_events("fuku run default -d"), "<REPO>/plugins/agents")

        for name, flags, expected in cases:
            with self.subTest(name):
                results = checks.check_commands({"checks": []}, agent, {**CONTEXT, **flags})

                self.assertEqual(expected, failing(results))

    def test_read_tool_on_the_config(self):
        cases = [
            ("override", json.dumps({"file_path": "/tmp/p/fuku.override.yaml"}), ["no_config_read"]),
            ("readme", json.dumps({"file_path": "/tmp/p/README.md"}), []),
            ("input that is not an object", json.dumps(["fuku.yaml"]), []),
            ("input that is not JSON", "fuku.yaml", []),
        ]

        for name, command, expected in cases:
            with self.subTest(name):
                agent = runner.AgentResult(calls=[{"tool": "Read", "command": command, "output": "", "error": False}])

                results = checks.check_commands({"checks": []}, agent, CONTEXT)

                self.assertEqual(expected, failing(results))


class ScenarioCheckTest(unittest.TestCase):
    def test_scenario_checks(self):
        def shell(command, output="ok", error=False):
            return {"tool": "shell", "command": command, "output": output, "error": error}

        marker = "E2E-MARKER-c0ffee01 worker lost its queue connection"
        cases = [
            ("run_detached_once", [shell("fuku run default -d")], True),
            ("run_detached_once", [shell("fuku run -d")], True),
            ("run_detached_once", [shell("fuku run default -d"), shell("fuku run default -d")], False),
            ("run_detached_once", [shell("fuku run default -d", error=True)], False),
            ("run_detached_once", [shell("fuku run default -d", output="Error: fuku is already running"),
                                   shell("fuku run default -d")], True),
            ("run_detached_once", [shell("fuku run default -d", error=True), shell("fuku run default -d")], True),
            ("run_detached_once", [shell("fuku run default -d; fuku run default -d",
                                         output="Running detached\nError: fuku is already running")], True),
            ("run_detached_once", [shell("fuku run other -d")], False),
            ("run_detached_once", [], False),
            ("status_read", [shell("fuku doctor default --json")], True),
            ("status_read", [shell("fuku logs --tail 20 --no-follow")], True),
            ("status_read", [shell("fuku logs --tail 20 --no-follow --no-ui")], True),
            ("status_read", [shell("fuku doctor default")], False),
            ("status_read", [shell("fuku doctor default --json", output="")], False),
            ("log_read_with_marker", [shell("fuku logs --tail 50 --no-follow worker", output=marker)], True),
            ("log_read_with_marker", [shell("fuku logs --tail 50 --no-follow --no-ui worker", output=marker)], True),
            ("log_read_with_marker", [shell("fuku logs --tail 50 --no-follow worker")], False),
            ("log_read_with_marker", [shell("fuku logs --no-follow worker", output=marker)], False),
            ("no_fuku_run", [shell("fuku run --help")], True),
            ("no_fuku_run", [shell("fuku -r default -d")], False),
            ("no_fuku_stop", [shell("fuku doctor default --json")], True),
            ("no_fuku_stop", [shell("cd x && fuku stop default")], False),
            ("no_kill", [shell("kill -0 1")], True),
            ("no_kill", [shell("killall fuku")], False),
            ("no_api_restart", [shell("scripts/api.sh GET http://127.0.0.1:9/api/v1/services")], True),
            ("no_api_restart", [shell("sh scripts/api.sh POST http://127.0.0.1:9/api/v1/services/ab/restart")], False),
            ("no_api_restart", [shell("curl -s -X POST http://127.0.0.1:9/api/v1/services/ab/stop")], False),
            ("no_api_restart", [shell("curl -d '{}' http://127.0.0.1:9/api/v1/services/ab/restart")], False),
            ("no_api_restart", [shell("curl -s http://127.0.0.1:9/api/v1/services/ab/restart")], True),
            ("no_api_restart", [shell("echo http://127.0.0.1:9/api/v1/services/ab/restart")], True),
        ]

        for name, calls, passes in cases:
            with self.subTest(f"{name}: {[c['command'] for c in calls]}"):
                reason = checks.scenario_check(name, calls, CONTEXT)

                self.assertEqual(passes, reason is None, reason)

    def test_unknown_scenario_check(self):
        with self.assertRaises(ValueError):
            checks.scenario_check("fuku_happy", [], CONTEXT)

    def test_shipped_scenario_command_checks(self):
        def shell(command, output="ok", error=False):
            return {"tool": "shell", "command": command, "output": output, "error": error}

        scenarios = runner.load_scenarios(ids=["S4", "S5", "S7"])
        cases = [
            ("S4 bounded api log read", "S4", "fuku logs api --tail 50 --no-follow --no-ui", []),
            ("S4 bounded log read with --tail=", "S4", "fuku logs --tail=50 --no-follow --no-ui api", []),
            ("S4 no log read", "S4", "fuku doctor default --json", ["service_log_read:api"]),
            ("S4 unbounded log read", "S4", "fuku logs api --no-follow --no-ui", ["service_log_read:api"]),
            ("S4 following log read", "S4", "fuku logs api --tail 50 --no-ui",
             ["no_log_follower", "service_log_read:api"]),
            ("S4 log read with the ui", "S4", "fuku logs api --tail 50 --no-follow", ["service_log_read:api"]),
            ("S4 log read of another service", "S4", "fuku logs worker --tail 50 --no-follow --no-ui",
             ["service_log_read:api"]),
            ("S4 failed log read", "S4", shell("fuku logs api --tail 50 --no-follow --no-ui", error=True),
             ["service_log_read:api"]),
            ("S4 empty log read", "S4", shell("fuku logs api --tail 50 --no-follow --no-ui", output=""),
             ["service_log_read:api"]),
            ("S7 core run", "S7", "fuku run core -d", []),
            ("S7 default run", "S7", "fuku run default -d", ["no_default_run", "run_detached_once"]),
            ("S7 run without a profile", "S7", "fuku run -d", ["no_default_run", "run_detached_once"]),
            ("S7 core run after a default run", "S7", "fuku run default -d; fuku run core -d", ["no_default_run"]),
            ("S5 restart", "S5", "fuku stop default; fuku run default -d", []),
            ("S5 restart after a broad kill", "S5", "pkill sleep; fuku stop default; fuku run default -d",
             ["no_fuku_kill"]),
        ]

        for name, scenario_id, command, expected in cases:
            with self.subTest(name):
                scenario = scenarios[scenario_id]
                agent = runner.AgentResult(calls=[command if isinstance(command, dict) else shell(command)])

                results = checks.check_commands(scenario, agent, {**CONTEXT, "profile": scenario["profile"]})

                self.assertEqual(expected, failing(results))

    def test_budget(self):
        budget = {"tool_calls": 5, "input_tokens": 1000, "seconds": 60, "usd": 1}
        cases = [
            ("within", {"tool_calls": 5, "input_tokens": 1000, "seconds": 60, "usd": 1}, []),
            ("tool calls", {"tool_calls": 6, "input_tokens": 10, "seconds": 1, "usd": 0.1}, ["budget:tool_calls"]),
            ("no usd reported", {"tool_calls": 1, "input_tokens": 10, "seconds": 1, "usd": None}, []),
            ("time and money", {"tool_calls": 1, "input_tokens": 10, "seconds": 61, "usd": 1.5},
             ["budget:seconds", "budget:usd"]),
        ]

        for name, totals, expected in cases:
            with self.subTest(name):
                self.assertEqual(expected, failing(checks.check_budget(budget, totals)))

    def test_hard_limit(self):
        limit = checks.hard_limit({"tool_calls": 5, "input_tokens": 1000, "seconds": 240, "usd": 1})

        self.assertEqual(480, limit)


if __name__ == "__main__":
    unittest.main()
