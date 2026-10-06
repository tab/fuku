"""Offline self-checks of the agent e2e runner, and its fuku-backed checks when FUKU_BIN names a built fuku"""

import contextlib
import glob
import importlib.util
import io
import json
import os
import pathlib
import re
import shlex
import shutil
import signal
import socket
import subprocess
import sys
import tempfile
import threading
import time
import unittest
import urllib.request
from unittest import mock

PLUGIN = pathlib.Path(__file__).resolve().parents[1]
E2E = PLUGIN / "e2e"

spec = importlib.util.spec_from_file_location("e2e_runner", E2E / "runner.py")
runner = importlib.util.module_from_spec(spec)
sys.modules[spec.name] = runner
spec.loader.exec_module(runner)

claude = runner.claude
codex = runner.codex
hosts = claude.hosts

SAMPLES = pathlib.Path(__file__).resolve().parent / "samples"
FUKU_BIN = os.environ.get("FUKU_BIN", "")
HAS_FUKU = bool(FUKU_BIN) and os.path.isfile(FUKU_BIN)
FIXTURE_SLEEPS = "sleep 36[01][0-9]"

VALID = {
    "prompt": "Start the default profile.",
    "profile": "default",
    "setup": ["start_fuku"],
    "checks": ["fuku_running"],
    "budget": {"tool_calls": 10, "input_tokens": 1000, "seconds": 60, "usd": 1},
}


def fixture_sleeps():
    """Returns the pgrep lines of fixture service processes still alive"""
    return subprocess.run(["pgrep", "-fl", FIXTURE_SLEEPS], capture_output=True, text=True).stdout


def sample(name):
    """Returns the lines of a recorded host transcript"""
    return (SAMPLES / f"{name}.jsonl").read_text(encoding="utf-8").splitlines()


def gone(pid):
    """Waits briefly for a PID to disappear and reports whether it did"""
    deadline = time.monotonic() + 3
    while time.monotonic() < deadline:
        try:
            os.kill(pid, 0)
        except ProcessLookupError:
            return True
        time.sleep(0.05)

    return False


FAKE_CLAUDE = r"""#!/bin/sh
. "$(dirname "$0")/fake.conf"
printf '%s\n' "$@" > "$FAKE_DIR/argv"
env > "$FAKE_DIR/env"
case "$FAKE_MODE" in
ok) cat "$FAKE_SAMPLE" ;;
echo) sed "s|ls; fuku version|echo $MY_TOKEN $FAKE_RUN_TOKEN|" "$FAKE_SAMPLE" ;;
garbage) echo "not an event" ;;
sleep) head -n 1 "$FAKE_SAMPLE"; sleep 3604 & echo $! > "$FAKE_DIR/child"; sleep 30 ;;
stubborn)
  sh -c 'trap "" TERM; echo $$ > "$0/child"; exec sleep 3606' "$FAKE_DIR" &
  while [ ! -s "$FAKE_DIR/child" ]; do sleep 0.05; done
  cat "$FAKE_SAMPLE" ;;
fail) echo "boom" >&2; exit 3 ;;
stderr) printf '%s' "$FAKE_STDERR" >&2; exit 3 ;;
esac
"""

FAKE_CODEX = r"""#!/bin/sh
. "$(dirname "$0")/fake.conf"
printf '%s\n' "$@" >> "$FAKE_DIR/argv"
case "$1" in
--version) echo "codex-cli 0.0.0-fake" ;;
plugin)
  [ "$2" = marketplace ] && exit 0
  case "$FAKE_TAMPER" in
  refuse) echo "no such plugin" >&2; exit 1 ;;
  garbled) echo "installed"; exit 0 ;;
  outside) printf '{"installedPath": "%s"}\n' "$FAKE_DIR"; exit 0 ;;
  esac
  dest="$CODEX_HOME/plugins/cache/fuku/fuku/0.1.0"
  mkdir -p "$dest" && cp -R "$FAKE_SKILLS" "$dest/skills"
  case "$FAKE_TAMPER" in
  edit) echo tampered >> "$dest/skills/fuku/SKILL.md" ;;
  extra) echo extra > "$dest/skills/fuku/EXTRA.md" ;;
  missing) rm "$dest/skills/fuku/references/control-api.md" ;;
  mode) chmod -x "$dest/skills/fuku/scripts/api.sh" ;;
  esac
  printf '{"installedPath": "%s"}\n' "$dest" ;;
exec)
  env > "$FAKE_DIR/env"
  ls -l "$CODEX_HOME/auth.json" | cut -c1-10 > "$FAKE_DIR/auth-mode"
  mkdir -p "$CODEX_HOME/sessions/x"
  echo '{"type":"turn_context","payload":{"model":"fake-model"}}' > "$CODEX_HOME/sessions/x/rollout.jsonl"
  case "$FAKE_MODE" in
  ok) sed "s|<CODEX_HOME>|$CODEX_HOME|g" "$FAKE_SAMPLE" ;;
  leak|split|base64|command)
    item() {
      out=$(printf '%s' "$2" | sed 's/\\/\\\\/g; s/"/\\"/g' | tr -d '\n')
      cmd=$(printf '%s' "$1" | sed 's/\\/\\\\/g; s/"/\\"/g')
      printf '{"type":"item.completed","item":{"id":"i","type":"command_execution","command":"%s",' "$cmd"
      printf '"aggregated_output":"%s","exit_code":0,"status":"completed"}}\n' "$out"
    }
    login="$CODEX_HOME/auth.json"
    case "$FAKE_MODE" in
    leak) item "cat notes.txt" "$(cat "$login")" ;;
    split) item 'head -c 40 $CODEX_HOME/auth.json' "$(head -c 40 "$login")"
           item 'tail -c +41 $CODEX_HOME/auth.json' "$(tail -c +41 "$login")" ;;
    base64) item 'base64 < $CODEX_HOME/auth.json' "$(base64 < "$login")" ;;
    command) item "$FAKE_COMMAND" "visible output" ;;
    esac
    echo '{"type":"turn.completed","usage":{"input_tokens":1,"cached_input_tokens":0,"output_tokens":1}}' ;;
  rotate)
    printf '{"tokens": {"refresh_token": "ROTATED-REFRESH-SECRET"}}' > "$CODEX_HOME/auth.json"
    sed "s|<CODEX_HOME>|$CODEX_HOME|g" "$FAKE_SAMPLE" ;;
  stash)
    mkdir -p "$CODEX_HOME/../stash" && cp "$CODEX_HOME/auth.json" "$CODEX_HOME/../stash/auth.json"
    sed "s|<CODEX_HOME>|$CODEX_HOME|g" "$FAKE_SAMPLE" ;;
  garbage) echo "not an event" ;;
  garbage_fail) echo "not an event"; echo "boom" >&2; exit 2 ;;
  sleep) sleep 3605 & echo $! > "$FAKE_DIR/child"; sleep 30 ;;
  fail) echo "boom" >&2; exit 2 ;;
  stderr) printf '%s' "$FAKE_STDERR" >&2; exit 2 ;;
  detach)
    "$FAKE_PYTHON" -c 'import os; os.setsid(); os.execvp("sleep", ["sleep", "3608"])' &
    echo $! > "$FAKE_DIR/detached"
    sleep 3605 &
    echo "$$ $!" > "$FAKE_DIR/host.tmp" && mv "$FAKE_DIR/host.tmp" "$FAKE_DIR/host"
    sleep 30 ;;
  interrupt) kill -INT "$PPID"; sleep 30 ;;
  esac ;;
esac
"""


def fake_host(test, name, script, **conf):
    """Writes a stand-in host executable and its config file into a temp dir the test removes"""
    directory = pathlib.Path(tempfile.mkdtemp(prefix=f"fuku-fake-{name}-"))
    test.addCleanup(shutil.rmtree, directory)
    path = directory / name
    path.write_text(script, encoding="utf-8")
    path.chmod(0o755)
    lines = [f"FAKE_DIR={shlex.quote(str(directory))}"] + [f"{k}={shlex.quote(str(v))}" for k, v in conf.items()]
    (directory / "fake.conf").write_text("\n".join(lines) + "\n", encoding="utf-8")

    return directory


def env_names(directory):
    """Returns the variable names a fake host saw, without those the shell sets itself"""
    names = {line.split("=", 1)[0] for line in (directory / "env").read_text().splitlines() if "=" in line}

    return names - {"PWD", "OLDPWD", "SHLVL", "_"}


def temp_roots():
    """Returns the runner's temp roots that exist"""
    return set(glob.glob(os.path.join(tempfile.gettempdir(), "fuku-agents-e2e-*")))


class StartFuku:
    """Stand-in agent that starts a profile in the background"""

    def __init__(self, profile="default"):
        self.profile = profile

    def run(self, project, prompt, env, **_):
        subprocess.run(["fuku", "run", self.profile, "-d"], cwd=project, env=env, capture_output=True, check=True)
        return runner.AgentResult()


class Idle:
    """Stand-in agent that does nothing"""

    def run(self, project, prompt, env, **_):
        return runner.AgentResult()


class RestartFuku:
    """Stand-in agent that stops the running fuku and starts a new one"""

    def run(self, project, prompt, env, **_):
        subprocess.run(["fuku", "stop", "default"], cwd=project, env=env, capture_output=True, check=True)
        subprocess.run(["fuku", "run", "default", "-d"], cwd=project, env=env, capture_output=True, check=True)
        return runner.AgentResult()


class ReportedRestart(RestartFuku):
    """Stand-in agent that restarts fuku and reports a transcript that also kills the service program by name"""

    def run(self, project, prompt, env, **_):
        super().run(project, prompt, env)
        call = {"tool": "shell", "command": "pkill sleep; fuku stop default; fuku run default -d", "output": "",
                "error": False}
        return runner.AgentResult(calls=[call], skill_loaded=True)


class StartOtherFuku:
    """Stand-in agent that starts the profile with a fuku binary other than the built one"""

    def __init__(self, binary):
        self.binary = binary

    def run(self, project, prompt, env, **_):
        subprocess.run([self.binary, "run", "default", "-d"], cwd=project, env=env, capture_output=True, check=True)
        return runner.AgentResult()


class RestartApiService:
    """Stand-in agent that restarts the api service through the loopback API with the run's token"""

    def run(self, project, prompt, env, **_):
        override = (project / "fuku.override.yaml").read_text(encoding="utf-8")
        port = re.search(r"127\.0\.0\.1:(\d+)", override).group(1)
        token = re.search(r'token: "([0-9a-f]+)"', override).group(1)
        opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))

        def call(method, path):
            request = urllib.request.Request(f"http://127.0.0.1:{port}/api/v1{path}", method=method,
                                             headers={"Authorization": f"Bearer {token}"})
            with opener.open(request, timeout=5) as response:
                return json.loads(response.read() or b"null")

        api = next(s for s in call("GET", "/services")["services"] if s["name"] == "api")
        call("POST", f"/services/{api['id']}/restart")
        deadline = time.monotonic() + 10
        while time.monotonic() < deadline:
            now = call("GET", f"/services/{api['id']}")
            if now["status"] == "running" and now["pid"] not in (0, api["pid"]):
                break
            time.sleep(0.2)
        return runner.AgentResult()


class LeaveProcess:
    """Stand-in agent that leaves an unmanaged process working in a service dir"""

    def __init__(self, test):
        self.test = test
        self.process = None

    def run(self, project, prompt, env, **_):
        self.process = subprocess.Popen(["sh", "-c", "exec sleep 3603"], cwd=project / "api", start_new_session=True)
        self.test.addCleanup(self.process.wait)
        self.test.addCleanup(self.process.kill)
        return runner.AgentResult()


class MoveServiceDir:
    """Stand-in agent that points the worker's dir at a directory outside the project"""

    def __init__(self, directory):
        self.directory = directory

    def run(self, project, prompt, env, **_):
        config = project / "fuku.yaml"
        config.write_text(config.read_text(encoding="utf-8").replace("dir: worker", f"dir: {self.directory}"),
                          encoding="utf-8")
        return runner.AgentResult()


class ScenarioTest(unittest.TestCase):
    def test_shipped_scenarios_validate(self):
        scenarios = runner.load_scenarios()

        self.assertEqual(["S1", "S2", "S3", "S4", "S5", "S6", "S7"], sorted(scenarios))

    def test_rejected_scenario(self):
        cases = [
            ("unknown key", {**VALID, "notes": "x"}, "keys must be exactly"),
            ("missing key", {k: v for k, v in VALID.items() if k != "budget"}, "keys must be exactly"),
            ("blank prompt", {**VALID, "prompt": "  "}, "prompt must be a non-empty string"),
            ("unknown profile", {**VALID, "profile": "staging"}, "unknown profile 'staging'"),
            ("setup as a string", {**VALID, "setup": "start_fuku"}, "setup must be a list of names"),
            ("unknown setup", {**VALID, "setup": ["start_fuku", "warm_cache"]}, "unknown setup 'warm_cache'"),
            ("unknown check", {**VALID, "checks": ["fuku_running", "fuku_happy"]}, "unknown check 'fuku_happy'"),
            ("parameter on a plain check", {**VALID, "checks": ["fuku_running:x"]}, "takes no parameter"),
            ("unknown service", {**VALID, "checks": ["service_pid_changed:db"]}, "needs a fixture service"),
            ("unknown service on a command check", {**VALID, "checks": ["service_log_read:db"]},
             "needs a fixture service"),
            ("unknown template", {**VALID, "setup": ["override:nope"]}, "names no template"),
            ("budget key", {**VALID, "budget": {"tool_calls": 1}}, "budget keys must be exactly"),
            ("zero budget", {**VALID, "budget": {**VALID["budget"], "seconds": 0}}, "positive number"),
            ("bool budget", {**VALID, "budget": {**VALID["budget"], "usd": True}}, "positive number"),
        ]

        for name, data, message in cases:
            with self.subTest(name), tempfile.TemporaryDirectory() as directory:
                pathlib.Path(directory, "S9.json").write_text(json.dumps(data), encoding="utf-8")
                stderr = io.StringIO()

                with contextlib.redirect_stderr(stderr):
                    code = runner.main(directory, {"FUKU_BIN": FUKU_BIN})

                self.assertEqual(2, code)
                self.assertIn(message, stderr.getvalue())
                with self.assertRaises(runner.UsageError):
                    runner.load_scenarios(directory)

    def test_unreadable_scenario_directory(self):
        cases = [
            ("no scenario file", {}, "no scenario file in"),
            ("not JSON", {"S9.json": "{not json"}, "S9.json: not valid JSON"),
        ]

        for name, files, message in cases:
            with self.subTest(name), tempfile.TemporaryDirectory() as directory:
                for file_name, text in files.items():
                    pathlib.Path(directory, file_name).write_text(text, encoding="utf-8")
                stderr = io.StringIO()

                with contextlib.redirect_stderr(stderr):
                    code = runner.main(directory, {"FUKU_BIN": FUKU_BIN})

                self.assertEqual(2, code)
                self.assertIn(message, stderr.getvalue())

    def test_missing_fuku_binary(self):
        stderr = io.StringIO()

        with contextlib.redirect_stderr(stderr):
            code = runner.main(runner.SCENARIOS, {"FUKU_BIN": "/nonexistent/fuku"})

        self.assertEqual(2, code)
        self.assertIn("FUKU_BIN must name the built fuku", stderr.getvalue())

    def test_temp_dir_inside_repository(self):
        fuku_bin = sys.executable
        repo = pathlib.Path(tempfile.gettempdir()).resolve()
        stderr = io.StringIO()

        with mock.patch.object(runner, "REPO", repo), contextlib.redirect_stderr(stderr):
            code = runner.main(runner.SCENARIOS, {"FUKU_BIN": fuku_bin})

        self.assertEqual(2, code)
        self.assertIn("is inside the repository", stderr.getvalue())

    def test_no_skill_under_e2e(self):
        skills = list(E2E.rglob("SKILL.md"))

        self.assertEqual([], skills)


class RunnerTest(unittest.TestCase):
    def test_prepare_inside_repository_leaves_nothing(self):
        roots_before = temp_roots()

        with mock.patch.object(runner, "REPO", pathlib.Path(tempfile.gettempdir()).resolve()):
            with self.assertRaises(runner.UsageError):
                runner.prepare(pathlib.Path(sys.executable), "default")

        self.assertEqual(roots_before, temp_roots())

    def test_probe_failure(self):
        cases = [
            ("no answer", [sys.executable, "-c", "import time; time.sleep(5)"],
             f"{sys.executable} did not answer within 0.2s"),
            ("missing program", ["/nonexistent/lsof"], "/nonexistent/lsof could not run: No such file or directory"),
        ]

        for name, args, message in cases:
            with self.subTest(name):
                with mock.patch.object(runner, "PROBE_SECONDS", 0.2), self.assertRaises(runner.ProbeError) as raised:
                    runner.capture(args)

                self.assertEqual(message, str(raised.exception))

    def test_instance_status(self):
        fingerprint = "0123456789abcdef"
        status = {"type": "status", "fingerprint": fingerprint, "pid": 7, "services": ["api"]}
        cases = [
            ("status of this project", json.dumps(status), status),
            ("status of another project", json.dumps({**status, "fingerprint": "f" * 16}), None),
            ("not a status", json.dumps({**status, "type": "error"}), None),
            ("not JSON", "fuku 0.0.0", None),
        ]

        for name, answer, expected in cases:
            with self.subTest(name), tempfile.TemporaryDirectory() as directory:
                directory = pathlib.Path(directory)
                run = runner.Run(directory, directory, directory / "fuku", "default", {}, fingerprint)
                server = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
                self.addCleanup(server.close)
                server.bind(str(directory / f"fuku-{fingerprint}.sock"))
                server.listen(1)

                def serve(text=answer):
                    connection, _ = server.accept()
                    with connection:
                        connection.makefile("rb").readline()
                        connection.sendall(text.encode() + b"\n")

                thread = threading.Thread(target=serve, daemon=True)
                thread.start()

                with mock.patch.object(runner, "SOCKET_DIR", directory):
                    actual = runner.instance_status(run)

                thread.join(timeout=5)
                self.assertEqual(expected, actual)

    def test_snapshot(self):
        project = pathlib.Path("/nowhere/project")
        status = {"type": "status", "pid": 7, "services": ["api", "worker"]}
        children = {"return_value": {11: 7, 12: 7, 13: 1}}
        cwds = {11: str(project / "api"), 12: "/elsewhere", 13: str(project / "worker")}
        probe_fails = {"side_effect": runner.ProbeError("ps did not answer within 10s")}
        cases = [
            ("no fuku", None, children, runner.Snapshot(), set()),
            ("services by their dir", status, children, runner.Snapshot(7, {"api": 11, "worker": None}), {7, 11}),
            ("process probe fails", status, probe_fails, runner.Snapshot(7), set()),
        ]

        for name, answer, processes, expected, seen in cases:
            with self.subTest(name), mock.patch.object(runner, "instance_status", return_value=answer), \
                    mock.patch.object(runner, "processes", **processes), \
                    mock.patch.object(runner, "lsof_paths",
                                      side_effect=lambda pids, kind: {p: cwds[p] for p in pids if p in cwds}):
                run = runner.Run(project, project, project / "fuku", "default", {}, "0" * 16)

                actual = runner.snapshot(run)

                self.assertEqual((expected, seen), (actual, run.seen_pids))

    def test_setup_steps(self):
        directory = pathlib.Path(tempfile.mkdtemp(prefix="fuku-fake-setup-"))
        self.addCleanup(shutil.rmtree, directory)
        shutil.copy(runner.FIXTURE / "fuku.yaml", directory / "fuku.yaml")
        values = {"PORT": "40123", "TOKEN": "0123456789abcdef0123456789abcdef", "MARKER": "c0ffee01"}
        run = runner.Run(directory, directory, directory / "fuku", "default", {}, "0" * 16, values=values)

        runner.SETUP_STEPS["override"](run, "api")
        runner.SETUP_STEPS["edit_service_command"](run, "api")
        with mock.patch.object(runner, "snapshot", return_value=runner.Snapshot(7, {"api": 11, "worker": None})):
            runner.SETUP_STEPS["wait_service_exited"](run, "worker")

        override = (directory / "fuku.override.yaml").read_text(encoding="utf-8")
        config = (directory / "fuku.yaml").read_text(encoding="utf-8")
        self.assertIn('listen: "127.0.0.1:40123"', override)
        self.assertIn('token: "0123456789abcdef0123456789abcdef"', override)
        self.assertNotIn("{{", override)
        self.assertIn('command: echo "api ready"; exec sleep 3611', config)
        self.assertIn('command: echo "worker ready"; exec sleep 3602', config)
        self.assertEqual({"api": "sleep 3611"}, run.expected_commands)

    def test_setup_failures(self):
        directory = pathlib.Path(tempfile.mkdtemp(prefix="fuku-fake-setup-"))
        self.addCleanup(shutil.rmtree, directory)
        (directory / "fuku").write_text('#!/bin/sh\necho "Error: no config" >&2\nexit 1\n', encoding="utf-8")
        (directory / "fuku").chmod(0o755)
        (directory / "fuku.yaml").write_text("services:\n  api:\n    command: echo hi\n", encoding="utf-8")
        hung = mock.Mock(side_effect=subprocess.TimeoutExpired(["fuku"], 30))
        running = mock.Mock(return_value=runner.Snapshot(7, {"worker": 99}))
        cases = [
            ("fuku run fails", "start_fuku", "", {}, "fuku run default -d exited 1: Error: no config"),
            ("fuku run hangs", "start_fuku", "", {"fuku": hung}, "fuku run default -d did not return within 30s"),
            ("service never exits", "wait_service_exited", "worker", {"WAIT_SECONDS": 0.3, "snapshot": running},
             "service 'worker' still runs after 0.3s"),
            ("command without exec sleep", "edit_service_command", "api", {},
             "service 'api' has no 'exec sleep <n>' command to edit"),
        ]

        for name, step, param, patches, message in cases:
            with self.subTest(name), contextlib.ExitStack() as stack:
                for attribute, value in patches.items():
                    stack.enter_context(mock.patch.object(runner, attribute, value))
                run = runner.Run(directory, directory, directory / "fuku", "default",
                                 {"PATH": f"{directory}:/usr/bin:/bin"}, "0" * 16)

                with self.assertRaises(runner.SetupError) as raised:
                    runner.SETUP_STEPS[step](run, param)

                self.assertEqual(message, str(raised.exception))

    def test_state_checks_without_fuku(self):
        running = runner.Snapshot(10, {"api": 11, "worker": 12})
        api_restarted = runner.Snapshot(10, {"api": 21, "worker": 12})
        worker_stopped = runner.Snapshot(10, {"api": 11, "worker": None})
        cases = [
            ("fuku_running", running, running, True),
            ("fuku_running", running, runner.Snapshot(), False),
            ("services_running", running, running, True),
            ("services_running", running, worker_stopped, False),
            ("services_running", running, runner.Snapshot(10, {}), False),
            ("same_fuku_pid", running, running, True),
            ("same_fuku_pid", running, runner.Snapshot(20, running.services), False),
            ("same_fuku_pid", runner.Snapshot(), runner.Snapshot(), False),
            ("same_service_pids", running, running, True),
            ("same_service_pids", running, api_restarted, False),
            ("same_service_pids", runner.Snapshot(), runner.Snapshot(), False),
            ("fuku_pid_changed", running, runner.Snapshot(20, running.services), True),
            ("fuku_pid_changed", running, running, False),
            ("fuku_pid_changed", runner.Snapshot(), running, False),
            ("fuku_pid_changed", running, runner.Snapshot(), False),
            ("service_pid_changed:api", running, api_restarted, True),
            ("service_pid_changed:api", running, running, False),
            ("service_pid_changed:api", running, runner.Snapshot(10, {"api": None, "worker": 12}), False),
            ("service_pid_changed:api", runner.Snapshot(10, {"api": None, "worker": 12}), running, False),
            ("other_service_pids_same:api", running, api_restarted, True),
            ("other_service_pids_same:api", running, runner.Snapshot(10, {"api": 21, "worker": 22}), False),
            ("other_service_pids_same:api", running, runner.Snapshot(), False),
            ("other_service_pids_same:api", runner.Snapshot(), runner.Snapshot(), False),
            ("service_stopped:worker", running, worker_stopped, True),
            ("service_stopped:worker", running, runner.Snapshot(10, {"api": 11}), True),
            ("service_stopped:worker", running, running, False),
            ("service_command_is:api", running, runner.Snapshot(), False),
            ("served_by_built_fuku", running, runner.Snapshot(), False),
        ]

        for name, before, after, passes in cases:
            with self.subTest(f"{name}: {before} -> {after}"):
                run = runner.Run(pathlib.Path("/nowhere"), pathlib.Path("/nowhere"), pathlib.Path("/nowhere/fuku"),
                                 "default", {}, "0" * 16, before=before, after=after)

                reason = runner.check_state(name, run)

                self.assertEqual(passes, reason is None, reason)

    def test_unknown_state_check(self):
        run = runner.Run(pathlib.Path("/nowhere"), pathlib.Path("/nowhere"), pathlib.Path("/nowhere/fuku"), "default",
                         {}, "0" * 16)

        with self.assertRaises(ValueError):
            runner.check_state("fuku_happy", run)

    def test_probed_state_checks(self):
        probe = runner.ProbeError("ps did not answer within 10s")
        cases = [
            ("service_command_is:api", "capture", {"return_value": "sleep 3611\n"}, None),
            ("service_command_is:api", "capture", {"return_value": "sleep 3601\n"},
             "api runs 'sleep 3601', expected 'sleep 3611'"),
            ("service_command_is:api", "capture", {"side_effect": probe}, "ps did not answer within 10s"),
            ("served_by_built_fuku", "lsof_paths", {"return_value": {10: "/built/fuku"}}, None),
            ("served_by_built_fuku", "lsof_paths", {"return_value": {10: "/other/fuku"}},
             "fuku pid 10 runs '/other/fuku', not /built/fuku"),
            ("served_by_built_fuku", "lsof_paths", {"return_value": {}}, "fuku pid 10 runs 'None', not /built/fuku"),
            ("served_by_built_fuku", "lsof_paths", {"side_effect": probe}, "ps did not answer within 10s"),
        ]

        for name, target, probed, expected in cases:
            with self.subTest(f"{name}: {probed}"), mock.patch.object(runner, target, **probed):
                run = runner.Run(pathlib.Path("/nowhere"), pathlib.Path("/nowhere"), pathlib.Path("/built/fuku"),
                                 "default", {}, "0" * 16, expected_commands={"api": "sleep 3611"},
                                 after=runner.Snapshot(10, {"api": 11}))

                self.assertEqual(expected, runner.check_state(name, run))

    def test_pid_alive(self):
        reaped = subprocess.Popen([sys.executable, "-c", "pass"])
        reaped.wait()
        zombie = subprocess.Popen([sys.executable, "-c", "pass"])
        self.addCleanup(zombie.wait)
        os.waitid(os.P_PID, zombie.pid, os.WEXITED | os.WNOWAIT)
        cases = [
            ("this process", os.getpid(), True),
            ("init, which a user cannot signal", 1, True),
            ("a reaped process", reaped.pid, False),
            ("an exited child not yet reaped", zombie.pid, False),
        ]

        for name, pid, alive in cases:
            with self.subTest(name):
                self.assertEqual(alive, runner.pid_alive(pid))

    def test_stop_instance(self):
        cases = [
            ("fuku of this project", "project", True),
            ("fuku of another project", "other", False),
        ]

        for name, cwd, stopped in cases:
            with self.subTest(name):
                directory = pathlib.Path(tempfile.mkdtemp(prefix="fuku-stop-")).resolve()
                self.addCleanup(shutil.rmtree, directory)
                (directory / "project").mkdir()
                (directory / "other").mkdir()
                fuku = subprocess.Popen(["sleep", "3695"], cwd=directory / cwd, start_new_session=True)
                self.addCleanup(fuku.wait)
                self.addCleanup(fuku.kill)
                run = runner.Run(directory / "project", directory, directory / "fuku", "default", {}, "0" * 16)

                with mock.patch.object(runner, "instance_status", return_value={"pid": fuku.pid}):
                    runner.stop_instance(run)

                self.assertEqual(stopped, fuku.poll() is not None)

    def test_teardown_reports_a_probe_that_does_not_answer(self):
        run = runner.prepare(pathlib.Path(sys.executable), "default")

        with mock.patch.object(runner, "processes", side_effect=runner.ProbeError("ps did not answer within 10s")):
            reason = runner.teardown(run)

        self.assertEqual("left behind: ps did not answer within 10s", reason)
        self.assertFalse(run.project.parent.exists())

    def test_setup_failure_skips_the_agent(self):
        directory = pathlib.Path(tempfile.mkdtemp(prefix="fuku-fake-fuku-"))
        self.addCleanup(shutil.rmtree, directory)
        (directory / "fuku").write_text('#!/bin/sh\necho "Error: no config" >&2\nexit 1\n', encoding="utf-8")
        (directory / "fuku").chmod(0o755)
        agent = mock.Mock()
        roots_before = temp_roots()

        result = runner.run_scenario(runner.load_scenarios(ids=["S2"])["S2"], agent, directory / "fuku")

        self.assertEqual({"setup": "fuku run default -d exited 1: Error: no config", "teardown": None}, result.checks)
        agent.run.assert_not_called()
        self.assertEqual(roots_before, temp_roots())

    def test_idle_agent_fails_the_end_checks(self):
        roots_before = temp_roots()

        result = runner.run_scenario(runner.load_scenarios(ids=["S1"])["S1"], Idle(), pathlib.Path(sys.executable))

        failed = sorted(name for name, reason in result.checks.items() if reason is not None)
        self.assertEqual(["fuku_running", "run_detached_once", "served_by_built_fuku", "services_running",
                          "skill_loaded"], failed)
        self.assertEqual(roots_before, temp_roots())

    def test_default_adapters(self):
        environ = {"HOME": "/nowhere"}

        adapters = runner.default_adapters(environ)

        claude_adapter, codex_adapter = adapters["claude"](), adapters["codex"]()
        self.assertEqual(runner.PLUGIN, claude_adapter.plugin_dir)
        self.assertEqual((runner.REPO, environ), (codex_adapter.repo, codex_adapter.environ))

    def test_sigterm_cleans_up_the_sweep(self):
        directory = fake_host(self, "codex", FAKE_CODEX, FAKE_MODE="detach", FAKE_SKILLS=PLUGIN / "skills",
                              FAKE_PYTHON=sys.executable)
        (directory / ".codex").mkdir()
        (directory / ".codex" / "auth.json").write_text('{"fake": true}', encoding="utf-8")
        env = {"PATH": f"{directory}:{os.environ['PATH']}", "HOME": str(directory), "TMPDIR": tempfile.gettempdir(),
               "FUKU_BIN": sys.executable, "HOST": "codex", "SCENARIO": "S1", "REPEATS": "1"}
        codex_roots = os.path.join(tempfile.gettempdir(), "fuku-agents-codex-*")
        codex_before, roots_before = set(glob.glob(codex_roots)), temp_roots()
        sweep = subprocess.Popen([sys.executable, str(E2E / "runner.py")], env=env, stdout=subprocess.DEVNULL,
                                 stderr=subprocess.DEVNULL)
        self.addCleanup(sweep.wait)
        self.addCleanup(sweep.kill)
        deadline = time.monotonic() + 30
        while not (directory / "host").exists() and time.monotonic() < deadline:
            time.sleep(0.05)
        host, child = (int(pid) for pid in (directory / "host").read_text().split())
        detached = int((directory / "detached").read_text())
        self.addCleanup(runner.signal_each, [host, child, detached], signal.SIGKILL)
        copies = [path for path in glob.glob(os.path.join(codex_roots, "run-*", "auth.json"))
                  if os.path.dirname(os.path.dirname(path)) not in codex_before]

        sweep.send_signal(signal.SIGTERM)
        with contextlib.suppress(subprocess.TimeoutExpired):
            sweep.wait(timeout=0.2)
        sweep.send_signal(signal.SIGTERM)
        code = sweep.wait(timeout=60)

        self.assertEqual(1, len(copies))
        self.assertEqual(128 + signal.SIGTERM, code)
        self.assertEqual([], [path for path in copies if os.path.exists(path)])
        self.assertEqual(codex_before, set(glob.glob(codex_roots)))
        self.assertEqual(roots_before, temp_roots())
        self.assertEqual([True, True, True], [gone(pid) for pid in (host, child, detached)])


@unittest.skipUnless(HAS_FUKU, "needs a built fuku: make build && FUKU_BIN=$PWD/cmd/fuku python3 -m unittest "
                               "discover -s plugins/agents/tests -p 'test_e2e_*.py'")
class FukuBackedTest(unittest.TestCase):
    def test_state_checks(self):
        scenarios = runner.load_scenarios()
        fuku_bin = runner.fuku_binary({"FUKU_BIN": FUKU_BIN})
        other_dir = tempfile.mkdtemp(prefix="fuku-other-")
        other = shutil.copy2(fuku_bin, pathlib.Path(other_dir, "fuku-other"))
        self.addCleanup(shutil.rmtree, other_dir)

        cases = [
            ("S1 started", "S1", StartFuku(), {
                "fuku_running": True, "services_running": True, "served_by_built_fuku": True}),
            ("S1 idle", "S1", Idle(), {
                "fuku_running": False, "services_running": False, "served_by_built_fuku": False}),
            ("S1 other fuku", "S1", StartOtherFuku(other), {
                "fuku_running": True, "services_running": True, "served_by_built_fuku": False}),
            ("S2 attached", "S2", Idle(), {
                "fuku_running": True, "same_fuku_pid": True, "same_service_pids": True, "served_by_built_fuku": True}),
            ("S2 restarted", "S2", RestartFuku(), {
                "fuku_running": True, "same_fuku_pid": False, "same_service_pids": False}),
            ("S3 worker exited", "S3", Idle(), {
                "fuku_running": True, "same_fuku_pid": True, "same_service_pids": True}),
            ("S4 api restarted", "S4", RestartApiService(), {
                "service_pid_changed:api": True, "other_service_pids_same:api": True, "same_fuku_pid": True}),
            ("S4 idle", "S4", Idle(), {
                "service_pid_changed:api": False, "other_service_pids_same:api": True, "same_fuku_pid": True}),
            ("S4 profile restarted", "S4", RestartFuku(), {
                "service_pid_changed:api": True, "other_service_pids_same:api": False, "same_fuku_pid": False}),
            ("S5 restarted", "S5", RestartFuku(), {
                "fuku_pid_changed": True, "service_command_is:api": True, "services_running": True}),
            ("S5 not restarted", "S5", Idle(), {
                "fuku_pid_changed": False, "service_command_is:api": False}),
            ("S6 left alone", "S6", Idle(), {
                "same_fuku_pid": True, "same_service_pids": True}),
            ("S7 core started", "S7", StartFuku("core"), {
                "fuku_running": True, "services_running": True, "service_stopped:worker": True,
                "served_by_built_fuku": True}),
            ("S7 default started", "S7", StartFuku(), {
                "fuku_running": True, "services_running": True, "service_stopped:worker": False}),
        ]

        for name, scenario_id, agent, expected in cases:
            with self.subTest(name):
                tmp_before = sorted(glob.glob("/tmp/fuku-*"))
                roots_before = temp_roots()

                result = runner.run_scenario(scenarios[scenario_id], agent, fuku_bin)

                passed = {check: result.checks[check] is None for check in expected}
                self.assertEqual("", fixture_sleeps())
                self.assertEqual(expected, passed, result.checks)
                self.assertNotIn("setup", result.checks)
                self.assertIsNone(result.checks["teardown"])
                self.assertEqual(tmp_before, sorted(glob.glob("/tmp/fuku-*")))
                self.assertEqual(roots_before, temp_roots())

    def test_broad_kill_fails_a_valid_restart(self):
        scenario = runner.load_scenarios(ids=["S5"])["S5"]
        fuku_bin = runner.fuku_binary({"FUKU_BIN": FUKU_BIN})

        result = runner.run_scenario(scenario, ReportedRestart(), fuku_bin)

        failed = sorted(name for name, reason in result.checks.items() if reason is not None)
        self.assertEqual(["no_fuku_kill"], failed, result.checks)
        self.assertEqual("", fixture_sleeps())

    def test_teardown_kills_an_unmanaged_process_in_the_project(self):
        scenario = runner.load_scenarios(ids=["S2"])["S2"]
        fuku_bin = runner.fuku_binary({"FUKU_BIN": FUKU_BIN})
        agent = LeaveProcess(self)
        tmp_before = sorted(glob.glob("/tmp/fuku-*"))

        result = runner.run_scenario(scenario, agent, fuku_bin)

        self.assertIsNone(result.checks["teardown"])
        self.assertIsNotNone(agent.process.wait(timeout=5))
        self.assertEqual("", fixture_sleeps())
        self.assertEqual(tmp_before, sorted(glob.glob("/tmp/fuku-*")))

    def test_teardown_spares_a_foreign_process_it_saw(self):
        fuku_bin = runner.fuku_binary({"FUKU_BIN": FUKU_BIN})
        foreign = subprocess.Popen(["sleep", "3698"], cwd=tempfile.gettempdir(), start_new_session=True)
        self.addCleanup(foreign.wait)
        self.addCleanup(foreign.kill)
        run = runner.prepare(fuku_bin, "default")
        run.seen_pids.add(foreign.pid)

        reason = runner.teardown(run)

        self.assertIsNone(reason)
        self.assertIsNone(foreign.poll())
        self.assertFalse(run.project.parent.exists())

    def test_teardown_spares_a_process_in_a_rewritten_service_dir(self):
        scenario = runner.load_scenarios(ids=["S2"])["S2"]
        fuku_bin = runner.fuku_binary({"FUKU_BIN": FUKU_BIN})
        decoy_dir = tempfile.mkdtemp(prefix="fuku-decoy-")
        self.addCleanup(shutil.rmtree, decoy_dir)
        decoy = subprocess.Popen(["sh", "-c", "exec sleep 3697"], cwd=decoy_dir, start_new_session=True)
        self.addCleanup(decoy.wait)
        self.addCleanup(decoy.kill)
        tmp_before = sorted(glob.glob("/tmp/fuku-*"))

        result = runner.run_scenario(scenario, MoveServiceDir(os.path.realpath(decoy_dir)), fuku_bin)

        self.assertIsNone(result.checks["teardown"])
        self.assertIsNone(decoy.poll())
        self.assertEqual("", fixture_sleeps())
        self.assertEqual(tmp_before, sorted(glob.glob("/tmp/fuku-*")))

    def test_teardown_after_an_agent_error(self):
        scenario = runner.load_scenarios(ids=["S2"])["S2"]
        fuku_bin = runner.fuku_binary({"FUKU_BIN": FUKU_BIN})
        tmp_before = sorted(glob.glob("/tmp/fuku-*"))
        roots_before = temp_roots()

        class Interrupted:
            def run(self, project, prompt, env, **_):
                raise KeyboardInterrupt

        with self.assertRaises(KeyboardInterrupt):
            runner.run_scenario(scenario, Interrupted(), fuku_bin)

        self.assertEqual(tmp_before, sorted(glob.glob("/tmp/fuku-*")))
        self.assertEqual(roots_before, temp_roots())
        self.assertEqual("", fixture_sleeps())


class ParseTest(unittest.TestCase):
    def test_claude_samples(self):
        cases = [
            ("claude-ok", "<REPO>/plugins/agents", {
                "calls": [("Skill", '{"skill": "fuku:fuku"}', False),
                          ("shell", "ls; fuku version; fuku doctor default --json", False),
                          ("shell", 'fuku run default -d; echo "exit=$?"; fuku logs --tail 30 --no-follow', False)],
                "skill_loaded": True, "host_error": None, "kind": None,
                "answer": "The `default` profile is up, with both of its services running:\n\n| Service | State | PID |\n"
                          "|---|---|---|\n| `worker` | re…<cut>",
                "totals": {"tool_calls": 3, "turns": 5, "input_tokens": 58445, "cached_input_tokens": 36140,
                           "output_tokens": 401, "usd": 0.47510499999999994, "model": "claude-fable-5-1",
                           "host_version": "2.1.289"}}),
            ("claude-ok", "/elsewhere/plugins/agents", {
                "calls": [("Skill", '{"skill": "fuku:fuku"}', False),
                          ("shell", "ls; fuku version; fuku doctor default --json", False),
                          ("shell", 'fuku run default -d; echo "exit=$?"; fuku logs --tail 30 --no-follow', False)],
                "skill_loaded": False, "host_error": None, "kind": None,
                "answer": "The `default` profile is up, with both of its services running:\n\n| Service | State | PID |\n"
                          "|---|---|---|\n| `worker` | re…<cut>",
                "totals": {"tool_calls": 3, "turns": 5, "input_tokens": 58445, "cached_input_tokens": 36140,
                           "output_tokens": 401, "usd": 0.47510499999999994, "model": "claude-fable-5-1",
                           "host_version": "2.1.289"}}),
            ("claude-error", "<REPO>/plugins/agents", {
                "calls": [("shell", 'fuku logs --tail 5 --no-follow; echo "exit code: $?"', False)],
                "skill_loaded": False, "kind": "api",
                "answer": "You've hit your session limit · resets <TIME>",
                "host_error": "API error 429: You've hit your session limit · resets <TIME>",
                "totals": {"tool_calls": 1, "turns": 2, "input_tokens": 5615, "cached_input_tokens": 0,
                           "output_tokens": 109, "usd": 0.11773, "model": "claude-fable-5-1",
                           "host_version": "2.1.289"}}),
        ]

        for name, plugin_dir, expected in cases:
            with self.subTest(f"{name} from {plugin_dir}"):
                result = claude.parse(sample(name), plugin_dir)

                calls = [(c["tool"], c["command"], c["error"]) for c in result.calls]
                totals = {k: result.totals[k] for k in expected["totals"]}
                self.assertEqual(expected["calls"], calls)
                self.assertEqual(expected["skill_loaded"], result.skill_loaded)
                self.assertEqual(expected["host_error"], result.host_error)
                self.assertEqual(expected["kind"], result.host_error_kind)
                self.assertEqual(expected["totals"], totals)
                self.assertEqual(expected["answer"], result.answer)

    def test_claude_answer_without_result_event(self):
        text = {"type": "text", "text": "Stopping fuku now."}
        message = {"model": "claude-fable-5-1", "type": "message", "role": "assistant", "content": [text]}
        lines = sample("claude-ok")[:-1] + [json.dumps({"type": "assistant", "message": message})]

        result = claude.parse(lines, "<REPO>/plugins/agents")

        self.assertEqual("Stopping fuku now.", result.answer)

    def test_claude_two_fuku_plugins(self):
        lines = sample("claude-ok")
        init = json.loads(lines[0])
        init["plugins"].append({"name": "fuku", "path": "/elsewhere/plugins/cache/fuku", "source": "fuku@fuku"})
        lines[0] = json.dumps(init)

        result = claude.parse(lines, "<REPO>/plugins/agents")

        self.assertFalse(result.skill_loaded)
        self.assertEqual("isolation", result.host_error_kind)
        self.assertIn("2 plugins named fuku", result.host_error)

    def test_claude_tool_result_blocks(self):
        events = [json.loads(line) for line in sample("claude-ok")]
        events[5]["message"]["content"][0]["content"] = [
            {"type": "text", "text": "api\n"}, {"type": "image"}, "stray", {"type": "text", "text": "fuku.yaml\n"}]
        lines = [json.dumps(event) for event in events]
        lines.insert(1, "")

        result = claude.parse(lines, "<REPO>/plugins/agents")

        self.assertEqual(("ls; fuku version; fuku doctor default --json", "api\nfuku.yaml\n"),
                         (result.calls[1]["command"], result.calls[1]["output"]))

    def test_claude_host_failure(self):
        cases = [
            ("no result event", None, ("crash", "no result event")),
            ("answer", {"subtype": "success", "result": "Done."}, (None, None)),
            ("turn cap", {"subtype": "error_max_turns", "is_error": True}, (None, None)),
            ("budget cap", {"subtype": "error_max_budget_usd", "is_error": True}, (None, None)),
            ("login", {"subtype": "success", "is_error": True, "result": "Not logged in · Please run /login"},
             ("login", "not logged in: Not logged in · Please run /login")),
            ("api status", {"subtype": "success", "is_error": True, "api_error_status": 529, "result": "Overloaded"},
             ("api", "API error 529: Overloaded")),
            ("api terminal reason", {"subtype": "success", "terminal_reason": "api_error", "result": "Reset"},
             ("api", "API error None: Reset")),
            ("error during execution", {"subtype": "error_during_execution", "is_error": True},
             ("crash", "error during execution: None")),
            ("host error", {"subtype": "success", "is_error": True, "result": "Prompt is too long"},
             ("api", "host error: Prompt is too long")),
        ]

        for name, final, expected in cases:
            with self.subTest(name):
                self.assertEqual(expected, claude.host_failure(final))

    def test_codex_samples(self):
        cases = [
            ("codex-ok", {
                "calls": [("shell", "cat <CODEX_HOME>/plugins/cache/fuku/fuku/0.1.0/skills/fuku/SKILL.md", False),
                          ("shell", "fuku run default -d", False),
                          ("shell", "fuku logs --profile default --tail 30 --no-follow", False)],
                "skill_loaded": True,
                "answer": "Started the `default` profile in the background. Both services are running and ready:\n\n"
                          "- `api`\n- `worker`\n\nFuku is in the `running` phase (PID `1975`) and remains running.",
                "totals": {"tool_calls": 3, "turns": 1, "input_tokens": 99290, "cached_input_tokens": 72448,
                           "output_tokens": 460, "usd": None}}),
            ("codex-error", {
                "calls": [("shell", "fuku run default -d", True)],
                "skill_loaded": False,
                "answer": "The default profile wasn’t started: installed fuku 0.21.0 rejects the required background "
                          "flag `-d`.\n\nThe pre-start check found no running fuku instance. The profile contains `api` "
                          "and `worker`; neither was started. A fuku version supporting `fuku run default -d` is needed.",
                "totals": {"tool_calls": 1, "turns": 1, "input_tokens": 94641, "cached_input_tokens": 84480,
                           "output_tokens": 475, "usd": None}}),
        ]

        for name, expected in cases:
            with self.subTest(name):
                result = codex.parse(sample(name), "<CODEX_HOME>")

                calls = [(c["tool"], c["command"], c["error"]) for c in result.calls]
                totals = {k: result.totals[k] for k in expected["totals"]}
                self.assertEqual(expected["calls"], calls)
                self.assertEqual(expected["skill_loaded"], result.skill_loaded)
                self.assertIsNone(result.host_error)
                self.assertEqual(expected["totals"], totals)
                self.assertEqual(expected["answer"], result.answer)

    def test_codex_events(self):
        completed = json.dumps({"type": "turn.completed",
                                "usage": {"input_tokens": 3, "cached_input_tokens": 1, "output_tokens": 2}})

        def message(kind, text):
            return json.dumps({"type": kind, "item": {"id": "m", "type": "agent_message", "text": text}})

        change = {"id": "f", "type": "file_change", "changes": [{"path": "fuku.yaml", "kind": "update"}],
                  "status": "failed"}
        cases = [
            ("blank line", ["", completed], [], "", None, None),
            ("answer only from a completed message",
             [message("item.completed", "Done."), message("item.started", "Partial"), completed], [], "Done.", None,
             None),
            ("turn failed", ['{"type": "turn.failed", "error": {"message": "stream disconnected"}}'], [], "", "api",
             "stream disconnected"),
            ("turn failed without a message", ['{"type": "turn.failed"}'], [], "", "api", "turn failed"),
            ("errors without a turn", ['{"type": "error", "message": "Reconnecting 1/5"}',
                                       '{"type": "error", "message": "Reconnecting 5/5"}'], [], "", "crash",
             "Reconnecting 5/5"),
            ("errors before a completed turn", ['{"type": "error", "message": "Reconnecting 1/5"}', completed], [], "",
             None, None),
            ("no events", [], [], "", "crash", "no turn.completed event"),
            ("failed file change", [json.dumps({"type": "item.completed", "item": change}), completed],
             [("file_change", json.dumps({"changes": change["changes"], "status": "failed"}, sort_keys=True), True)],
             "", None, None),
        ]

        for name, lines, calls, answer, kind, error in cases:
            with self.subTest(name):
                result = codex.parse(lines, "<CODEX_HOME>")

                self.assertEqual(calls, [(c["tool"], c["command"], c["error"]) for c in result.calls])
                self.assertEqual((answer, kind, error), (result.answer, result.host_error_kind, result.host_error))

    def test_codex_command_cut_off_by_the_deadline(self):
        lines = sample("codex-ok")[:6]
        lines[5] = lines[5].replace("fuku run default -d", "fuku run default")

        result = codex.parse(lines, "<CODEX_HOME>")

        calls = [(c["tool"], c["command"], c["error"]) for c in result.calls]
        self.assertEqual([("shell", "cat <CODEX_HOME>/plugins/cache/fuku/fuku/0.1.0/skills/fuku/SKILL.md", False),
                          ("shell", "fuku run default", False)], calls)

    def test_codex_skill_read(self):
        path = "<CODEX_HOME>/plugins/cache/fuku/fuku/0.1.0/skills/fuku/SKILL.md"
        skill = (PLUGIN / "skills" / "fuku" / "SKILL.md").read_text(encoding="utf-8")
        numbered = "".join(f"{n:6}\t{line}\n" for n, line in enumerate(skill.splitlines(), 1))
        last = skill.rstrip().splitlines()[-1]
        skipped = "".join(line for n, line in enumerate(skill.splitlines(keepends=True), 1) if n != 10)
        cases = [
            ("cat", f"cat {path}", skill, 0, True),
            ("sed range", f"sed -n '1,200p' {path}", skill, 0, True),
            ("numbered and cut", f"nl -ba {path} | head -n 400", numbered, 0, True),
            ("head", f"head -n 400 {path}", skill, 0, True),
            ("head of the front matter", f"head -n 2 {path}", "---\nname: fuku\n", 0, False),
            ("sed of the front matter", f"sed -n '1,2p' {path}", "---\nname: fuku\n", 0, False),
            ("ls", f"ls -l {path}", f"-rw-r--r--  1 me  staff  9000 {path}\n", 0, False),
            ("echo", f"echo {path}", f"{path}\n", 0, False),
            ("grep of the name line", f"grep '^name:' {path}", "name: fuku\n", 0, False),
            ("failed cat", f"cat {path}", skill, 1, False),
            ("cat of another file", "cat README.md", skill, 0, False),
            ("cat to null", f"cat {path} > /dev/null", "", 0, False),
            ("sed past the frontmatter", f"sed -n '10,20p' {path}", "body\n", 0, False),
            ("sed of the first and last line", f"sed -n '2p;$p' {path}", f"name: fuku\n{last}\n", 0, False),
            ("sed skipping a line", f"sed -n '1,9p;11,$p' {path}", skipped, 0, False),
        ]
        completed = json.dumps({"type": "turn.completed", "usage": {}})

        for name, command, output, exit_code, loaded in cases:
            with self.subTest(name):
                item = {"id": "c", "type": "command_execution", "command": f"/bin/zsh -lc {shlex.quote(command)}",
                        "aggregated_output": output, "exit_code": exit_code, "status": "completed"}

                result = codex.parse([json.dumps({"type": "item.completed", "item": item}), completed],
                                     "<CODEX_HOME>")

                self.assertEqual(loaded, result.skill_loaded)

    def test_unreadable_events(self):
        cases = [
            ("claude malformed", claude.parse, sample("claude-ok")[:2] + ["{not json"], "<REPO>/plugins/agents"),
            ("claude unknown event", claude.parse, sample("claude-ok") + ['{"type": "surprise"}'],
             "<REPO>/plugins/agents"),
            ("claude no init", claude.parse, sample("claude-ok")[1:], "<REPO>/plugins/agents"),
            ("codex malformed", codex.parse, sample("codex-ok") + ["[1, 2]"], "<CODEX_HOME>"),
            ("codex unknown event", codex.parse, sample("codex-ok") + ['{"type": "surprise"}'], "<CODEX_HOME>"),
            ("codex unknown item", codex.parse,
             sample("codex-ok") + ['{"type": "item.completed", "item": {"id": "x", "type": "teleport"}}'],
             "<CODEX_HOME>"),
        ]

        for name, parse, lines, where in cases:
            with self.subTest(name), self.assertRaises(hosts.ParseError):
                parse(lines, where)

    def test_parse_error_redacts_before_cutting(self):
        line = "x" * 110 + "SECRET-VALUE-0123456789"
        error = hosts.ParseError("line 1: not an event", line)

        message = error.describe(["SECRET-VALUE-0123456789"])

        self.assertEqual("line 1: not an event: '" + "x" * 110 + "<redacted>'", message)
        self.assertNotIn("SECRET", message)

    def test_shell_text(self):
        cases = [
            ("/bin/zsh -c 'fuku run default -d'", "fuku run default -d"),
            ("/bin/zsh -lc 'fuku run default -d'", "fuku run default -d"),
            ('/bin/bash -lc "fuku logs --tail 5 --no-follow"', "fuku logs --tail 5 --no-follow"),
            ("/bin/zsh -c \"rg -n '\"'^ *listen:'\"' fuku.yaml\"", "rg -n '^ *listen:' fuku.yaml"),
            ("/bin/zsh -c 'command -v fuku\nfuku version'", "command -v fuku\nfuku version"),
            ("fuku run default -d", "fuku run default -d"),
            ("/bin/zsh -c fuku run", "/bin/zsh -c fuku run"),
            ("/bin/zsh -c 'fuku run", "/bin/zsh -c 'fuku run"),
        ]

        for wrapped, expected in cases:
            with self.subTest(wrapped):
                self.assertEqual(expected, hosts.shell_text(wrapped))

    def test_complete_lines(self):
        cases = [
            ("finished without a newline", "a\nb", False, ["a", "b"]),
            ("cut mid-line", "a\nb", True, ["a"]),
            ("cut after a line", "a\nb\n", True, ["a", "b"]),
            ("cut before any output", "", True, []),
        ]

        for name, stdout, timed_out, expected in cases:
            with self.subTest(name):
                host = hosts.HostRun(None, stdout, "", 1.0, timed_out, False)

                self.assertEqual(expected, hosts.complete_lines(host))


class ClaudeAdapterTest(unittest.TestCase):
    def test_runs_the_host_headless(self):
        directory = fake_host(self, "claude", FAKE_CLAUDE, FAKE_MODE="ok", FAKE_SAMPLE=SAMPLES / "claude-ok.jsonl")
        env = {"PATH": f"{directory}:/usr/bin:/bin", "HOME": str(directory), "USER": "tester", "LOGNAME": "tester",
               "SHELL": "/bin/sh", "TMPDIR": tempfile.gettempdir(), "LANG": "C", "LC_ALL": "C", "TERM": "dumb",
               "OPENAI_API_KEY": "fake-openai", "ANTHROPIC_API_KEY": "fake-anthropic", "CLAUDECODE": "1",
               "CODEX_HOME": "/nowhere", "UNLISTED": "x"}
        with claude.ClaudeAdapter("<REPO>/plugins/agents") as adapter:
            result = adapter.run(directory, "Start the default profile.", env, budget={"usd": 1.5})

        argv = (directory / "argv").read_text(encoding="utf-8").splitlines()
        seen = (directory / "env").read_text()
        self.assertEqual(claude.command("Start the default profile.", "<REPO>/plugins/agents", 1.5)[1:], argv)
        self.assertEqual({"PATH", "HOME", "USER", "LOGNAME", "SHELL", "TMPDIR", "LANG", "LC_ALL", "TERM"},
                         env_names(directory))
        self.assertNotIn("fake-openai", seen)
        self.assertNotIn("fake-anthropic", seen)
        self.assertTrue(result.skill_loaded)
        self.assertIsNone(result.host_error)
        self.assertFalse(result.timed_out)
        self.assertEqual(3, result.totals["tool_calls"])

    def test_secrets_are_redacted(self):
        directory = fake_host(self, "claude", FAKE_CLAUDE, FAKE_MODE="echo", FAKE_SAMPLE=SAMPLES / "claude-ok.jsonl",
                              FAKE_RUN_TOKEN="run-token-abcdef12")
        env = {"PATH": f"{directory}:/usr/bin:/bin", "HOME": str(directory), "E2E_PASS_ENV": "MY_TOKEN",
               "MY_TOKEN": "opt-in-secret-123"}
        adapter = claude.ClaudeAdapter("<REPO>/plugins/agents")

        result = adapter.run(directory, "Start the default profile.", env, secrets=["run-token-abcdef12"])

        text = json.dumps([result.calls, result.host_error, result.totals])
        self.assertIn("MY_TOKEN", env_names(directory))
        self.assertEqual("echo <redacted> <redacted>; fuku doctor default --json", result.calls[1]["command"])
        self.assertNotIn("opt-in-secret-123", text)
        self.assertNotIn("run-token-abcdef12", text)

    def test_token_in_any_message_is_exposed(self):
        def final_answer(events):
            events[-1]["result"] = "The API token is run-token-abcdef12."

        def intermediate_message(events):
            text = {"type": "text", "text": "The override sets the token run-token-abcdef12."}
            events.insert(4, {"type": "assistant", "message": {"role": "assistant", "content": [text]}})

        cases = [
            ("final answer", final_answer, "The API token is <redacted>."),
            ("intermediate message", intermediate_message, json.loads(sample("claude-ok")[-1])["result"]),
        ]

        for name, change, answer in cases:
            with self.subTest(name):
                events = [json.loads(line) for line in sample("claude-ok")]
                change(events)
                directory = fake_host(self, "claude", FAKE_CLAUDE, FAKE_MODE="ok")
                (directory / "events.jsonl").write_text("\n".join(json.dumps(e) for e in events) + "\n",
                                                        encoding="utf-8")
                (directory / "fake.conf").write_text((directory / "fake.conf").read_text() +
                                                     f"FAKE_SAMPLE={shlex.quote(str(directory / 'events.jsonl'))}\n")
                env = {"PATH": f"{directory}:/usr/bin:/bin", "HOME": str(directory)}
                adapter = claude.ClaudeAdapter("<REPO>/plugins/agents")

                result = adapter.run(directory, "Start the default profile.", env, secrets=["run-token-abcdef12"])

                self.assertTrue(result.token_exposed)
                self.assertEqual(answer, result.answer)

    def test_login_read_by_source_is_blanked(self):
        events = [json.loads(line) for line in sample("claude-ok")]
        for event in events:
            for block in event.get("message", {}).get("content", []) if event["type"] != "result" else []:
                if block.get("type") == "tool_use" and block["id"] == "toolu_2":
                    block["name"], block["input"] = "Read", {"file_path": "/Users/someone/.claude/.credentials.json"}
                if block.get("type") == "tool_result" and block["tool_use_id"] == "toolu_2":
                    block["content"] = '{"claudeAiOauth": {"accessToken": "sk-ant-oat01-FAKE"}}'
        directory = fake_host(self, "claude", FAKE_CLAUDE, FAKE_MODE="ok")
        (directory / "events.jsonl").write_text("\n".join(json.dumps(e) for e in events) + "\n", encoding="utf-8")
        (directory / "fake.conf").write_text((directory / "fake.conf").read_text() +
                                             f"FAKE_SAMPLE={shlex.quote(str(directory / 'events.jsonl'))}\n")
        env = {"PATH": f"{directory}:/usr/bin:/bin", "HOME": str(directory)}
        adapter = claude.ClaudeAdapter("<REPO>/plugins/agents")

        result = adapter.run(directory, "Start the default profile.", env)

        self.assertTrue(result.credentials_read)
        self.assertEqual(("Read", "<redacted>"), (result.calls[1]["tool"], result.calls[1]["output"]))
        self.assertNotIn("sk-ant-oat01-FAKE", json.dumps(result.calls))

    def test_host_failure(self):
        cases = [
            ("fail", "crash", "claude exited 3: boom"),
            ("garbage", "unreadable", "unreadable events: line 1: not a Claude Code event"),
        ]

        for mode, kind, message in cases:
            with self.subTest(mode):
                directory = fake_host(self, "claude", FAKE_CLAUDE, FAKE_MODE=mode,
                                      FAKE_SAMPLE=SAMPLES / "claude-ok.jsonl")
                env = {"PATH": f"{directory}:/usr/bin:/bin", "HOME": str(directory)}
                adapter = claude.ClaudeAdapter("<REPO>/plugins/agents")

                result = adapter.run(directory, "Start the default profile.", env)

                self.assertEqual(kind, result.host_error_kind)
                self.assertIn(message, result.host_error)
                self.assertFalse(result.skill_loaded)

    def test_host_error_redacts_before_cutting(self):
        cases = [
            ("run token", "run-token-0123456789abcdef0123", {}),
            ("opted-in variable", "opt-in-secret-0123456789abcdef", {"E2E_PASS_ENV": "MY_TOKEN"}),
        ]

        for name, secret, extra in cases:
            with self.subTest(name):
                directory = fake_host(self, "claude", FAKE_CLAUDE, FAKE_MODE="stderr", FAKE_STDERR="x" * 280 + secret)
                env = {"PATH": f"{directory}:/usr/bin:/bin", "HOME": str(directory), "MY_TOKEN": secret, **extra}
                adapter = claude.ClaudeAdapter("<REPO>/plugins/agents")

                result = adapter.run(directory, "Start the default profile.", env,
                                     secrets=[] if extra else [secret])

                self.assertIn("claude exited 3: " + "x" * 280 + "<redacted>", result.host_error)
                self.assertNotIn(secret[:20], result.host_error)

    def test_deadline_kills_the_process_group(self):
        directory = fake_host(self, "claude", FAKE_CLAUDE, FAKE_MODE="sleep", FAKE_SAMPLE=SAMPLES / "claude-ok.jsonl")
        env = {"PATH": f"{directory}:/usr/bin:/bin", "HOME": str(directory)}
        adapter = claude.ClaudeAdapter("<REPO>/plugins/agents")

        result = adapter.run(directory, "Start the default profile.", env, timeout=1)

        child = int((directory / "child").read_text())
        self.assertTrue(result.timed_out)
        self.assertIsNone(result.host_error)
        self.assertLess(result.totals["seconds"], 10)
        self.assertTrue(gone(child))

    def test_child_that_ignores_sigterm_is_killed(self):
        directory = fake_host(self, "claude", FAKE_CLAUDE, FAKE_MODE="stubborn",
                              FAKE_SAMPLE=SAMPLES / "claude-ok.jsonl")
        env = {"PATH": f"{directory}:/usr/bin:/bin", "HOME": str(directory)}
        adapter = claude.ClaudeAdapter("<REPO>/plugins/agents")

        result = adapter.run(directory, "Start the default profile.", env)

        child = int((directory / "child").read_text())
        self.assertIsNone(result.host_error)
        self.assertTrue(gone(child))

    def test_surviving_group_is_a_host_error(self):
        directory = fake_host(self, "claude", FAKE_CLAUDE, FAKE_MODE="ok", FAKE_SAMPLE=SAMPLES / "claude-ok.jsonl")
        env = {"PATH": f"{directory}:/usr/bin:/bin", "HOME": str(directory)}
        adapter = claude.ClaudeAdapter("<REPO>/plugins/agents")

        with mock.patch.object(hosts.os, "killpg"):
            result = adapter.run(directory, "Start the default profile.", env)

        self.assertEqual("leftover", result.host_error_kind)
        self.assertEqual("host process left behind", result.host_error)


class CodexAdapterTest(unittest.TestCase):
    def test_session_model(self):
        cases = [
            ("recorded", {"sessions/2026/a.jsonl": '{"type": "session_meta"}\n'
                                                   '{"type": "turn_context", "payload": {"model": "gpt-x"}}\n'},
             "gpt-x"),
            ("unreadable turn context", {"sessions/a.jsonl": '{"type": "turn_context"\n'}, None),
            ("turn context without a model", {"sessions/a.jsonl": '{"type": "turn_context", "payload": {}}\n'}, None),
            ("no session", {}, None),
        ]

        for name, files, model in cases:
            with self.subTest(name), tempfile.TemporaryDirectory() as home:
                for relative, text in files.items():
                    path = pathlib.Path(home, relative)
                    path.parent.mkdir(parents=True, exist_ok=True)
                    path.write_text(text, encoding="utf-8")

                self.assertEqual(model, codex.session_model(home))

    def test_login_secrets(self):
        cases = [
            ("JSON", b'{"tokens": {"refresh_token": "r", "list": ["a", 1, null]}, "key": null}', ["r", "a"]),
            ("not JSON", b"KEY=abc def", ["KEY=abc", "def"]),
            ("not UTF-8", b"secret-word \xff", ["secret-word", "\ufffd"]),
        ]

        for name, data, secrets in cases:
            with self.subTest(name):
                self.assertEqual(secrets, codex.login_secrets(data))

    def test_auth_copy_is_removed(self):
        cases = [
            ("success", "ok", None),
            ("failure", "fail", None),
            ("timeout", "sleep", None),
            ("interrupt", "interrupt", KeyboardInterrupt),
        ]
        roots_before = set(glob.glob(os.path.join(tempfile.gettempdir(), "fuku-agents-codex-*")))

        for name, mode, raised in cases:
            with self.subTest(name):
                directory = fake_host(self, "codex", FAKE_CODEX, FAKE_MODE=mode, FAKE_SKILLS=PLUGIN / "skills",
                                      FAKE_SAMPLE=SAMPLES / "codex-ok.jsonl")
                (directory / ".codex").mkdir()
                (directory / ".codex" / "auth.json").write_text('{"fake": true}', encoding="utf-8")
                project = directory / "project"
                project.mkdir()
                env = {"PATH": f"{directory}:/usr/bin:/bin", "HOME": str(directory), "CODEX_HOME": "/nowhere"}
                outcome = {}

                with codex.CodexAdapter(runner.REPO, env) as adapter:
                    root = adapter.root
                    try:
                        outcome["result"] = adapter.run(project, "Start the default profile.", env, timeout=2)
                    except KeyboardInterrupt as err:
                        outcome["raised"] = type(err)
                    auth_left = list(root.rglob("auth.json"))

                self.assertEqual(raised, outcome.get("raised"))
                self.assertEqual([], auth_left)
                self.assertFalse(root.exists())
                self.assertEqual("-rw-------", (directory / "auth-mode").read_text().strip())
                self.assertEqual('{"fake": true}', (directory / ".codex" / "auth.json").read_text())
        self.assertEqual(roots_before, set(glob.glob(os.path.join(tempfile.gettempdir(), "fuku-agents-codex-*"))))

    def test_results(self):
        cases = [
            ("ok", {"skill_loaded": True, "host_error": None, "host_error_kind": None, "timed_out": False,
                    "model": "fake-model", "login_rotated": None, "credentials_read": False}),
            ("fail", {"skill_loaded": False, "host_error": "no turn.completed event; codex exited 2: boom",
                      "host_error_kind": "crash", "timed_out": False, "model": "fake-model", "login_rotated": None,
                      "credentials_read": False}),
            ("sleep", {"skill_loaded": False, "host_error": None, "host_error_kind": None, "timed_out": True,
                       "model": "fake-model", "login_rotated": None, "credentials_read": False}),
            ("rotate", {"skill_loaded": True, "host_error": None, "host_error_kind": None, "timed_out": False,
                        "model": "fake-model", "credentials_read": False,
                        "login_rotated": "Codex refreshed its login during the run; your real login may need "
                                         "`codex login`"}),
        ]

        for mode, expected in cases:
            with self.subTest(mode):
                directory = fake_host(self, "codex", FAKE_CODEX, FAKE_MODE=mode, FAKE_SKILLS=PLUGIN / "skills",
                                      FAKE_SAMPLE=SAMPLES / "codex-ok.jsonl")
                (directory / ".codex").mkdir()
                (directory / ".codex" / "auth.json").write_text("{}", encoding="utf-8")
                project = directory / "project"
                project.mkdir()
                env = {"PATH": f"{directory}:/usr/bin:/bin", "HOME": str(directory)}

                with codex.CodexAdapter(runner.REPO, env) as adapter:
                    result = adapter.run(project, "Start the default profile.", env, timeout=2)

                argv = (directory / "argv").read_text(encoding="utf-8").splitlines()
                actual = {"skill_loaded": result.skill_loaded, "host_error": result.host_error,
                          "host_error_kind": result.host_error_kind, "timed_out": result.timed_out,
                          "model": result.totals["model"], "login_rotated": result.login_rotated,
                          "credentials_read": result.credentials_read}
                self.assertEqual(expected, actual)
                self.assertEqual("codex-cli 0.0.0-fake", result.totals["host_version"])
                self.assertEqual(codex.exec_command(project, "Start the default profile.")[1:], argv[-12:])
                self.assertEqual("{}", (directory / ".codex" / "auth.json").read_text())

    def test_unreadable_events_fail_the_run(self):
        cases = [
            ("garbage", "unreadable", "unreadable events: line 1: not a Codex event: 'not an event'"),
            ("garbage_fail", "crash",
             "unreadable events: line 1: not a Codex event: 'not an event'; codex exited 2: boom"),
        ]

        for mode, kind, message in cases:
            with self.subTest(mode):
                directory = fake_host(self, "codex", FAKE_CODEX, FAKE_MODE=mode, FAKE_SKILLS=PLUGIN / "skills")
                (directory / ".codex").mkdir()
                (directory / ".codex" / "auth.json").write_text("{}", encoding="utf-8")
                project = directory / "project"
                project.mkdir()
                env = {"PATH": f"{directory}:/usr/bin:/bin", "HOME": str(directory)}

                with codex.CodexAdapter(runner.REPO, env) as adapter:
                    result = adapter.run(project, "Start the default profile.", env, timeout=2)

                self.assertEqual((kind, message), (result.host_error_kind, result.host_error))

    def test_host_error_redacts_before_cutting(self):
        cases = [
            ("run token", "run-token-0123456789abcdef0123", {}),
            ("login", "FAKE-REFRESH-SECRET-0123456789", {}),
            ("opted-in variable", "opt-in-secret-0123456789abcdef", {"E2E_PASS_ENV": "MY_TOKEN"}),
        ]

        for name, secret, extra in cases:
            with self.subTest(name):
                directory = fake_host(self, "codex", FAKE_CODEX, FAKE_MODE="stderr", FAKE_SKILLS=PLUGIN / "skills",
                                      FAKE_STDERR="x" * 280 + secret)
                (directory / ".codex").mkdir()
                login = '{"tokens": {"refresh_token": "FAKE-REFRESH-SECRET-0123456789"}}'
                (directory / ".codex" / "auth.json").write_text(login, encoding="utf-8")
                project = directory / "project"
                project.mkdir()
                env = {"PATH": f"{directory}:/usr/bin:/bin", "HOME": str(directory), "MY_TOKEN": secret, **extra}

                with codex.CodexAdapter(runner.REPO, env) as adapter:
                    result = adapter.run(project, "Start the default profile.", env, timeout=2,
                                         secrets=["run-token-0123456789abcdef0123"])

                self.assertIn("codex exited 2: " + "x" * 280 + "<redacted>", result.host_error)
                self.assertNotIn(secret[:20], result.host_error)

    def test_missing_login(self):
        directory = fake_host(self, "codex", FAKE_CODEX, FAKE_MODE="ok", FAKE_SKILLS=PLUGIN / "skills",
                              FAKE_SAMPLE=SAMPLES / "codex-ok.jsonl")
        project = directory / "project"
        project.mkdir()
        env = {"PATH": f"{directory}:/usr/bin:/bin", "HOME": str(directory)}

        with codex.CodexAdapter(runner.REPO, env) as adapter:
            result = adapter.run(project, "Start the default profile.", env, timeout=2)

        self.assertEqual(("login", f"Codex is not logged in: no {directory}/.codex/auth.json"),
                         (result.host_error_kind, result.host_error))
        self.assertFalse((directory / "env").exists())

    def test_auth_copy_left_is_an_error(self):
        directory = fake_host(self, "codex", FAKE_CODEX, FAKE_MODE="stash", FAKE_SKILLS=PLUGIN / "skills",
                              FAKE_SAMPLE=SAMPLES / "codex-ok.jsonl")
        (directory / ".codex").mkdir()
        (directory / ".codex" / "auth.json").write_text("{}", encoding="utf-8")
        project = directory / "project"
        project.mkdir()
        env = {"PATH": f"{directory}:/usr/bin:/bin", "HOME": str(directory)}

        with codex.CodexAdapter(runner.REPO, env) as adapter:
            root = adapter.root
            with self.assertRaises(codex.AdapterError) as raised:
                adapter.run(project, "Start the default profile.", env, timeout=2)

        self.assertEqual(f"an auth copy is left: {root}/stash/auth.json", str(raised.exception))
        self.assertFalse(root.exists())

    def test_environment_is_an_allowlist(self):
        directory = fake_host(self, "codex", FAKE_CODEX, FAKE_MODE="ok", FAKE_SKILLS=PLUGIN / "skills",
                              FAKE_SAMPLE=SAMPLES / "codex-ok.jsonl")
        (directory / ".codex").mkdir()
        (directory / ".codex" / "auth.json").write_text("{}", encoding="utf-8")
        project = directory / "project"
        project.mkdir()
        env = {"PATH": f"{directory}:/usr/bin:/bin", "HOME": str(directory), "USER": "tester", "LANG": "C",
               "OPENAI_API_KEY": "fake-openai", "ANTHROPIC_API_KEY": "fake-anthropic", "CLAUDECODE": "1",
               "CODEX_HOME": "/nowhere", "UNLISTED": "x"}

        with codex.CodexAdapter(runner.REPO, env) as adapter:
            adapter.run(project, "Start the default profile.", env, timeout=2)

        seen = (directory / "env").read_text()
        self.assertEqual({"PATH", "HOME", "USER", "LANG", "CODEX_HOME"}, env_names(directory))
        self.assertNotIn("fake-openai", seen)
        self.assertNotIn("fake-anthropic", seen)
        self.assertNotIn("/nowhere", seen)

    def test_login_is_redacted(self):
        directory = fake_host(self, "codex", FAKE_CODEX, FAKE_MODE="leak", FAKE_SKILLS=PLUGIN / "skills",
                              FAKE_SAMPLE=SAMPLES / "codex-ok.jsonl")
        (directory / ".codex").mkdir()
        login = '{"tokens": {"refresh_token": "FAKE-REFRESH-SECRET", "id": "short"}, "OPENAI_API_KEY": null}'
        (directory / ".codex" / "auth.json").write_text(login, encoding="utf-8")
        project = directory / "project"
        project.mkdir()
        env = {"PATH": f"{directory}:/usr/bin:/bin", "HOME": str(directory)}

        with codex.CodexAdapter(runner.REPO, env) as adapter:
            result = adapter.run(project, "Start the default profile.", env, timeout=2)

        text = json.dumps([result.calls, result.host_error, result.totals])
        self.assertTrue(result.credentials_read)
        self.assertNotIn("FAKE-REFRESH-SECRET", text)
        self.assertIn('"refresh_token": "<redacted>"', result.calls[0]["output"])
        self.assertIn("short", result.calls[0]["output"])

    def test_secret_in_any_message_is_found(self):
        answer = json.loads(sample("codex-ok")[-2])["item"]["text"]
        cases = [
            ("login in the final answer", -2, "Your refresh token is FAKE-REFRESH-SECRET-0123456789.",
             {"credentials_read": True, "token_exposed": False}, "Your refresh token is <redacted>."),
            ("login in an intermediate message", 2, "The refresh token is FAKE-REFRESH-SECRET-0123456789.",
             {"credentials_read": True, "token_exposed": False}, answer),
            ("token in an intermediate message", 2, "The override sets the token run-token-abcdef12.",
             {"credentials_read": False, "token_exposed": True}, answer),
        ]

        for name, index, text, flags, final in cases:
            with self.subTest(name):
                directory = fake_host(self, "codex", FAKE_CODEX, FAKE_MODE="ok", FAKE_SKILLS=PLUGIN / "skills")
                events = [json.loads(line) for line in sample("codex-ok")]
                events[index]["item"]["text"] = text
                (directory / "events.jsonl").write_text("\n".join(json.dumps(e) for e in events) + "\n",
                                                        encoding="utf-8")
                (directory / "fake.conf").write_text((directory / "fake.conf").read_text() +
                                                     f"FAKE_SAMPLE={shlex.quote(str(directory / 'events.jsonl'))}\n")
                (directory / ".codex").mkdir()
                login = '{"tokens": {"refresh_token": "FAKE-REFRESH-SECRET-0123456789"}}'
                (directory / ".codex" / "auth.json").write_text(login, encoding="utf-8")
                project = directory / "project"
                project.mkdir()
                env = {"PATH": f"{directory}:/usr/bin:/bin", "HOME": str(directory)}

                with codex.CodexAdapter(runner.REPO, env) as adapter:
                    result = adapter.run(project, "Start the default profile.", env, timeout=2,
                                         secrets=["run-token-abcdef12"])

                self.assertEqual(flags, {"credentials_read": result.credentials_read,
                                         "token_exposed": result.token_exposed})
                self.assertEqual(final, result.answer)

    def test_login_read_by_source_is_blanked(self):
        cases = [
            ("split", {}, ["<redacted>", "<redacted>"], True),
            ("base64", {}, ["<redacted>"], True),
            ("command", {"FAKE_COMMAND": "cat $CODEX_HOME/auth.json"}, ["<redacted>"], True),
            ("command", {"FAKE_COMMAND": "ls $CODEX_HOME"}, ["<redacted>"], True),
            ("command", {"FAKE_COMMAND": "$CODEX_HOME/plugins/cache/fuku/fuku/0.1.0/skills/fuku/scripts/api.sh GET "
                                         "http://127.0.0.1:9/api/v1/services"}, ["visible output"], False),
            ("command", {"FAKE_COMMAND": 'ls "${CODEX_HOME}/plugins/cache/fuku"'}, ["visible output"], False),
        ]

        for mode, conf, outputs, flagged in cases:
            with self.subTest(conf.get("FAKE_COMMAND", mode)):
                directory = fake_host(self, "codex", FAKE_CODEX, FAKE_MODE=mode, FAKE_SKILLS=PLUGIN / "skills",
                                      FAKE_SAMPLE=SAMPLES / "codex-ok.jsonl", **conf)
                (directory / ".codex").mkdir()
                login = '{"tokens": {"refresh_token": "FAKE-REFRESH-SECRET-0123456789"}, "OPENAI_API_KEY": null}'
                (directory / ".codex" / "auth.json").write_text(login, encoding="utf-8")
                project = directory / "project"
                project.mkdir()
                env = {"PATH": f"{directory}:/usr/bin:/bin", "HOME": str(directory)}

                with codex.CodexAdapter(runner.REPO, env) as adapter:
                    result = adapter.run(project, "Start the default profile.", env, timeout=2)

                self.assertEqual(flagged, result.credentials_read)
                self.assertEqual(outputs, [c["output"] for c in result.calls])

    def test_deadline_kills_the_process_group(self):
        directory = fake_host(self, "codex", FAKE_CODEX, FAKE_MODE="sleep", FAKE_SKILLS=PLUGIN / "skills",
                              FAKE_SAMPLE=SAMPLES / "codex-ok.jsonl")
        (directory / ".codex").mkdir()
        (directory / ".codex" / "auth.json").write_text("{}", encoding="utf-8")
        project = directory / "project"
        project.mkdir()
        env = {"PATH": f"{directory}:/usr/bin:/bin", "HOME": str(directory)}

        with codex.CodexAdapter(runner.REPO, env) as adapter:
            result = adapter.run(project, "Start the default profile.", env, timeout=1)

        child = int((directory / "child").read_text())
        self.assertTrue(result.timed_out)
        self.assertLess(result.totals["seconds"], 10)
        self.assertTrue(gone(child))

    def test_installed_skill_must_match_the_tree(self):
        cases = [
            ("edit", "fuku/SKILL.md"),
            ("extra", "fuku/EXTRA.md"),
            ("missing", "fuku/references/control-api.md"),
            ("mode", "fuku/scripts/api.sh"),
        ]
        roots_before = set(glob.glob(os.path.join(tempfile.gettempdir(), "fuku-agents-codex-*")))

        for tamper, path in cases:
            with self.subTest(tamper):
                directory = fake_host(self, "codex", FAKE_CODEX, FAKE_SKILLS=PLUGIN / "skills", FAKE_TAMPER=tamper)
                env = {"PATH": f"{directory}:/usr/bin:/bin", "HOME": str(directory)}

                with self.assertRaises(codex.AdapterError) as raised:
                    with codex.CodexAdapter(runner.REPO, env):
                        pass

                self.assertEqual(f"the installed skill differs from the working tree: {path}", str(raised.exception))
        self.assertEqual(roots_before, set(glob.glob(os.path.join(tempfile.gettempdir(), "fuku-agents-codex-*"))))

    def test_install_failure(self):
        cases = [
            ("no codex on PATH", "", "codex --version: "),
            ("plugin add refused", "refuse", "codex plugin add fuku@fuku --json exited 1: no such plugin"),
            ("no installed path", "garbled", "codex plugin add printed no installedPath: "),
            ("installed outside the home", "outside", "the plugin was installed outside the private home: "),
        ]
        roots_before = set(glob.glob(os.path.join(tempfile.gettempdir(), "fuku-agents-codex-*")))

        for name, tamper, message in cases:
            with self.subTest(name):
                directory = fake_host(self, "codex", FAKE_CODEX, FAKE_SKILLS=PLUGIN / "skills", FAKE_TAMPER=tamper)
                (directory / "empty").mkdir()
                path = f"{directory}:/usr/bin:/bin" if tamper else str(directory / "empty")
                env = {"PATH": path, "HOME": str(directory)}

                with self.assertRaises(codex.AdapterError) as raised:
                    with codex.CodexAdapter(runner.REPO, env):
                        pass

                self.assertTrue(str(raised.exception).startswith(message), str(raised.exception))
        self.assertEqual(roots_before, set(glob.glob(os.path.join(tempfile.gettempdir(), "fuku-agents-codex-*"))))

    def test_surviving_group_is_a_host_error(self):
        directory = fake_host(self, "codex", FAKE_CODEX, FAKE_MODE="ok", FAKE_SKILLS=PLUGIN / "skills",
                              FAKE_SAMPLE=SAMPLES / "codex-ok.jsonl")
        (directory / ".codex").mkdir()
        (directory / ".codex" / "auth.json").write_text("{}", encoding="utf-8")
        project = directory / "project"
        project.mkdir()
        env = {"PATH": f"{directory}:/usr/bin:/bin", "HOME": str(directory)}

        with codex.CodexAdapter(runner.REPO, env) as adapter, mock.patch.object(hosts.os, "killpg"):
            result = adapter.run(project, "Start the default profile.", env, timeout=2)

        self.assertEqual(("leftover", "host process left behind"), (result.host_error_kind, result.host_error))


if __name__ == "__main__":
    unittest.main()
