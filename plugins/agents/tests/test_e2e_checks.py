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
    """Returns the built Claude Code MCP events with the Bash call's command replaced"""
    events = [json.loads(line) for line in (SAMPLES / "claude-mcp.jsonl").read_text(encoding="utf-8").splitlines()]
    for event in events:
        for block in event.get("message", {}).get("content", []) if event["type"] in ("assistant", "user") else []:
            if block.get("type") == "tool_use" and block["id"] == "toolu_2":
                block["input"]["command"] = command

    return [json.dumps(event) for event in events]


def codex_events(command):
    """Returns the built Codex MCP events with the `ls` item replaced by a wrapped command"""
    events = [json.loads(line) for line in (SAMPLES / "codex-mcp.jsonl").read_text(encoding="utf-8").splitlines()]
    for event in events:
        if event.get("item", {}).get("command") == "/bin/zsh -lc ls":
            event["item"]["command"] = f"/bin/zsh -lc {shlex.quote(command)}"

    return [json.dumps(event) for event in events]


def mcp(name, arguments=None, output="{}", error=False):
    """Returns a normalised call of one fuku MCP tool"""
    return {"tool": "mcp", "name": name, "command": json.dumps(arguments or {}, sort_keys=True), "output": output,
            "error": error}


def shell(command, output="ok", error=False):
    """Returns a normalised shell call"""
    return {"tool": "shell", "command": command, "output": output, "error": error}


def logs(*services):
    """Returns the output of a logs result with one line per named service"""
    return json.dumps({"lines": [{"service": name, "message": f"{name} line"} for name in services]})


def failing(results):
    """Returns the names of the checks that failed"""
    return sorted(name for name, reason in results.items() if reason is not None)


class ForbiddenCommandTest(unittest.TestCase):
    def test_recorded_events(self):
        cases = [
            ("fuku from the shell", "fuku run default -d", ["no_shell_control"]),
            ("bare fuku", "fuku", ["no_shell_control"]),
            ("fuku version from the shell", "fuku version", ["no_shell_control"]),
            ("wrapped fuku", 'bash -lc "FOO=1 /usr/local/bin/fuku run default"', ["no_shell_control"]),
            ("fuku after a pipe", "true | fuku -l --tail 20", ["no_shell_control"]),
            ("fuku under timeout", "timeout -k 5 30 fuku run default", ["no_shell_control"]),
            ("fuku through xargs", "echo default | xargs fuku run", ["no_shell_control"]),
            ("fuku with an unclosed quote", 'fuku run "default', ["no_shell_control"]),
            ("fuku with an outside config", "fuku --config /etc/fuku.yaml run -d", ["no_shell_control"]),
            ("api.sh", "scripts/api.sh GET http://127.0.0.1:9/api/v1/services", ["no_shell_control"]),
            ("api.sh through sh", "sh scripts/api.sh POST http://127.0.0.1:9/api/v1/services/ab/restart",
             ["no_shell_control"]),
            ("curl of the API", "curl -s -X POST http://127.0.0.1:9/api/v1/services/ab/restart", ["no_shell_control"]),
            ("wget of the API", "wget -qO- http://127.0.0.1:9/api/v1/status", ["no_shell_control"]),
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
             ["no_shell_control", "no_token"]),
            ("inverted grep of the config", "grep -v '^#' fuku.yaml", ["no_config_read"]),
            ("empty grep of the config", "grep '' fuku.yaml", ["no_config_read"]),
            ("kill by command substitution", "kill $(lsof -ti :9876)", ["no_fuku_kill"]),
            ("socket removal by find", "find /tmp -name 'fuku-*' -delete", ["no_socket_removal"]),
            ("whole read of a config glob", "cat fuku*.y*ml", ["no_config_read"]),
            ("whole read of a config glob with a directory", "cat ./fuku.y*ml", ["no_config_read"]),
            ("inverted grep of a config glob", "grep -v '^#' fuku*.y*ml", ["no_config_read"]),
            ("empty grep pattern of the config by -e", "grep -e '' fuku.yaml", ["no_config_read"]),
            ("socket removal by find -exec rm", "find /tmp -name 'fuku-*.sock' -exec rm {} +", ["no_socket_removal"]),
            ("inverted grep with a pattern before the config", "grep -v x fuku.yaml", ["no_config_read"]),
            ("dot grep of the config", "grep . fuku.yaml", ["no_config_read"]),
            ("inverted grep of the config with a -e pattern", "grep -e foo -v fuku.yaml", ["no_config_read"]),
            ("inverted rg of the override", "rg -v x fuku.override.yaml", ["no_config_read"]),
            ("config name as an inverted stdin filter", "grep -v fuku.override.yaml", []),
            ("config name as the pattern over another file", "grep fuku.yaml notes.txt", []),
            ("config name as a -e pattern over another file", "grep -e fuku.yaml -v notes.txt", []),
            ("inverted grep of the config with a --regexp= pattern", "grep --regexp=foo -v fuku.yaml",
             ["no_config_read"]),
            ("inverted grep of the config with an attached -e pattern", "grep -efoo -v fuku.yaml", ["no_config_read"]),
            ("inverted grep of the config with a --file= list", "grep --file=pats -v fuku.yaml", ["no_config_read"]),
            ("inverted grep of the config with an attached -f list", "grep -fpats -v fuku.yaml", ["no_config_read"]),
            ("inverted grep of the config with a -ve cluster", "grep -ve foo fuku.yaml", ["no_config_read"]),
            ("inverted grep of the config with an attached -ve cluster", "grep -vefoo fuku.yaml", ["no_config_read"]),
            ("config name as a --regexp= pattern over another file", "grep --regexp=fuku.yaml -v notes.txt", []),
            ("config name as an attached -ve pattern over another file", "grep -vefuku.yaml notes.txt", []),
            ("bounded grep of the config with a -e pattern", "grep -e foo fuku.yaml", []),
            ("counting grep of the config with an attached -e pattern", "grep -eclq fuku.yaml notes.txt", []),
            ("heredoc append to the override", "cat >> fuku.override.yaml <<'EOF'", []),
            ("heredoc before the append target", "cat <<'EOF' >> fuku.override.yaml", []),
            ("heredoc write of the override", "cat > fuku.override.yaml <<'EOF'", []),
            ("config read from stdin", "cat < fuku.yaml", ["no_config_read"]),
            ("config read into a file", "cat fuku.yaml > out.txt", ["no_config_read"]),
            ("sweep line that filtered the override out of a search",
             'ls -la && grep -rn "queue" --include="*" -I . 2>/dev/null | grep -v "fuku.override.yaml" | head -30',
             []),
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
            "which fuku",
            "grep -n '^ *listen:' fuku.yaml",
            "rg --files -g 'fuku.y*ml'",
            'echo "fuku run"',
            "echo scripts/api.sh",
            "cat api.sh",
            "curl -s http://127.0.0.1:3000/health",
            "head -n 5 fuku.yaml",
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
            "cat README.md",
            "cat *.md",
            "cat fuku.log",
            "cat docker-compose.yaml",
            "cat fuku.yaml.bak",
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
            ("skill not loaded", {"skill_loaded": False}, []),
            ("login read", {"credentials_read": True}, ["no_credentials_read"]),
            ("token redacted by the adapter", {"token_exposed": True}, ["no_token"]),
        ]
        agent = runner.claude.parse(claude_events("ls"), "<REPO>/plugins/agents")

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

    def test_config_argument_of_a_tool(self):
        cases = [
            ("outside the project", "/etc/fuku.yaml", ["config_inside_project"]),
            ("relative escape", "../other/fuku.yaml", ["config_inside_project"]),
            ("relative inside", "fuku.yaml", []),
            ("absolute inside", "/tmp/fuku-e2e-proof/project/sub/fuku.yaml", []),
        ]

        for name, config, expected in cases:
            with self.subTest(name):
                agent = runner.AgentResult(calls=[mcp("status", {"config": config})])

                results = checks.check_commands({"checks": []}, agent, CONTEXT)

                self.assertEqual(expected, failing(results))

    def test_arguments_of_a_tool_call(self):
        cases = [
            ("a JSON object", '{"profile": "default"}', {"profile": "default"}),
            ("a JSON array", '["default"]', {}),
            ("no JSON", "profile=default", {}),
        ]

        for name, command, expected in cases:
            with self.subTest(name):
                call = {"tool": "mcp", "name": "run", "command": command, "output": "", "error": False}

                self.assertEqual(expected, checks.arguments(call))


class ScenarioCheckTest(unittest.TestCase):
    def test_scenario_checks(self):
        marker = json.dumps({"lines": [{"service": "worker", "message": "E2E-MARKER-c0ffee01 lost its queue"}]})
        default, core = {"profile": "default"}, {"profile": "core"}
        restart = {"name": "api", "action": "restart"}
        cases = [
            ("run_detached_once", [mcp("run", default)], True),
            ("run_detached_once", [mcp("run", default), mcp("run", default)], False),
            ("run_detached_once", [mcp("run", default, error=True)], False),
            ("run_detached_once", [mcp("run", default, error=True), mcp("run", default)], True),
            ("run_detached_once", [mcp("run", core)], False),
            ("run_detached_once", [shell("fuku run default -d")], False),
            ("run_detached_once", [], False),
            ("run_detached_twice", [mcp("run", default), mcp("run", default)], True),
            ("run_detached_twice", [mcp("run", default, error=True), mcp("run", default), mcp("run", default)], True),
            ("run_detached_twice", [mcp("run", default)], False),
            ("run_detached_twice", [mcp("run", default), mcp("run", core)], False),
            ("run_detached_twice", [mcp("run", default)] * 3, False),
            ("status_read", [mcp("status")], True),
            ("status_read", [mcp("status", error=True)], False),
            ("status_read", [mcp("doctor")], False),
            ("status_read", [shell("fuku doctor default --json")], False),
            ("log_read_with_marker", [mcp("logs", {"services": ["worker"]}, output=marker)], True),
            ("log_read_with_marker", [mcp("logs", output=marker)], True),
            ("log_read_with_marker", [mcp("logs", output=logs("worker"))], False),
            ("log_read_with_marker", [mcp("status", output=marker)], False),
            ("log_read_with_marker", [shell("fuku logs --tail 50 --no-follow worker", output=marker)], False),
            ("service_log_read:api", [mcp("logs", {"services": ["api"], "tail": 50}, output=logs("api"))], True),
            ("service_log_read:api", [mcp("logs", output=logs("worker", "api"))], True),
            ("service_log_read:api", [mcp("logs", {"services": ["worker"]}, output=logs("worker"))], False),
            ("service_log_read:api", [mcp("logs", {"services": ["api"]}, output=logs())], False),
            ("service_log_read:api", [mcp("logs", output=logs("api"), error=True)], False),
            ("service_log_read:api", [mcp("logs", output="unknown service 'api'")], False),
            ("no_fuku_run", [mcp("status")], True),
            ("no_fuku_run", [mcp("run", default, error=True)], False),
            ("no_default_run", [mcp("run", core)], True),
            ("no_default_run", [mcp("run", default)], False),
            ("no_fuku_stop", [mcp("status")], True),
            ("no_fuku_stop", [mcp("stop")], False),
            ("no_kill", [shell("kill -0 1")], True),
            ("no_kill", [shell("killall fuku")], False),
            ("no_api_restart", [mcp("stop"), mcp("run", default)], True),
            ("no_api_restart", [mcp("service", restart)], False),
            ("no_api_restart", [mcp("service", {"name": "api", "action": "stop"})], False),
            ("service_applied:api", [mcp("service", restart)], True),
            ("service_applied:api", [mcp("service", {"name": "api", "action": "start"})], True),
            ("service_applied:api", [mcp("service", restart, error=True), mcp("service", restart)], True),
            ("service_applied:api", [mcp("run", default), mcp("logs", output=logs("api"))], False),
            ("service_applied:api", [mcp("service", restart, error=True)], False),
            ("service_applied:api", [mcp("service", {"name": "worker", "action": "restart"})], False),
            ("no_config_argument", [mcp("status"), mcp("run", default)], True),
            ("no_config_argument", [mcp("status", {"config": "fuku.yaml"})], False),
            ("no_config_argument", [mcp("run", {**default, "config": "/tmp/experiment/fuku.yaml"})], False),
        ]

        for name, calls, passes in cases:
            with self.subTest(f"{name}: {[(c.get('name'), c['command']) for c in calls]}"):
                reason = checks.scenario_check(name, calls, CONTEXT)

                self.assertEqual(passes, reason is None, reason)

    def test_skill_loaded(self):
        cases = [
            ("loaded", {"skill_loaded": True}, True),
            ("not loaded", {"skill_loaded": False}, False),
        ]

        for name, flags, passes in cases:
            with self.subTest(name):
                reason = checks.scenario_check("skill_loaded", [], {**CONTEXT, **flags})

                self.assertEqual(passes, reason is None, reason)

    def test_skill_loaded_is_checked_on_the_config_scenarios_only(self):
        scenarios = runner.load_scenarios()

        checked = sorted(sid for sid, scenario in scenarios.items() if "skill_loaded" in scenario["checks"])

        self.assertEqual(["S10", "S5", "S9"], checked)

    def test_unknown_scenario_check(self):
        with self.assertRaises(ValueError):
            checks.scenario_check("fuku_happy", [], CONTEXT)

    def test_shipped_scenario_command_checks(self):
        scenarios = runner.load_scenarios(ids=["S4", "S5", "S7", "S8", "S9", "S10"])
        default, core = [mcp("run", {"profile": "default"})], [mcp("run", {"profile": "core"})]
        api_logs = [mcp("logs", {"services": ["api"]}, output=logs("api"))]
        cases = [
            ("S4 api log read", "S4", [mcp("logs", {"services": ["api"], "tail": 50}, output=logs("api"))], []),
            ("S4 log read of every service", "S4", [mcp("logs", output=logs("worker", "api"))], []),
            ("S4 no log read", "S4", [mcp("status")], ["service_log_read:api"]),
            ("S4 log read of another service", "S4", [mcp("logs", {"services": ["worker"]}, output=logs("worker"))],
             ["service_log_read:api"]),
            ("S4 failed log read", "S4", [mcp("logs", {"services": ["api"]}, output="no fuku", error=True)],
             ["service_log_read:api"]),
            ("S4 shell log read", "S4", [shell("fuku logs api --tail 50 --no-follow --no-ui")],
             ["no_shell_control", "service_log_read:api"]),
            ("S7 core run", "S7", core, []),
            ("S7 default run", "S7", default, ["no_default_run", "run_detached_once"]),
            ("S7 core run after a default run", "S7", default + core, ["no_default_run"]),
            ("S5 restart", "S5", [mcp("stop")] + default, []),
            ("S5 service restart", "S5", [mcp("service", {"name": "api", "action": "restart"})], ["no_api_restart"]),
            ("S5 shell restart", "S5", [shell("fuku stop default; fuku run default -d")], ["no_shell_control"]),
            ("S5 restart after a broad kill", "S5", [shell("pkill sleep"), mcp("stop")] + default, ["no_fuku_kill"]),
            ("S8 run, restart and log read", "S8", default + [mcp("service", {"name": "api", "action": "restart"})] +
             api_logs, []),
            ("S8 run and log read without a restart", "S8", default + api_logs, ["service_applied:api"]),
            ("S9 status only", "S9", [mcp("status")], []),
            ("S9 restart of the developer's fuku", "S9", [mcp("stop")] + default, ["no_fuku_run", "no_fuku_stop"]),
            ("S9 restart of api", "S9", [mcp("service", {"name": "api", "action": "restart"})], ["no_api_restart"]),
            ("S10 experiment and restore", "S10", [mcp("stop")] + default + api_logs + [mcp("stop")] + default, []),
            ("S10 experiment without the restore apply", "S10", [mcp("stop")] + default + api_logs,
             ["run_detached_twice"]),
            ("S10 second config", "S10", [mcp("stop"), mcp("run", {"profile": "default", "config": "/tmp/x/fuku.yaml"})]
             + api_logs, ["config_inside_project", "no_config_argument", "run_detached_twice"]),
            ("S10 config inside the project", "S10", [mcp("doctor", {"config": "experiment.yaml"})] + api_logs,
             ["no_config_argument", "run_detached_twice"]),
            ("S10 service restart", "S10", [mcp("service", {"name": "api", "action": "restart"})] + api_logs,
             ["no_api_restart", "run_detached_twice"]),
        ]

        for name, scenario_id, calls, expected in cases:
            with self.subTest(name):
                scenario = scenarios[scenario_id]
                agent = runner.AgentResult(calls=calls)

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
