"""End-to-end runner of the fuku agent skill: scenarios, the temp project, setup, end-state checks and teardown"""

import contextlib
import dataclasses
import hashlib
import json
import os
import pathlib
import re
import secrets
import shutil
import signal
import socket
import subprocess
import sys
import tempfile
import time

HERE = pathlib.Path(__file__).resolve().parent
REPO = HERE.parents[2]
PLUGIN = REPO / "plugins" / "agents"

sys.path.insert(0, str(HERE))

import checks  # noqa: E402
import claude  # noqa: E402
import codex  # noqa: E402
import hosts  # noqa: E402
import report  # noqa: E402
from hosts import AgentResult  # noqa: E402
SCENARIOS = HERE / "scenarios"
FIXTURE = HERE / "fixture"
OVERRIDES = FIXTURE / "overrides"
SERVICES = sorted(p.name for p in FIXTURE.iterdir() if p.is_dir() and p != OVERRIDES)


def fixture_profiles():
    """Returns the profile names of the fixture config, read from its top-level profiles block"""
    names, inside = [], False
    for line in (FIXTURE / "fuku.yaml").read_text(encoding="utf-8").splitlines():
        if line and not line.startswith(" "):
            inside = line == "profiles:"
            continue
        match = re.match(r"  ([\w-]+):", line)
        if inside and match:
            names.append(match.group(1))

    return sorted(names)


PROFILES = fixture_profiles()
SERVICE_PROGRAMS = re.findall(r"\bexec (.+)$", (FIXTURE / "fuku.yaml").read_text(encoding="utf-8"), re.M)

KEYS = {"prompt", "profile", "setup", "checks", "budget"}
BUDGET = {"tool_calls", "input_tokens", "seconds", "usd"}
SETUP = {"start_fuku": False, "override": "template", "wait_service_exited": "service", "edit_service_command": "service"}
STATE_CHECKS = {
    "fuku_running": False,
    "services_running": False,
    "same_fuku_pid": False,
    "same_service_pids": False,
    "fuku_pid_changed": False,
    "served_by_built_fuku": False,
    "service_pid_changed": "service",
    "other_service_pids_same": "service",
    "service_command_is": "service",
    "service_stopped": "service",
}
COMMAND_CHECKS = checks.SCENARIO_CHECKS

SOCKET_DIR = pathlib.Path("/tmp")
WAIT_SECONDS = 15
STOP_SECONDS = 30
PROBE_SECONDS = 10
GRACE_SECONDS = 3


class UsageError(Exception):
    """A missing fuku binary, a scenario file or a temp dir the runner refuses, reported with exit code 2"""


class SetupError(Exception):
    """A setup step that did not reach its state"""


class ProbeError(Exception):
    """A process probe that failed or did not answer in time"""


@dataclasses.dataclass
class Snapshot:
    """The fuku serving the project and the PID of each running service, None when absent"""

    fuku_pid: object = None
    services: dict = dataclasses.field(default_factory=dict)


@dataclasses.dataclass
class Run:
    """One temp project and what its setup recorded"""

    project: pathlib.Path
    bin_dir: pathlib.Path
    fuku_bin: pathlib.Path
    profile: str
    env: dict
    fingerprint: str
    values: dict = dataclasses.field(default_factory=dict)
    expected_commands: dict = dataclasses.field(default_factory=dict)
    seen_pids: set = dataclasses.field(default_factory=set)
    before: Snapshot = dataclasses.field(default_factory=Snapshot)
    after: Snapshot = dataclasses.field(default_factory=Snapshot)


@dataclasses.dataclass
class RunResult:
    """The checks of one run, each name mapped to None on a pass or to the reason it failed, and the host's result"""

    checks: dict
    agent: AgentResult

    @property
    def passed(self):
        return all(reason is None for reason in self.checks.values()) and self.agent.host_error is None


def split_name(name):
    """Splits a setup or check name into its base and its parameter"""
    base, _, param = name.partition(":")
    return base, param


def validate_name(path, name, known, label):
    """Rejects a name outside its closed list, or one whose parameter is missing or unknown"""
    base, param = split_name(name)
    if base not in known:
        raise UsageError(f"{path.name}: unknown {label} '{name}'")
    kind = known[base]
    if not kind and param:
        raise UsageError(f"{path.name}: {label} '{name}' takes no parameter")
    if kind == "service" and param not in SERVICES:
        raise UsageError(f"{path.name}: {label} '{name}' needs a fixture service: {', '.join(SERVICES)}")
    if kind == "template" and not (OVERRIDES / f"{param}.yaml").is_file():
        raise UsageError(f"{path.name}: {label} '{name}' names no template in {OVERRIDES}")


def validate_scenario(path, data):
    """Rejects a scenario whose keys, names or budget break the contract"""
    if not isinstance(data, dict) or set(data) != KEYS:
        found = sorted(data) if isinstance(data, dict) else type(data).__name__
        raise UsageError(f"{path.name}: keys must be exactly {sorted(KEYS)}, found {found}")
    if not isinstance(data["prompt"], str) or not data["prompt"].strip():
        raise UsageError(f"{path.name}: prompt must be a non-empty string")
    if data["profile"] not in PROFILES:
        raise UsageError(f"{path.name}: unknown profile '{data['profile']}', the fixture has: {', '.join(PROFILES)}")
    for key in ("setup", "checks"):
        if not isinstance(data[key], list) or not all(isinstance(n, str) for n in data[key]):
            raise UsageError(f"{path.name}: {key} must be a list of names")
    for name in data["setup"]:
        validate_name(path, name, SETUP, "setup")
    known_checks = {**STATE_CHECKS, **COMMAND_CHECKS}
    for name in data["checks"]:
        validate_name(path, name, known_checks, "check")
    budget = data["budget"]
    if not isinstance(budget, dict) or set(budget) != BUDGET:
        raise UsageError(f"{path.name}: budget keys must be exactly {sorted(BUDGET)}")
    if not all(isinstance(v, (int, float)) and not isinstance(v, bool) and v > 0 for v in budget.values()):
        raise UsageError(f"{path.name}: every budget value must be a positive number")


def load_scenarios(directory=SCENARIOS, ids=None):
    """Reads and validates every scenario file, or the named ones, before any host runs"""
    paths = sorted(pathlib.Path(directory).glob("*.json"))
    if ids:
        missing = sorted(set(ids) - {p.stem for p in paths})
        if missing:
            raise UsageError(f"unknown scenario: {', '.join(missing)}")
        paths = [p for p in paths if p.stem in ids]
    if not paths:
        raise UsageError(f"no scenario file in {directory}")

    scenarios = {}
    for path in paths:
        try:
            data = json.loads(path.read_text(encoding="utf-8"))
        except json.JSONDecodeError as err:
            raise UsageError(f"{path.name}: not valid JSON: {err}") from err
        validate_scenario(path, data)
        scenarios[path.stem] = data

    return scenarios


def fuku_binary(environ):
    """Returns the built fuku named by FUKU_BIN, the binary `make build` writes"""
    value = environ.get("FUKU_BIN", "")
    path = pathlib.Path(value).resolve() if value else None
    if path is None or not path.is_file() or not os.access(path, os.X_OK):
        raise UsageError(f"FUKU_BIN must name the built fuku, run `make build` first (got '{value}')")

    return path


def fingerprint(project):
    """Returns the project fingerprint fuku derives from the resolved directory"""
    return hashlib.sha256(str(pathlib.Path(project).resolve()).encode()).hexdigest()[:16]


def free_port():
    """Returns a loopback port that was free a moment ago"""
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        return sock.getsockname()[1]


def outside_repository(path):
    """Refuses a temp location inside the repository, where a run could touch the working tree"""
    resolved = pathlib.Path(path).resolve()
    if resolved.is_relative_to(REPO):
        raise UsageError(f"the temp dir {resolved} is inside the repository {REPO}, set TMPDIR elsewhere")

    return resolved


def prepare(fuku_bin, profile):
    """Copies the fixture into a fresh temp project and puts the built fuku first on its PATH"""
    root = pathlib.Path(tempfile.mkdtemp(prefix="fuku-agents-e2e-")).resolve()
    try:
        outside_repository(root)
        project = root / "project"
        shutil.copytree(FIXTURE, project, ignore=shutil.ignore_patterns(OVERRIDES.name))
        bin_dir = root / "bin"
        bin_dir.mkdir()
        (bin_dir / "fuku").symlink_to(fuku_bin)
    except BaseException:
        shutil.rmtree(root, ignore_errors=True)
        raise

    env = {key: value for key, value in os.environ.items() if key != "FUKU_BIN"}
    env["PATH"] = f"{bin_dir}{os.pathsep}{env.get('PATH', '')}"
    values = {"PORT": str(free_port()), "TOKEN": secrets.token_hex(16), "MARKER": secrets.token_hex(4)}

    return Run(project, bin_dir, fuku_bin, profile, env, fingerprint(project), values)


def fuku(run, *args, timeout=STOP_SECONDS):
    """Runs the built fuku in the project"""
    return subprocess.run(["fuku", *args], cwd=run.project, env=run.env, capture_output=True, text=True,
                          timeout=timeout, check=False)


def capture(args):
    """Runs a probe command and returns its stdout, raising ProbeError when it does not finish in time"""
    try:
        return subprocess.run(args, capture_output=True, text=True, timeout=PROBE_SECONDS, check=False).stdout
    except subprocess.TimeoutExpired as err:
        raise ProbeError(f"{args[0]} did not answer within {PROBE_SECONDS}s") from err
    except OSError as err:
        raise ProbeError(f"{args[0]} could not run: {err.strerror}") from err


def socket_path(run):
    """Returns the project socket"""
    return SOCKET_DIR / f"fuku-{run.fingerprint}.sock"


def lock_path(run):
    """Returns the project lock"""
    return SOCKET_DIR / f"fuku-{run.fingerprint}.lock"


def instance_status(run):
    """Returns the status frame of the fuku on the project socket, or None when nothing answers"""
    try:
        with socket.socket(socket.AF_UNIX, socket.SOCK_STREAM) as sock:
            sock.settimeout(2)
            sock.connect(str(socket_path(run)))
            sock.sendall(b'{"type":"subscribe","services":[],"tail":1,"noFollow":true}\n')
            line = sock.makefile("rb").readline()
    except OSError:
        return None
    try:
        status = json.loads(line)
    except json.JSONDecodeError:
        return None

    return status if status.get("type") == "status" and status.get("fingerprint") == run.fingerprint else None


def processes():
    """Returns every visible process as {pid: ppid}"""
    pairs = (line.split() for line in capture(["ps", "-A", "-o", "pid=,ppid="]).splitlines() if line.strip())

    return {int(pid): int(ppid) for pid, ppid in pairs}


def lsof_paths(pids, kind):
    """Returns {pid: path} of each process's cwd or executable, read from /proc or lsof"""
    proc = pathlib.Path("/proc")
    if proc.is_dir():
        found = {}
        for pid in pids:
            try:
                found[pid] = os.readlink(proc / str(pid) / ("cwd" if kind == "cwd" else "exe"))
            except OSError:
                pass
        return found

    args = ["lsof", "-a", "-d", kind, "-Fpn"]
    if pids is not None:
        args += ["-p", ",".join(str(p) for p in pids)]
    found, pid = {}, None
    for line in capture(args).splitlines():
        if line.startswith("p"):
            pid = int(line[1:])
        if line.startswith("n") and pid is not None and pid not in found:
            found[pid] = line[1:]

    return found


def all_cwds():
    """Returns {pid: cwd} of every process the user can see"""
    if pathlib.Path("/proc").is_dir():
        return lsof_paths([int(p) for p in os.listdir("/proc") if p.isdigit()], "cwd")

    return lsof_paths(None, "cwd")


def snapshot(run):
    """Reads the fuku PID from the project socket and each service PID as a child of it in the service's dir"""
    status = instance_status(run)
    if status is None or not status.get("pid"):
        return Snapshot()

    fuku_pid = status["pid"]
    try:
        children = [pid for pid, ppid in processes().items() if ppid == fuku_pid]
        cwds = lsof_paths(children, "cwd") if children else {}
    except ProbeError:
        return Snapshot(fuku_pid)
    by_dir = {cwd: pid for pid, cwd in cwds.items()}
    services = {name: by_dir.get(str(run.project / name)) for name in status.get("services") or []}
    run.seen_pids.update([fuku_pid, *[pid for pid in services.values() if pid]])

    return Snapshot(fuku_pid, services)


def start_fuku(run, _):
    """Starts the profile in the background, as a developer would before the prompt"""
    try:
        result = fuku(run, "run", run.profile, "-d")
    except subprocess.TimeoutExpired as err:
        raise SetupError(f"fuku run {run.profile} -d did not return within {STOP_SECONDS}s") from err
    if result.returncode != 0:
        raise SetupError(f"fuku run {run.profile} -d exited {result.returncode}: {result.stderr.strip()}")


def write_override(run, template):
    """Writes the named template as the project override with this run's port, token and marker"""
    text = (OVERRIDES / f"{template}.yaml").read_text(encoding="utf-8")
    for key, value in run.values.items():
        text = text.replace("{{" + key + "}}", value)
    (run.project / "fuku.override.yaml").write_text(text, encoding="utf-8")


def wait_service_exited(run, service):
    """Waits until fuku runs and the named service has no process left"""
    deadline = time.monotonic() + WAIT_SECONDS
    while time.monotonic() < deadline:
        current = snapshot(run)
        if current.fuku_pid and service in current.services and current.services[service] is None:
            return
        time.sleep(0.2)
    raise SetupError(f"service '{service}' still runs after {WAIT_SECONDS}s")


def edit_service_command(run, service):
    """Changes the program a service execs in fuku.yaml, so only a profile restart can apply it"""
    config = run.project / "fuku.yaml"
    lines = config.read_text(encoding="utf-8").splitlines(keepends=True)
    start = lines.index(f"  {service}:\n")
    index = next(i for i in range(start + 1, len(lines)) if lines[i].startswith("    command:"))
    match = re.search(r"exec sleep (\d+)", lines[index])
    if match is None:
        raise SetupError(f"service '{service}' has no 'exec sleep <n>' command to edit")
    program = f"sleep {int(match.group(1)) + 10}"
    lines[index] = lines[index].replace(match.group(0), f"exec {program}")
    config.write_text("".join(lines), encoding="utf-8")
    run.expected_commands[service] = program


SETUP_STEPS = {
    "start_fuku": start_fuku,
    "override": write_override,
    "wait_service_exited": wait_service_exited,
    "edit_service_command": edit_service_command,
}


def check_state(name, run):
    """Evaluates one state check, returning None on a pass or the reason it failed"""
    base, param = split_name(name)
    before, after = run.before, run.after

    if base == "fuku_running":
        return None if after.fuku_pid else "no fuku answers on the project socket"
    if base == "services_running":
        stopped = sorted(n for n, pid in after.services.items() if pid is None)
        return None if after.fuku_pid and after.services and not stopped else f"not running: {stopped or 'fuku'}"
    if base == "same_fuku_pid":
        same = before.fuku_pid is not None and before.fuku_pid == after.fuku_pid
        return None if same else f"fuku pid {before.fuku_pid} became {after.fuku_pid}"
    if base == "same_service_pids":
        same = after.fuku_pid is not None and before.services == after.services
        return None if same else f"service pids {before.services} became {after.services}"
    if base == "fuku_pid_changed":
        changed = after.fuku_pid is not None and before.fuku_pid not in (None, after.fuku_pid)
        return None if changed else f"fuku pid {before.fuku_pid} became {after.fuku_pid}"
    if base == "service_pid_changed":
        old, new = before.services.get(param), after.services.get(param)
        return None if old and new and old != new else f"{param} pid {old} became {new}"
    if base == "other_service_pids_same":
        others = {n: p for n, p in before.services.items() if n != param}
        now = {n: p for n, p in after.services.items() if n != param}
        return None if after.fuku_pid and others == now else f"other service pids {others} became {now}"
    if base == "service_command_is":
        return check_service_command(run, param)
    if base == "service_stopped":
        pid = after.services.get(param)
        return None if pid is None else f"{param} runs as pid {pid}"
    if base == "served_by_built_fuku":
        return check_served_by_built_fuku(run)

    raise ValueError(f"unknown state check '{name}'")


def check_service_command(run, service):
    """Compares the argument line of the service's process with the command the setup wrote"""
    pid = run.after.services.get(service)
    try:
        actual = capture(["ps", "-o", "command=", "-p", str(pid)]).strip() if pid else None
    except ProbeError as err:
        return str(err)
    expected = run.expected_commands.get(service)

    return None if actual and actual == expected else f"{service} runs '{actual}', expected '{expected}'"


def check_served_by_built_fuku(run):
    """Compares the executable of the fuku on the project socket with the built binary"""
    pid = run.after.fuku_pid
    if not pid:
        return "no fuku answers on the project socket"
    try:
        exe = lsof_paths([pid], "txt").get(pid)
    except ProbeError as err:
        return str(err)
    built = str(run.fuku_bin)

    return None if exe and os.path.realpath(exe) == built else f"fuku pid {pid} runs '{exe}', not {built}"


def pid_alive(pid):
    """Reports whether a PID names a live, unreaped process"""
    try:
        os.kill(pid, 0)
    except ProcessLookupError:
        return False
    except PermissionError:
        return True
    out = capture(["ps", "-o", "stat=", "-p", str(pid)]).strip()

    return bool(out) and not out.startswith("Z")


def project_pids(run):
    """Returns the processes working inside the temp project, from a fresh scan, never the runner or its ancestors"""
    tree = processes()
    protected, pid = {os.getpid()}, os.getpid()
    while pid in tree and pid not in (0, 1):
        pid = tree[pid]
        protected.add(pid)
    root = str(run.project)

    inside = (p for p, cwd in all_cwds().items() if cwd == root or cwd.startswith(root + os.sep))

    return sorted(p for p in inside if p not in protected)


def signal_each(pids, signum):
    """Sends a signal to each PID that still exists"""
    for pid in pids:
        with contextlib.suppress(ProcessLookupError):
            os.kill(pid, signum)


def stop_instance(run):
    """Sends SIGTERM to the fuku on the project socket when it is this run's, and waits for it to exit"""
    status = instance_status(run)
    pid = status.get("pid") if status else None
    if not pid:
        return
    if lsof_paths([pid], "cwd").get(pid) != str(run.project):
        return
    signal_each([pid], signal.SIGTERM)
    deadline = time.monotonic() + STOP_SECONDS
    while time.monotonic() < deadline and pid_alive(pid):
        time.sleep(0.1)


def kill_in_project(run):
    """Sends SIGTERM to every process in the temp project, then SIGKILL to those a fresh scan still finds there"""
    signal_each(project_pids(run), signal.SIGTERM)
    deadline = time.monotonic() + GRACE_SECONDS
    while time.monotonic() < deadline and project_pids(run):
        time.sleep(0.1)
    signal_each(project_pids(run), signal.SIGKILL)


def teardown(run):
    """Stops this run's fuku, kills what is left in the project, removes socket, lock and temp root, then checks"""
    left = []
    try:
        # no fuku stop: its dir cleanup reads a config the agent may have pointed outside the project
        stop_instance(run)
        kill_in_project(run)
        remaining = project_pids(run)
        left += [f"pid {pid}" for pid in remaining]
        if not remaining:
            for path in (socket_path(run), lock_path(run)):
                path.unlink(missing_ok=True)
    except ProbeError as err:
        left.append(str(err))
    shutil.rmtree(run.project.parent, ignore_errors=True)
    left += [str(p) for p in (socket_path(run), lock_path(run), run.project.parent) if p.exists()]

    return None if not left else f"left behind: {', '.join(left)}"


def run_scenario(scenario, agent, fuku_bin):
    """Runs one scenario: temp project, setup, the agent under its hard limit, the checks, teardown on every path"""
    run, results, result = None, {}, AgentResult()
    try:
        run = prepare(fuku_bin, scenario["profile"])
        try:
            for name in scenario["setup"]:
                base, param = split_name(name)
                SETUP_STEPS[base](run, param)
            run.before = snapshot(run)
        except SetupError as err:
            results["setup"] = str(err)
            return RunResult(results, result)

        result = agent.run(run.project, scenario["prompt"], run.env, budget=scenario["budget"],
                           secrets=[run.values["TOKEN"]], timeout=checks.hard_limit(scenario["budget"]))
        run.after = snapshot(run)
        for name in scenario["checks"]:
            if split_name(name)[0] in STATE_CHECKS:
                results[name] = check_state(name, run)
    finally:
        if run is not None:
            results["teardown"] = teardown(run)

    results.update(checks.check_commands(scenario, result, context(run, result)))
    results.update(checks.check_budget(scenario["budget"], result.totals))
    results["time_limit"] = f"killed after {checks.hard_limit(scenario['budget'])}s" if result.timed_out else None
    result.token_exposed = hosts.redact(result, [run.values["TOKEN"]]) or result.token_exposed

    return RunResult(results, result)


def context(run, result):
    """Returns what the command checks need to know about a run"""
    return {
        "project": run.project,
        "profile": run.profile,
        "token": run.values["TOKEN"],
        "marker": run.values["MARKER"],
        "pids": run.seen_pids,
        "targets": ["fuku", *SERVICE_PROGRAMS, *(p.split()[0] for p in SERVICE_PROGRAMS),
                    *run.expected_commands.values(),
                    *(str(run.project / name) for name in SERVICES)],
        "skill_loaded": result.skill_loaded,
        "credentials_read": result.credentials_read,
        "token_exposed": result.token_exposed,
    }


HOSTS = ["claude", "codex"]
DEFAULT_REPEATS = 3


def sweep_options(environ):
    """Reads HOST, SCENARIO and REPEATS, refusing an unknown host or a repeat count below one"""
    host = environ.get("HOST", "")
    if host and host not in HOSTS:
        raise UsageError(f"HOST must be one of {', '.join(HOSTS)}, got '{host}'")
    repeats = environ.get("REPEATS") or str(DEFAULT_REPEATS)
    if not repeats.isdigit() or int(repeats) < 1:
        raise UsageError(f"REPEATS must be a whole number of at least 1, got '{repeats}'")
    ids = [s for s in environ.get("SCENARIO", "").split(",") if s]

    return [host] if host else HOSTS, ids, int(repeats)


def default_adapters(environ):
    """Returns how each host's adapter is made: the working-tree plugin for Claude Code, a private home for Codex"""
    return {"claude": lambda: claude.ClaudeAdapter(PLUGIN), "codex": lambda: codex.CodexAdapter(REPO, environ)}


def sweep_host(host, make_adapter, scenarios, repeats, fuku_bin, records, problems):
    """Runs every scenario of one host in turn, stopping the host at its first outage or missing login"""
    try:
        with make_adapter() as adapter:
            for scenario_id, scenario in scenarios.items():
                for number in range(1, repeats + 1):
                    result = run_scenario(scenario, adapter, fuku_bin)
                    record = report.classify(host, scenario_id, number, result)
                    records.append(record)
                    print(f"{host} {scenario_id} #{number}: {record['status']}", flush=True)
                    stop = report.stop_message(host, record)
                    if stop:
                        problems.append(stop)
                        return
    except codex.AdapterError as err:
        problems.append(f"{host}: {err}")


def main(directory=SCENARIOS, environ=None, adapters=None):
    """Runs HOST x SCENARIO x REPEATS, writes the report outside the repository and returns the exit code"""
    environ = os.environ if environ is None else environ
    try:
        hosts_wanted, ids, repeats = sweep_options(environ)
        scenarios = load_scenarios(directory, ids)
        fuku_bin = fuku_binary(environ)
        outside_repository(tempfile.gettempdir())
    except UsageError as err:
        print(f"runner: {err}", file=sys.stderr)
        return 2

    adapters = adapters or default_adapters(environ)
    home = environ.get("HOME", str(pathlib.Path.home()))
    before = report.guard_snapshot(home)
    records, problems = [], []
    for host in hosts_wanted:
        sweep_host(host, adapters[host], scenarios, repeats, fuku_bin, records, problems)
    problems += [f"guard: {change}" for change in report.compare(before, report.guard_snapshot(home))]

    summary = report.summarize(records, hosts_wanted, list(scenarios))
    out = outside_repository(tempfile.mkdtemp(prefix="fuku-agents-report-"))
    path = report.write(out, records, summary, problems, report.largest_outputs(records))
    print(report.table(summary))
    for problem in problems:
        print(f"runner: {problem}", file=sys.stderr)
    for record in records:
        if record["flags"]["login_rotated"]:
            print(f"runner: {record['host']} {record['scenario']} #{record['run']}: {record['flags']['login_rotated']}",
                  file=sys.stderr)
    print(f"report: {path}")

    return report.exit_code(summary, problems)


def terminate(signum, _frame):
    """Turns SIGTERM into an exit that unwinds the sweep through the cleanup an interrupt takes, ignoring a repeat"""
    signal.signal(signal.SIGTERM, signal.SIG_IGN)
    raise SystemExit(128 + signum)


if __name__ == "__main__":
    signal.signal(signal.SIGTERM, terminate)
    sys.exit(main())
