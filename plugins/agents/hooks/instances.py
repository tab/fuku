#!/usr/bin/env python3
"""Report running fuku instances at session start so the session reuses them."""

from __future__ import annotations

import importlib.util
import json
import os
import pathlib
import sys
from typing import Any


CONTROL_PATH = pathlib.Path(__file__).resolve().parents[1] / "skills" / "fuku" / "scripts" / "control.py"
CONFIG_NAMES = ("fuku.yaml", "fuku.yml")
PROBE_TIMEOUT = 0.1


def project_root() -> pathlib.Path:
    """Resolve the project directory the session was started in."""
    return pathlib.Path(os.environ.get("CLAUDE_PROJECT_DIR") or os.getcwd())


def has_config(root: pathlib.Path) -> bool:
    """Report whether the project is orchestrated by fuku."""
    return any((root / name).is_file() for name in CONFIG_NAMES)


def load_control() -> Any:
    """Load the packaged control helper so discovery logic is not duplicated."""
    spec = importlib.util.spec_from_file_location("fuku_control", CONTROL_PATH)
    if spec is None or spec.loader is None:
        return None

    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)

    return module


def describe(control: Any) -> str:
    """Build the context that keeps the session on the developer's service stack."""
    try:
        discovery = control.discover(None, timeout=PROBE_TIMEOUT)
    except Exception:
        discovery = {"socket_profiles": [], "api": {"reachable": False, "reason": "discovery failed"}}

    api = discovery.get("api", {})
    instance = discovery.get("instance", {})
    if api.get("reachable") and instance:
        status = instance.get("status", {})
        services = instance.get("services", [])
        names = ", ".join(
            f"{item.get('name')} ({item.get('status')})" for item in services if item.get("name")
        )

        lines = [
            f"This project's fuku API is running for profile '{status.get('profile')}' "
            f"in phase '{status.get('phase')}' on {api.get('base_url')}.",
            f"Services: {names or 'none'}.",
            "Reuse this stack. Do not start another `fuku run` or start its services directly.",
        ]
    else:
        socket_profiles = discovery.get("socket_profiles", [])
        lines = ["No running fuku API was confirmed for this project."]
        if socket_profiles:
            lines.append(
                f"This project has live log sockets for profiles: {', '.join(socket_profiles)}. "
                "Read them with `fuku logs --profile <name>` and do not start those profiles again."
            )
        reason = api.get("reason")
        if reason:
            lines.append(f"Control API discovery: {reason}.")
        lines.append("Use the fuku skill to inspect the project before starting or controlling services.")

    lines.append("Use fuku logs and lifecycle actions instead of launching replacement service processes.")

    return "\n".join(lines)


def main() -> int:
    root = project_root()
    if not has_config(root):
        return 0

    os.chdir(root)

    control = load_control()
    if control is None:
        return 0

    payload = {
        "hookSpecificOutput": {
            "hookEventName": "SessionStart",
            "additionalContext": describe(control),
        }
    }

    json.dump(payload, sys.stdout)
    sys.stdout.write("\n")

    return 0


if __name__ == "__main__":
    try:
        code = main()
    except Exception:
        code = 0

    raise SystemExit(code)
