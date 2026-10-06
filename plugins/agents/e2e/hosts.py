"""What the host adapters share: the result shape, the host environment, redaction and the host process"""

import contextlib
import dataclasses
import os
import re
import shlex
import signal
import subprocess
import tempfile
import time

GRACE_SECONDS = 3
PROBE_SECONDS = 1
SHELL_WRAPPER = re.compile(r"^/bin/(?:ba|z)?sh -l?c (.+)$", re.S)
BASE_ENV = {"HOME", "USER", "LOGNAME", "SHELL", "TMPDIR", "LANG", "TERM"}
PASS_ENV = "E2E_PASS_ENV"
MIN_SECRET = 8
REDACTED = "<redacted>"


class ParseError(Exception):
    """A host event the adapter cannot read, with the line it could not read"""

    def __init__(self, message, line=""):
        super().__init__(message)
        self.line = line

    def describe(self, values):
        """Returns the message with the line, its secrets removed before the line is cut to 120 characters"""
        if not self.line:
            return str(self)

        return f"{self}: {scrub(self.line, secret_values(values))[:120]!r}"


@dataclasses.dataclass
class AgentResult:
    """What a host adapter returns: the tool calls, the skill load, the totals, a host failure and the final answer"""

    calls: list = dataclasses.field(default_factory=list)
    totals: dict = dataclasses.field(default_factory=dict)
    skill_loaded: bool = False
    host_error: object = None
    host_error_kind: object = None
    timed_out: bool = False
    credentials_read: bool = False
    token_exposed: bool = False
    login_rotated: object = None
    answer: str = ""

    def fail(self, kind, message):
        """Records a host failure, keeping an earlier message and letting a leftover process win the kind"""
        self.host_error = "; ".join(filter(None, [self.host_error, message]))
        if self.host_error_kind is None or kind == "leftover":
            self.host_error_kind = kind


@dataclasses.dataclass
class HostRun:
    """One finished host process: exit code, stdout, stderr, wall time, deadline hit and a group member left behind"""

    returncode: object
    stdout: str
    stderr: str
    seconds: float
    timed_out: bool
    left_behind: bool


def host_env(env, own):
    """Returns a host environment from an allowlist plus the names E2E_PASS_ENV opts in, and their values"""
    passed = [name for name in env.get(PASS_ENV, "").split(",") if name]
    keep = {k: v for k, v in env.items() if k in BASE_ENV or k.startswith("LC_") or k in passed}
    keep.update(own)
    keep["PATH"] = env["PATH"]

    return keep, [env[name] for name in passed if name in env]


def secret_values(values):
    """Returns the distinct secrets long enough to redact, longest first so a prefix never hides a longer one"""
    return sorted({v for v in values if isinstance(v, str) and len(v) >= MIN_SECRET}, key=len, reverse=True)


def scrub(text, secrets):
    """Replaces every secret in a text with a placeholder"""
    if not isinstance(text, str):
        return text
    for secret in secrets:
        text = text.replace(secret, REDACTED)

    return text


def redact(result, values, transcript=""):
    """Removes every secret from what a result carries, and reports whether one was there or in the raw transcript"""
    secrets = secret_values(values)
    texts = [c[k] for c in result.calls for k in ("command", "output")] + [result.answer, transcript]
    found = any(s in t for t in texts for s in secrets)
    for call in result.calls:
        call["command"], call["output"] = scrub(call["command"], secrets), scrub(call["output"], secrets)
    result.host_error = scrub(result.host_error, secrets)
    result.answer = scrub(result.answer, secrets)
    result.totals = {k: scrub(v, secrets) for k, v in result.totals.items()}

    return found


def mark_credential_reads(result, patterns):
    """Blanks the whole output of every call whose command names a login source, and reports whether one did"""
    found = False
    for call in result.calls:
        if any(re.search(pattern, call["command"]) for pattern in patterns):
            call["output"] = REDACTED
            found = True

    return found


def shell_text(command):
    """Returns the command a shell wrapper such as `/bin/zsh -lc '…'` runs, or the command unchanged"""
    match = SHELL_WRAPPER.match(command)
    if not match:
        return command
    try:
        words = shlex.split(match.group(1))
    except ValueError:
        return command

    return words[0] if len(words) == 1 else command


def complete_lines(host):
    """Returns the stdout lines of a host run, without the last one when a deadline cut it off mid-line"""
    lines = host.stdout.splitlines()
    if host.timed_out and lines and not host.stdout.endswith("\n"):
        lines.pop()

    return lines


def group_alive(pgid):
    """Reports whether a process group still has a live member, allowing a moment for killed members to go"""
    deadline = time.monotonic() + PROBE_SECONDS
    while True:
        try:
            os.killpg(pgid, 0)
        except (ProcessLookupError, PermissionError):
            return False
        if time.monotonic() >= deadline:
            return True
        time.sleep(0.05)


def stop_group(process):
    """Sends SIGTERM to the host's group, waits for the host, sends SIGKILL to the rest and reports a survivor"""
    # no-op: macOS answers EPERM for a group whose members are all exiting
    with contextlib.suppress(ProcessLookupError, PermissionError):
        os.killpg(process.pid, signal.SIGTERM)
    with contextlib.suppress(subprocess.TimeoutExpired):
        process.wait(timeout=GRACE_SECONDS)
    with contextlib.suppress(ProcessLookupError, PermissionError):
        os.killpg(process.pid, signal.SIGKILL)
    process.wait()

    # ponytail: a child that calls setsid leaves the group; only the teardown's project scan reaches it
    return group_alive(process.pid)


def run_host(args, cwd, env, timeout):
    """Runs a host in its own process group, kills the group on the deadline or an interrupt, and reports a survivor"""
    with tempfile.TemporaryFile("w+") as stdout, tempfile.TemporaryFile("w+") as stderr:
        started = time.monotonic()
        process = subprocess.Popen(args, cwd=cwd, env=env, stdin=subprocess.DEVNULL, stdout=stdout, stderr=stderr,
                                   text=True, start_new_session=True)
        timed_out, left_behind = False, False
        try:
            process.wait(timeout=timeout)
        except subprocess.TimeoutExpired:
            timed_out = True
        finally:
            left_behind = stop_group(process)
        seconds = round(time.monotonic() - started, 1)
        stdout.seek(0)
        stderr.seek(0)

        return HostRun(None if timed_out else process.returncode, stdout.read(), stderr.read(), seconds, timed_out,
                       left_behind)
