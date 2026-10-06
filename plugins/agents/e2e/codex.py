"""Codex adapter: a private CODEX_HOME with the working-tree plugin, and the `codex exec --json` events"""

import hashlib
import json
import os
import pathlib
import re
import shutil
import subprocess
import tempfile

import checks
import hosts

EXECUTABLE = "codex"
INSTALL_SECONDS = 60
EVENTS = {"thread.started", "turn.started", "turn.completed", "turn.failed", "item.started", "item.updated",
          "item.completed", "error"}
TOOL_ITEMS = {"file_change", "mcp_tool_call", "web_search"}
QUIET_ITEMS = {"agent_message", "reasoning", "todo_list", "error"}
SKILL_READERS = checks.READERS | checks.BOUNDED | checks.SCRIPTED
LINE_NUMBER = re.compile(r"^\s*(?:\d+[\t:]\s*)?")
SKILL_FILE = pathlib.Path(__file__).resolve().parents[1] / "skills" / "fuku" / "SKILL.md"


class AdapterError(Exception):
    """A Codex home the adapter could not build, or an auth copy it could not remove"""


def exec_command(project, prompt):
    """Returns the headless command line: the project as the workspace, sandbox network on, no login shell"""
    return [
        EXECUTABLE, "exec", "--json", "--skip-git-repo-check", "-C", str(project),
        "-s", "workspace-write",
        "-c", "sandbox_workspace_write.network_access=true",
        "-c", "allow_login_shell=false",
        prompt,
    ]


def parse(lines, codex_home):
    """Reads `codex exec --json` lines into the normalised result; the skill counts when its SKILL.md was read"""
    skill_path = re.compile(re.escape(str(codex_home)) + r"/plugins/cache/fuku/[^\s'\"]*/skills/fuku/SKILL\.md")
    calls, started, usage, turns, failure, errors, answer = [], {}, {}, 0, None, [], ""
    for number, line in enumerate(lines, 1):
        if not line.strip():
            continue
        try:
            event = json.loads(line)
            kind = event["type"]
        except (json.JSONDecodeError, TypeError, KeyError) as err:
            raise hosts.ParseError(f"line {number}: not a Codex event", line) from err
        if kind not in EVENTS:
            raise hosts.ParseError(f"line {number}: unknown Codex event type '{kind}'")
        if kind == "turn.completed":
            turns += 1
            for key, value in (event.get("usage") or {}).items():
                usage[key] = usage.get(key, 0) + value
        if kind == "turn.failed":
            failure = (event.get("error") or {}).get("message", "turn failed")
        if kind == "error":
            errors.append(event.get("message", "error"))
        if kind not in ("item.started", "item.completed"):
            continue
        item = event.get("item") or {}
        item_type = item.get("type")
        if item_type == "agent_message" and kind == "item.completed":
            answer = item.get("text", "")
        if item_type == "command_execution":
            call = {
                "tool": "shell",
                "command": hosts.shell_text(item.get("command", "")),
                "output": item.get("aggregated_output", ""),
                "error": item.get("exit_code") not in (0, None) or item.get("status") == "failed",
            }
        elif item_type in TOOL_ITEMS:
            details = {k: v for k, v in item.items() if k not in ("id", "type")}
            call = {"tool": item_type, "command": json.dumps(details, sort_keys=True), "output": "",
                    "error": item.get("status") == "failed"}
        elif item_type in QUIET_ITEMS:
            continue
        else:
            raise hosts.ParseError(f"line {number}: unknown Codex item type '{item_type}'")
        if item.get("id") in started:
            started.pop(item.get("id")).update(call)
        else:
            calls.append(call)
        if kind == "item.started":
            started[item.get("id")] = call

    kind = "api" if failure else None
    if failure is None and turns == 0:
        kind, failure = "crash", errors[-1] if errors else "no turn.completed event"
    totals = {
        "tool_calls": len(calls),
        "turns": turns,
        "input_tokens": usage.get("input_tokens", 0),
        "cached_input_tokens": usage.get("cached_input_tokens", 0),
        "output_tokens": usage.get("output_tokens", 0),
        "usd": None,
        "model": None,
        "host_version": None,
    }
    loaded = any(read_skill(c, skill_path) for c in calls)

    result = hosts.AgentResult(calls, totals, loaded, answer=answer)
    if kind:
        result.fail(kind, failure)

    return result


def read_skill(call, skill_path):
    """Reports whether a successful shell call ran a reader on the installed SKILL.md and printed its every line"""
    printed = printed_lines(call["output"]) >= printed_lines(SKILL_FILE.read_text(encoding="utf-8"))
    if call["tool"] != "shell" or call["error"] or not printed:
        return False

    return bool(skill_path.search(call["command"])) and any(argv[0] in SKILL_READERS
                                                            for argv in checks.commands(call["command"]))


def printed_lines(text):
    """Returns the non-blank lines of a text without a line-number prefix, indentation or trailing whitespace"""
    return {LINE_NUMBER.sub("", line).rstrip() for line in text.splitlines()} - {""}


def session_model(codex_home):
    """Returns the model of the session Codex recorded under its home, or None"""
    for path in sorted(pathlib.Path(codex_home).glob("sessions/**/*.jsonl")):
        for line in path.read_text(encoding="utf-8", errors="replace").splitlines():
            if '"turn_context"' not in line:
                continue
            try:
                return json.loads(line)["payload"]["model"]
            except (json.JSONDecodeError, KeyError, TypeError):
                return None

    return None


def tree_files(root):
    """Returns {relative path: (bytes, executable bits)} of every file under a directory"""
    root = pathlib.Path(root)

    files = (p for p in root.rglob("*") if p.is_file())

    return {str(p.relative_to(root)): (p.read_bytes(), p.stat().st_mode & 0o111) for p in files}


def tree_diff(expected, actual):
    """Returns the relative paths whose presence, bytes or executable bits differ between two directories"""
    left, right = tree_files(expected), tree_files(actual)

    return sorted(path for path in set(left) | set(right) if left.get(path) != right.get(path))


def codex(args, env):
    """Runs one setup command of the Codex CLI, bounded and without stdin, and returns its stdout"""
    try:
        done = subprocess.run([EXECUTABLE, *args], env=env, stdin=subprocess.DEVNULL, capture_output=True, text=True,
                              timeout=INSTALL_SECONDS, check=False)
    except (OSError, subprocess.TimeoutExpired) as err:
        raise AdapterError(f"codex {' '.join(args)}: {err}") from err
    if done.returncode != 0:
        raise AdapterError(f"codex {' '.join(args)} exited {done.returncode}: {done.stderr.strip()[:300]}")

    return done.stdout


def install(repo, home, environ):
    """Builds a Codex home with the fuku plugin from the working tree and proves its skill equals the tree's"""
    home.mkdir()
    env, _ = hosts.host_env(environ, {"CODEX_HOME": str(home)})
    version = codex(["--version"], env).strip()
    codex(["plugin", "marketplace", "add", str(repo)], env)
    try:
        installed = pathlib.Path(json.loads(codex(["plugin", "add", "fuku@fuku", "--json"], env))["installedPath"])
    except (json.JSONDecodeError, KeyError, TypeError) as err:
        raise AdapterError(f"codex plugin add printed no installedPath: {err}") from err
    if not installed.resolve().is_relative_to(home.resolve()):
        raise AdapterError(f"the plugin was installed outside the private home: {installed}")
    differ = tree_diff(repo / "plugins" / "agents" / "skills", installed / "skills")
    if differ:
        raise AdapterError(f"the installed skill differs from the working tree: {', '.join(differ)}")

    return version


def leaves(value):
    """Returns every string leaf of a parsed JSON value"""
    if isinstance(value, dict):
        return [leaf for item in value.values() for leaf in leaves(item)]
    if isinstance(value, list):
        return [leaf for item in value for leaf in leaves(item)]

    return [value] if isinstance(value, str) else []


def login_secrets(data):
    """Returns the string leaves of a Codex login file, or its whitespace-separated words when it is not JSON"""
    try:
        return leaves(json.loads(data))
    except (json.JSONDecodeError, UnicodeDecodeError):
        return data.decode("utf-8", errors="replace").split()


def copy_auth(source, target):
    """Copies the Codex login into a run's home with mode 600, never showing its contents"""
    data = source.read_bytes()
    descriptor = os.open(target, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(descriptor, "wb") as handle:
        handle.write(data)


class CodexAdapter:
    """Runs one scenario prompt in Codex with a private home per run, built from one template per sweep"""

    name = "codex"

    def __init__(self, repo, environ=None):
        self.repo = pathlib.Path(repo)
        self.environ = dict(os.environ if environ is None else environ)
        self.root = None
        self.version = None
        self.runs = 0

    def __enter__(self):
        self.root = pathlib.Path(tempfile.mkdtemp(prefix="fuku-agents-codex-")).resolve()
        try:
            self.version = install(self.repo, self.root / "template", self.environ)
        except BaseException:
            self.close()
            raise
        return self

    def __exit__(self, *_):
        self.close()
        return False

    def close(self):
        """Removes the sweep's Codex homes"""
        if self.root is not None:
            shutil.rmtree(self.root, ignore_errors=True)
            self.root = None

    def run(self, project, prompt, env, budget=None, timeout=None, secrets=()):
        self.runs += 1
        home = self.root / f"run-{self.runs}"
        copy = home / "auth.json"
        source = pathlib.Path(self.environ.get("HOME", "~")).expanduser() / ".codex" / "auth.json"
        login, after = None, None
        try:
            shutil.copytree(self.root / "template", home, symlinks=True)
            if not source.is_file():
                result = hosts.AgentResult()
                result.fail("login", f"Codex is not logged in: no {source}")
                return result
            login = source.read_bytes()
            copy_auth(source, copy)
            run_env, passed = hosts.host_env(env, {"CODEX_HOME": str(home)})
            host = hosts.run_host(exec_command(project, prompt), project, run_env, timeout)
            model = session_model(home)
        finally:
            if copy.is_file():
                after = copy.read_bytes()
            copy.unlink(missing_ok=True)
            shutil.rmtree(home, ignore_errors=True)
            left = sorted(str(p) for p in self.root.rglob("auth.json"))
            if left:
                raise AdapterError(f"an auth copy is left: {', '.join(left)}")

        credentials = login_secrets(login) + login_secrets(after or b"")
        try:
            result = parse(hosts.complete_lines(host), home)
        except hosts.ParseError as err:
            result = hosts.AgentResult()
            result.fail("unreadable", f"unreadable events: {err.describe([*credentials, *secrets, *passed])}")
        if host.timed_out:
            result.host_error, result.host_error_kind, result.timed_out = None, None, True
        if host.returncode not in (0, None):
            if result.host_error_kind == "unreadable":
                result.host_error_kind = None
            stderr = hosts.scrub(host.stderr.strip(), hosts.secret_values([*credentials, *secrets, *passed]))
            result.fail("crash", f"codex exited {host.returncode}: {stderr[:300]}")
        if host.left_behind:
            result.fail("leftover", "host process left behind")
        if after is None or hashlib.sha256(after).digest() != hashlib.sha256(login).digest():
            result.login_rotated = "Codex refreshed its login during the run; your real login may need `codex login`"
        result.totals.update(seconds=host.seconds, model=model, host_version=self.version)
        sources = [r"auth\.json", r"CODEX_HOME(?!\}?/plugins(/|\b))", re.escape(str(home)) + r"(?!/plugins(/|\b))"]
        by_source = hosts.mark_credential_reads(result, sources)
        result.credentials_read = hosts.redact(result, credentials, host.stdout) or by_source
        result.token_exposed = hosts.redact(result, secrets, host.stdout)
        hosts.redact(result, passed)

        return result
