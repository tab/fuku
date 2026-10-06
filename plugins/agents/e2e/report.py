"""The sweep report: run classification, pass rates, medians, the largest fuku outputs and the host config guards"""

import hashlib
import json
import os
import pathlib
import statistics

import checks

OUTAGE_KINDS = {"api"}
STOPPING = {"outage", "login"}
LOGIN_HINTS = {"codex": "run `codex login`", "claude": "run `claude` and log in with /login"}
MEDIANS = ["tool_calls", "input_tokens", "output_tokens", "seconds", "usd"]
LARGEST = 10
ANSWER_CHARS = 4000


def classify(host, scenario, number, result):
    """Returns the record of one run: its status, failed checks, flags and totals"""
    agent = result.agent
    kind = agent.host_error_kind
    if kind == "crash" and "unreadable events" in (agent.host_error or ""):
        kind = "unreadable"
    failed = {name: reason for name, reason in result.checks.items() if reason is not None}
    if agent.host_error:
        failed["host_error"] = f"{kind}: {agent.host_error}"
    status = "outage" if kind in OUTAGE_KINDS else "login" if kind == "login" else "passed" if not failed else "failed"

    return {
        "host": host,
        "scenario": scenario,
        "run": number,
        "status": status,
        "failed": failed,
        "host_error_kind": kind,
        "flags": {
            "over_budget": any(name.startswith("budget:") for name in failed),
            "timed_out": agent.timed_out,
            "host_outage": status == "outage",
            "credentials_read": agent.credentials_read,
            "login_rotated": agent.login_rotated,
        },
        "totals": agent.totals,
        "calls": agent.calls,
        "answer": agent.answer[:ANSWER_CHARS],
    }


def stop_message(host, record):
    """Returns why a host's remaining runs were skipped, or None when the run does not stop the host"""
    if record["status"] == "login":
        return f"{host} is not logged in; {LOGIN_HINTS.get(host, 'log in')}"
    if record["status"] == "outage":
        return f"{host}: host outage, the remaining runs were skipped: {record['failed']['host_error']}"

    return None


def median(values):
    """Returns the median of the values that exist, or None"""
    present = [v for v in values if v is not None]

    return statistics.median(present) if present else None


def summarize(records, hosts, scenario_ids):
    """Returns per host and scenario the pass rate, the status, the runs and the medians of the completed runs"""
    summary = {}
    for host in hosts:
        for scenario in scenario_ids:
            runs = [r for r in records if r["host"] == host and r["scenario"] == scenario]
            completed = [r for r in runs if r["status"] not in STOPPING]
            passed = [r for r in completed if r["status"] == "passed"]
            status = "not run" if not completed else ("pass" if len(passed) == len(completed) else "fail")
            last = completed[-1]["totals"] if completed else {}
            summary[f"{host}/{scenario}"] = {
                "status": status,
                "pass_rate": len(passed) / len(completed) if completed else None,
                "completed": len(completed),
                "outages": sum(r["status"] == "outage" for r in runs),
                "medians": {key: median(r["totals"].get(key) for r in completed) for key in MEDIANS},
                "host_version": last.get("host_version"),
                "model": last.get("model"),
                "runs": [{k: r[k] for k in ("run", "status", "failed", "host_error_kind", "flags")} for r in runs],
            }

    return summary


def largest_outputs(records, limit=LARGEST):
    """Returns the largest fuku command outputs of the sweep, each tool call counted once"""
    found = []
    for record in records:
        for call in record["calls"]:
            if call["tool"] == "shell" and checks.fuku_invocations([call]):
                found.append({"command": call["command"], "bytes": len(call["output"].encode()),
                              "host": record["host"], "scenario": record["scenario"], "run": record["run"]})

    return sorted(found, key=lambda item: item["bytes"], reverse=True)[:limit]


def exit_code(summary, problems):
    """Returns 0 when every scenario passed on every host and nothing else failed, else 1"""
    return 0 if not problems and all(s["status"] == "pass" for s in summary.values()) else 1


def sha256(path):
    """Returns the sha256 of a file, or None when it is missing"""
    return hashlib.sha256(path.read_bytes()).hexdigest() if path.is_file() else None


def tree_fingerprint(root):
    """Returns {relative path: content hash} of every file, link and directory under a directory, skipping none"""
    found = {}

    def unreadable(err):
        found[os.path.relpath(err.filename, root)] = f"unreadable: {err.strerror}"

    for directory, dirs, files in os.walk(root, onerror=unreadable):
        for name in dirs + files:
            path = pathlib.Path(directory, name)
            found[str(path.relative_to(root))] = entry_hash(path)

    return found


def entry_hash(path):
    """Returns the sha256 of a file, the target of a link, a directory marker or the error that hid the entry"""
    try:
        if path.is_symlink():
            return f"link {os.readlink(path)}"
        if path.is_dir():
            return "dir"
        if not path.is_file():
            return "special"
        return sha256(path)
    except OSError as err:
        return f"unreadable: {err.strerror}"


def fuku_paths(root):
    """Returns the sorted paths under a directory whose own name contains fuku"""
    if not root.is_dir():
        return []
    found = []
    for directory, dirs, files in os.walk(root):
        found += [str(pathlib.Path(directory, n).relative_to(root)) for n in dirs + files if "fuku" in n.lower()]

    return sorted(found)


def guard_snapshot(home):
    """Fingerprints the real Codex and Claude Code config a sweep must not change: hashes, names and paths only"""
    home = pathlib.Path(home)

    return {
        "~/.codex/config.toml": sha256(home / ".codex" / "config.toml"),
        "~/.codex/plugins fuku paths": fuku_paths(home / ".codex" / "plugins"),
        "~/.claude/settings.json": sha256(home / ".claude" / "settings.json"),
        "~/.claude/plugins": tree_fingerprint(home / ".claude" / "plugins"),
    }


def compare(before, after):
    """Returns what changed between two guard snapshots, by name and path, never by content"""
    changes = []
    for key, old in before.items():
        new = after.get(key)
        if old == new:
            continue
        if isinstance(old, list) and isinstance(new, list):
            added, removed = sorted(set(new) - set(old)), sorted(set(old) - set(new))
            changes.append(f"{key}: added {added}, removed {removed}")
            continue
        if isinstance(old, dict) and isinstance(new, dict):
            added, removed = sorted(set(new) - set(old)), sorted(set(old) - set(new))
            edited = sorted(path for path in set(old) & set(new) if old[path] != new[path])
            changes.append(f"{key}: added {added}, removed {removed}, changed {edited}")
            continue
        changes.append(f"{key} changed")

    return changes


def table(summary):
    """Returns a short text table of the sweep"""
    header = ("host/scenario", "status", "rate", "runs", "calls", "input", "secs", "usd")
    lines = ["{:<16} {:<8} {:>5} {:>4} {:>5} {:>8} {:>6} {:>6}".format(*header)]
    for name, item in summary.items():
        rate = "-" if item["pass_rate"] is None else f"{item['pass_rate']:.0%}"
        m = item["medians"]
        cells = [("-" if m[k] is None else f"{m[k]:g}") for k in ("tool_calls", "input_tokens", "seconds", "usd")]
        lines.append(f"{name:<16} {item['status']:<8} {rate:>5} {item['completed']:>4} {cells[0]:>5} {cells[1]:>8} "
                     f"{cells[2]:>6} {cells[3]:>6}")

    return "\n".join(lines)


def write(directory, records, summary, problems, largest):
    """Writes report.json and one file of redacted calls per run, and returns the report path"""
    directory = pathlib.Path(directory)
    for record in records:
        name = f"{record['host']}-{record['scenario']}-{record['run']}.json"
        body = {k: record[k] for k in ("host", "scenario", "run", "status", "failed", "totals", "calls", "answer")}
        (directory / name).write_text(json.dumps(body, indent=2), encoding="utf-8")
    report = {"summary": summary, "problems": problems, "largest_fuku_outputs": largest}
    path = directory / "report.json"
    path.write_text(json.dumps(report, indent=2), encoding="utf-8")

    return path
