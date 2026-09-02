#!/usr/bin/env python3
"""Inspect and control a running fuku instance through its loopback API."""

from __future__ import annotations

import argparse
import hashlib
import ipaddress
import json
import math
import os
import pathlib
import re
import socket
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
from typing import Any


API_PORT_RETRIES = 10
DEFAULT_TAIL = 100
SUMMARY_LIMIT = 20
MAX_PORT = 65535
PROBE_TIMEOUT = 0.25

SOCKET_DIR = pathlib.Path("/tmp")
SOCKET_PREFIX = "fuku-"
SOCKET_SUFFIX = ".sock"
SOCKET_DIAL_TIMEOUT = 0.5
SOCKET_STATUS_LIMIT = 65536

PROJECT_FINGERPRINT_LENGTH = 16
IDENTITY_VERSION = "0.22.0"


class ControlError(Exception):
    """A safe user-facing control error."""


class NoRedirectHandler(urllib.request.HTTPRedirectHandler):
    """Reject redirects so the bearer token stays on the configured loopback endpoint."""

    def redirect_request(self, request, file_pointer, code, message, headers, new_url):
        raise ControlError("fuku API redirects are not allowed")


def normalize_base_url(value: str) -> str:
    """Validate and normalize a loopback fuku API URL."""
    if not value:
        raise ControlError("set server.listen, --base-url or FUKU_BASE_URL")

    try:
        parsed = urllib.parse.urlsplit(value)
    except ValueError as exc:
        raise ControlError("fuku API URL is invalid") from exc
    if parsed.scheme != "http":
        raise ControlError("fuku API URL must use http")
    if parsed.username or parsed.password:
        raise ControlError("fuku API URL must not contain credentials")
    if parsed.path not in ("", "/") or parsed.query or parsed.fragment:
        raise ControlError("fuku API URL must not contain a path, query or fragment")

    host = (parsed.hostname or "").lower()
    if host not in {"localhost", "ip6-localhost"}:
        try:
            if not ipaddress.ip_address(host).is_loopback:
                raise ControlError("fuku API URL must use a loopback host")
        except ValueError as exc:
            raise ControlError("fuku API URL must use a loopback host") from exc

    try:
        if parsed.port is None or parsed.port == 0:
            raise ControlError("fuku API URL must include a port")
    except ValueError as exc:
        raise ControlError("fuku API URL has an invalid port") from exc

    return value.rstrip("/")


def json_object(data: bytes) -> dict[str, Any]:
    """Decode an API response and require a JSON object."""
    try:
        result = json.loads(data.decode("utf-8"))
    except (UnicodeDecodeError, json.JSONDecodeError) as exc:
        raise ControlError("fuku API returned invalid JSON") from exc

    if not isinstance(result, dict):
        raise ControlError("fuku API returned an unexpected response")

    return result


class APIClient:
    """Small standard-library client for the fuku control API."""

    def __init__(self, base_url: str, token: str | None, request_timeout: float = 10.0) -> None:
        self.base_url = normalize_base_url(base_url)
        self._token = token
        self.request_timeout = request_timeout
        self.opener = urllib.request.build_opener(
            urllib.request.ProxyHandler({}),
            NoRedirectHandler(),
        )

    def request(self, method: str, path: str, authenticated: bool = True) -> dict[str, Any]:
        headers = {"Accept": "application/json"}
        if authenticated:
            if not self._token:
                raise ControlError("set server.auth.token or FUKU_API_TOKEN before using this command")
            headers["Authorization"] = f"Bearer {self._token}"

        request = urllib.request.Request(
            f"{self.base_url}/api/v1{path}",
            headers=headers,
            method=method,
        )

        try:
            with self.opener.open(request, timeout=self.request_timeout) as response:
                return json_object(response.read())
        except urllib.error.HTTPError as exc:
            try:
                exc.read()
            finally:
                exc.close()
            raise ControlError(f"fuku API returned HTTP {exc.code}") from exc
        except urllib.error.URLError as exc:
            reason = getattr(exc, "reason", exc)
            raise ControlError(f"cannot reach fuku API: {reason}") from exc

    def live(self) -> dict[str, Any]:
        return self.request("GET", "/live", authenticated=False)

    def ready(self) -> dict[str, Any]:
        return self.request("GET", "/ready", authenticated=False)

    def status(self) -> dict[str, Any]:
        return self.request("GET", "/status")

    def services(self) -> list[dict[str, Any]]:
        result = self.request("GET", "/services")
        services = result.get("services")
        if not isinstance(services, list) or not all(isinstance(item, dict) for item in services):
            raise ControlError("fuku API returned an invalid service list")
        return services

    def service(self, service_id: str) -> dict[str, Any]:
        safe_id = urllib.parse.quote(service_id, safe="")
        return self.request("GET", f"/services/{safe_id}")

    def action(self, service_id: str, action: str) -> dict[str, Any]:
        safe_id = urllib.parse.quote(service_id, safe="")
        return self.request("POST", f"/services/{safe_id}/{action}")

    def logs(self, service_ids: list[str], tail: int, since: str | None) -> dict[str, Any]:
        params: list[tuple[str, str]] = [("service", service_id) for service_id in service_ids]
        params.append(("tail", str(tail)))
        if since:
            params.append(("since", since))

        result = self.request("GET", "/logs?" + urllib.parse.urlencode(params))
        lines = result.get("lines")
        if not isinstance(lines, list) or not all(isinstance(item, dict) for item in lines):
            raise ControlError("fuku API returned an invalid log list")

        return result


def service_by_name(services: list[dict[str, Any]], name: str) -> dict[str, Any]:
    """Find one service by exact name."""
    matches = [service for service in services if service.get("name") == name]
    if not matches:
        raise ControlError(f"service not found in active profile: {name}")
    if len(matches) != 1:
        raise ControlError(f"service name is not unique in active profile: {name}")
    return matches[0]


def completed_without_action(action: str, service: dict[str, Any]) -> dict[str, Any] | None:
    status = service.get("status")
    if action == "start" and status == "running":
        return {"action": action, "result": "already running", "service": service}
    if action == "stop" and status == "stopped":
        return {"action": action, "result": "already stopped", "service": service}
    return None


def validate_action_state(action: str, status: Any) -> None:
    allowed = {
        "start": {"stopped", "failed"},
        "stop": {"running"},
        "restart": {"running", "stopped", "failed"},
    }
    if status not in allowed[action]:
        states = ", ".join(sorted(allowed[action]))
        raise ControlError(f"cannot {action} service in state {status!r}; expected one of: {states}")


def lifecycle_moved(initial: dict[str, Any], current: dict[str, Any], saw_transition: bool) -> bool:
    """Report whether the service passed a lifecycle transition since the initial read.

    fuku exposes a monotonic revision for exactly this. An instance reached through --base-url may
    predate that field, and there the only observable signals are a status change or a new pid.
    """
    before = initial.get("revision")
    after = current.get("revision")
    if isinstance(before, int) and isinstance(after, int):
        return after > before

    return saw_transition or current.get("pid") != initial.get("pid")


def action_complete(
    action: str,
    initial: dict[str, Any],
    current: dict[str, Any],
    moved: bool,
) -> bool:
    status = current.get("status")
    if action == "stop":
        return status == "stopped"
    if action == "start":
        return status == "running"
    if status != "running":
        return False
    if initial.get("status") != "running":
        return True
    return moved


def wait_for_action(
    client: APIClient,
    action: str,
    initial: dict[str, Any],
    wait_seconds: float,
    poll_interval: float,
) -> dict[str, Any]:
    deadline = time.monotonic() + wait_seconds
    saw_transition = False
    current = initial
    initial_status = initial.get("status")

    while time.monotonic() < deadline:
        if poll_interval > 0:
            time.sleep(poll_interval)

        current = client.service(str(initial["id"]))
        current_status = current.get("status")
        if current_status != initial_status:
            saw_transition = True

        moved = lifecycle_moved(initial, current, saw_transition)

        if action_complete(action, initial, current, moved):
            return current

        if current_status == "failed" and action in {"start", "restart"} and moved:
            error = current.get("error") or "service entered failed state"
            raise ControlError(f"{current.get('name')} failed: {error}")

    raise ControlError(
        f"timed out after {wait_seconds:g}s waiting for {initial.get('name')} to {action}; "
        f"last state: {current.get('status')}"
    )


def perform_action(
    client: APIClient,
    action: str,
    name: str,
    wait_seconds: float,
    poll_interval: float,
    no_wait: bool,
) -> dict[str, Any]:
    status = client.status()
    if status.get("phase") != "running":
        raise ControlError(f"fuku is not accepting actions in phase {status.get('phase')!r}")

    profile = status.get("profile")
    service = service_by_name(client.services(), name)
    completed = completed_without_action(action, service)
    if completed is not None:
        return {**completed, "profile": profile}

    validate_action_state(action, service.get("status"))
    accepted = client.action(str(service["id"]), action)
    if no_wait:
        return {"action": action, "result": "accepted", "profile": profile, "service": accepted}

    final_service = wait_for_action(client, action, service, wait_seconds, poll_interval)
    return {"action": action, "result": "completed", "profile": profile, "service": final_service}


def collapse_lines(lines: list[dict[str, Any]]) -> list[dict[str, Any]]:
    """Fold consecutive identical lines into one entry carrying how often it repeated."""
    collapsed: list[dict[str, Any]] = []

    for line in lines:
        previous = collapsed[-1] if collapsed else None
        same = (
            previous is not None
            and previous["service"] == line.get("service")
            and previous["message"] == line.get("message")
        )

        if same:
            previous["repeat"] += 1
            previous["last"] = line.get("timestamp")

            continue

        collapsed.append(
            {
                "service": line.get("service"),
                "message": line.get("message"),
                "repeat": 1,
                "first": line.get("timestamp"),
                "last": line.get("timestamp"),
            }
        )

    return collapsed


def summarize_lines(lines: list[dict[str, Any]], limit: int = SUMMARY_LIMIT) -> tuple[list[dict[str, Any]], int]:
    """Count each distinct service and message pair, most frequent first, keeping only the top entries."""
    counts: dict[tuple[str, str], dict[str, Any]] = {}

    for line in lines:
        key = (line.get("service"), line.get("message"))
        entry = counts.setdefault(
            key,
            {"service": line.get("service"), "message": line.get("message"), "count": 0},
        )
        entry["count"] += 1
        entry["last"] = line.get("timestamp")

    ranked = sorted(counts.values(), key=lambda entry: entry["count"], reverse=True)

    return ranked[:limit], max(0, len(ranked) - limit)


def read_logs(
    client: APIClient,
    names: list[str],
    tail: int,
    since: str | None,
    summary: bool,
) -> dict[str, Any]:
    """Read the buffered output of the running instance without following the live stream."""
    status = client.status()
    services = client.services()

    service_ids = [str(service_by_name(services, name)["id"]) for name in names]

    result = client.logs(service_ids, tail, since)
    lines = result.get("lines", [])

    report: dict[str, Any] = {
        "profile": status.get("profile"),
        "tail": result.get("tail", tail),
        "total": len(lines),
    }

    if summary:
        report["summary"], omitted = summarize_lines(lines)
        if omitted:
            report["omitted"] = omitted

        return report

    report["lines"] = collapse_lines(lines)

    return report


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--config", help="use one explicit config file without an override")
    parser.add_argument("--base-url", help="override server.listen with a loopback HTTP URL")
    parser.add_argument("--token-env", default="FUKU_API_TOKEN")
    parser.add_argument("--request-timeout", type=float, default=10.0)

    subparsers = parser.add_subparsers(dest="command", required=True)
    subparsers.add_parser("discover", help="find the configured project API and live socket profiles")
    subparsers.add_parser("live", help="check API liveness")
    subparsers.add_parser("ready", help="check whether profile data is resolved")
    subparsers.add_parser("status", help="show fuku phase and service counts")

    services_parser = subparsers.add_parser("services", help="show all or named services")
    services_parser.add_argument("names", nargs="*")

    logs_parser = subparsers.add_parser("logs", help="read buffered output without following the stream")
    logs_parser.add_argument("names", nargs="*")
    logs_parser.add_argument("--tail", type=int, default=DEFAULT_TAIL)
    logs_parser.add_argument("--since", help="only lines buffered within this duration, such as 30s or 5m")
    logs_parser.add_argument(
        "--summary",
        action="store_true",
        help="report how often each distinct line repeated instead of the lines themselves",
    )

    for action in ("start", "stop", "restart"):
        action_parser = subparsers.add_parser(action, help=f"{action} one exact service name")
        action_parser.add_argument("name")
        action_parser.add_argument("--wait-seconds", type=float, default=60.0)
        action_parser.add_argument("--poll-interval", type=float, default=0.5)
        action_parser.add_argument("--no-wait", action="store_true")

    return parser


def validate_numeric_options(args: argparse.Namespace) -> None:
    if not math.isfinite(args.request_timeout) or args.request_timeout <= 0:
        raise ControlError("--request-timeout must be a finite number greater than zero")

    if args.command == "logs" and args.tail <= 0:
        raise ControlError("--tail must be a whole number greater than zero")

    if args.command not in {"start", "stop", "restart"}:
        return

    if not math.isfinite(args.wait_seconds) or args.wait_seconds <= 0:
        raise ControlError("--wait-seconds must be a finite number greater than zero")
    if not math.isfinite(args.poll_interval) or args.poll_interval < 0:
        raise ControlError("--poll-interval must be a finite number greater than or equal to zero")


YAML_KEY = re.compile(r"^(?P<indent> *)(?P<key>[A-Za-z0-9_-]+):(?P<value>.*)$")
UNSUPPORTED_SCALAR_PREFIXES = ("&", "*", "!", "|", ">", "{", "[")


def strip_yaml_comment(value: str) -> str:
    """Remove a YAML comment while keeping hashes inside quoted or plain values."""
    single_quoted = False
    double_quoted = False
    escaped = False

    for index, character in enumerate(value):
        if double_quoted and character == "\\" and not escaped:
            escaped = True
            continue
        if character == '"' and not single_quoted and not escaped:
            double_quoted = not double_quoted
        elif character == "'" and not double_quoted:
            single_quoted = not single_quoted
        elif character == "#" and not single_quoted and not double_quoted:
            if index == 0 or value[index - 1].isspace():
                return value[:index].rstrip()
        escaped = False

    return value.rstrip()


def yaml_scalar(value: str) -> str | None:
    """Read one safe scalar without exposing it in an error."""
    value = strip_yaml_comment(value.strip())
    if not value:
        raise ControlError("server connection setting must be a scalar value")

    if value.startswith('"'):
        try:
            parsed = json.loads(value)
        except json.JSONDecodeError as exc:
            raise ControlError("server connection setting has invalid quoting") from exc
        if not isinstance(parsed, str):
            raise ControlError("server connection setting must be a string")
        return parsed

    if value.startswith("'"):
        if len(value) < 2 or not value.endswith("'"):
            raise ControlError("server connection setting has invalid quoting")
        return value[1:-1].replace("''", "'")

    if value.startswith(UNSUPPORTED_SCALAR_PREFIXES):
        raise ControlError("server connection setting uses an unsupported YAML value")

    value = re.split(r"\s+#", value, maxsplit=1)[0].rstrip()
    if value.lower() in {"null", "~"}:
        return None
    return value


def read_server_settings(path: pathlib.Path) -> dict[str, str | None]:
    """Read only server.listen and server.auth.token from one config file."""
    try:
        lines = path.read_text(encoding="utf-8").splitlines()
    except (OSError, UnicodeError) as exc:
        raise ControlError(f"cannot read config file: {path}") from exc

    settings: dict[str, str | None] = {}
    parents: list[tuple[int, str]] = []

    for line in lines:
        if not line.strip() or line.lstrip().startswith(("#", "---", "...")):
            continue
        if line[: len(line) - len(line.lstrip())].find("\t") != -1:
            raise ControlError(f"config file uses tab indentation: {path}")

        match = YAML_KEY.match(line)
        if match is None:
            continue

        indent = len(match.group("indent"))
        while parents and indent <= parents[-1][0]:
            parents.pop()

        key = match.group("key")
        value = match.group("value").strip()
        current_path = tuple(parent_key for _, parent_key in parents) + (key,)

        if current_path == ("server",) and value and not value.startswith("#"):
            server_value = yaml_scalar(value)
            if server_value not in (None, "{}"):
                raise ControlError("server setting must be a YAML map")
            settings.update({"listen": None, "token": None})
        elif current_path == ("server", "auth") and value and not value.startswith("#"):
            auth_value = yaml_scalar(value)
            if auth_value not in (None, "{}"):
                raise ControlError("server.auth setting must be a YAML map")
            settings["token"] = None
        elif current_path == ("server", "listen"):
            settings["listen"] = yaml_scalar(value)
        elif current_path == ("server", "auth", "token"):
            settings["token"] = yaml_scalar(value)

        if not value or value.startswith("#"):
            parents.append((indent, key))

    return settings


def config_paths(explicit: str | None) -> list[pathlib.Path]:
    """Resolve config files with the same default and override precedence as fuku."""
    if explicit:
        path = pathlib.Path(explicit)
        if not path.is_file():
            raise ControlError(f"config file not found: {path}")
        return [path]

    root = pathlib.Path.cwd()
    base = next((root / name for name in ("fuku.yaml", "fuku.yml") if (root / name).is_file()), None)
    if base is None:
        raise ControlError("fuku.yaml or fuku.yml was not found")

    paths = [base]
    for name in ("fuku.override.yaml", "fuku.override.yml"):
        override = base.parent / name
        if override.is_file():
            paths.append(override)
            break
    return paths


def load_server_settings(explicit: str | None) -> dict[str, str | None]:
    """Load effective server settings without returning them to the command output."""
    settings: dict[str, str | None] = {}
    for path in config_paths(explicit):
        settings.update(read_server_settings(path))
    return settings


def base_url_from_listen(value: str) -> str:
    """Convert the configured host:port listen address to an HTTP URL."""
    return normalize_base_url(value if "://" in value else f"http://{value}")


def join_host_port(host: str, port: int) -> str:
    """Join a host and port, bracketing an IPv6 literal."""
    if ":" in host:
        return f"[{host}]:{port}"
    return f"{host}:{port}"


def scanned_ports(listen: str, retries: int = API_PORT_RETRIES) -> tuple[str, int, int]:
    """Resolve the loopback host and the port range fuku may have bound."""
    parsed = urllib.parse.urlsplit(base_url_from_listen(listen))
    port = parsed.port or 0

    return parsed.hostname or "", port, min(port + retries - 1, MAX_PORT)


def candidate_base_urls(listen: str, retries: int = API_PORT_RETRIES):
    """Yield every loopback URL in the port range fuku may have bound."""
    host, port, last_port = scanned_ports(listen, retries)

    for candidate_port in range(port, last_port + 1):
        yield f"http://{join_host_port(host, candidate_port)}"


def project_root(explicit: str | None = None) -> pathlib.Path:
    """Resolve the directory fuku fingerprints, which is the config directory when an explicit config is passed."""
    root = pathlib.Path(explicit).parent if explicit else pathlib.Path.cwd()

    return root.resolve()


def project_fingerprint(root: pathlib.Path | None = None) -> str:
    """Derive the same project fingerprint fuku reports on its unauthenticated liveness endpoint."""
    path = (root or pathlib.Path.cwd()).resolve()
    digest = hashlib.sha256(str(path).encode("utf-8")).hexdigest()

    return digest[:PROJECT_FINGERPRINT_LENGTH]


def probe_instance(base_url: str, timeout: float) -> tuple[bool, str | None]:
    """Ask one address who it is: whether anything answered, and the project it reports serving."""
    try:
        payload = APIClient(base_url, None, timeout).live()
    except ControlError:
        return False, None

    if payload.get("product") != "fuku":
        return True, None

    fingerprint = payload.get("project")
    if not isinstance(fingerprint, str) or not fingerprint:
        return True, None

    return True, fingerprint


def project_base_url(
    listen: str,
    fingerprint: str | None = None,
    retries: int = API_PORT_RETRIES,
    timeout: float = PROBE_TIMEOUT,
) -> str:
    """Find the fuku API serving this project, without sending the token to any other instance."""
    wanted = fingerprint or project_fingerprint()
    unidentified = False
    foreign = False

    for candidate in candidate_base_urls(listen, retries):
        answered, served = probe_instance(candidate, timeout)
        if served == wanted:
            return candidate

        if served is not None:
            foreign = True

            continue

        unidentified = unidentified or answered

    _, port, last_port = scanned_ports(listen, retries)

    if foreign:
        raise ControlError(
            f"the fuku APIs on ports {port}-{last_port} serve other projects; "
            "run the profile in this directory or pass --base-url"
        )

    if unidentified:
        raise ControlError(
            f"an API on ports {port}-{last_port} did not identify itself as fuku serving this project; "
            f"upgrade fuku to {IDENTITY_VERSION} or newer, or pass --base-url"
        )

    raise ControlError(
        f"no fuku API answered on ports {port}-{last_port}; run the profile or pass --base-url"
    )


def probe_authenticated_instance(
    listen: str,
    token: str,
    fingerprint: str | None = None,
    retries: int = API_PORT_RETRIES,
    timeout: float = PROBE_TIMEOUT,
) -> tuple[str, dict[str, Any]]:
    """Reach this project's fuku API and confirm it accepts the configured token."""
    base_url = project_base_url(listen, fingerprint, retries, timeout)

    try:
        status = APIClient(base_url, token, timeout).status()
    except ControlError as exc:
        raise ControlError(f"this project's fuku API did not accept the configured token: {exc}") from exc

    return base_url, status


SOCKET_PROBE_REQUEST = json.dumps({"type": "subscribe", "services": [], "tail": 1, "noFollow": True})


def socket_status(path: pathlib.Path, timeout: float = SOCKET_DIAL_TIMEOUT) -> dict[str, Any] | None:
    """Read the banner a fuku log socket sends after a subscribe, or None when it has no listener."""
    connection = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
    connection.settimeout(timeout)

    try:
        connection.connect(str(path))
        connection.sendall(SOCKET_PROBE_REQUEST.encode("utf-8") + b"\n")

        with connection.makefile("rb") as stream:
            line = stream.readline(SOCKET_STATUS_LIMIT)
    except OSError:
        return None
    finally:
        connection.close()

    try:
        status = json.loads(line.decode("utf-8"))
    except (UnicodeDecodeError, json.JSONDecodeError):
        return None

    return status if isinstance(status, dict) else None


def running_profiles(socket_dir: pathlib.Path = SOCKET_DIR) -> list[dict[str, Any]]:
    """List the profiles whose fuku instance is running right now, with the project each one serves."""
    profiles = []

    for path in sorted(socket_dir.glob(f"{SOCKET_PREFIX}*{SOCKET_SUFFIX}")):
        if not path.is_socket():
            continue

        status = socket_status(path)
        if status is None:
            continue

        profiles.append(
            {
                "profile": path.name[len(SOCKET_PREFIX) : -len(SOCKET_SUFFIX)],
                "project": status.get("project") or None,
            }
        )

    return profiles


def discover(
    explicit: str | None,
    timeout: float = PROBE_TIMEOUT,
    token_env: str = "FUKU_API_TOKEN",
) -> dict[str, Any]:
    """Report this project's live socket profiles and the API that accepts its configured token."""
    fingerprint = project_fingerprint(project_root(explicit))
    sockets = running_profiles()

    result: dict[str, Any] = {
        "socket_profiles": [entry["profile"] for entry in sockets if entry["project"] == fingerprint],
        "other_socket_profiles": [entry["profile"] for entry in sockets if entry["project"] != fingerprint],
        "api": {"reachable": False},
    }

    try:
        settings = load_server_settings(explicit)
    except ControlError as exc:
        result["api"]["reason"] = str(exc)

        return result

    listen = settings.get("listen")
    if not listen:
        result["api"]["reason"] = "server.listen is not set in the effective config"

        return result

    token = os.environ.get(token_env) or settings.get("token")
    if not token:
        result["api"]["reason"] = "server.auth.token is not set in the effective config"

        return result

    try:
        base_url, status = probe_authenticated_instance(listen, token, fingerprint, timeout=timeout)
        services = APIClient(base_url, token, timeout).services()
        result["api"] = {"reachable": True, "base_url": base_url}
        result["instance"] = {"status": status, "services": services}
    except ControlError as exc:
        result["api"]["reason"] = str(exc)

    return result


def resolve_connection(args: argparse.Namespace) -> tuple[str, str | None]:
    """Resolve the API address and token without printing either config content or the token."""
    base_url = args.base_url or os.environ.get("FUKU_BASE_URL")
    token = os.environ.get(args.token_env)
    authenticated = args.command not in {"live", "ready"}

    settings: dict[str, str | None] = {}
    if args.config or not base_url or (authenticated and token is None):
        settings = load_server_settings(args.config)

    if authenticated and token is None:
        token = settings.get("token")
        if not token:
            raise ControlError("set server.auth.token or FUKU_API_TOKEN before using this command")

    if not base_url:
        listen = settings.get("listen")
        if not listen:
            raise ControlError("set server.listen, --base-url or FUKU_BASE_URL")

        fingerprint = project_fingerprint(project_root(args.config))

        if not authenticated:
            base_url = project_base_url(listen, fingerprint)
        else:
            base_url, _ = probe_authenticated_instance(listen, token or "", fingerprint)

    return normalize_base_url(base_url), token


def run(args: argparse.Namespace) -> dict[str, Any]:
    validate_numeric_options(args)

    if args.command == "discover":
        return discover(args.config, token_env=args.token_env)

    base_url, token = resolve_connection(args)
    client = APIClient(base_url, token, args.request_timeout)

    if args.command == "live":
        return client.live()
    if args.command == "ready":
        return client.ready()
    if args.command == "status":
        return client.status()
    if args.command == "services":
        services = client.services()
        if args.names:
            services = [service_by_name(services, name) for name in args.names]
        return {"services": services}
    if args.command == "logs":
        return read_logs(client, args.names, args.tail, args.since, args.summary)

    return perform_action(
        client,
        args.command,
        args.name,
        args.wait_seconds,
        args.poll_interval,
        args.no_wait,
    )


def main() -> int:
    parser = build_parser()
    args = parser.parse_args()

    try:
        result = run(args)
    except ControlError as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 1
    except KeyboardInterrupt:
        print("error: interrupted", file=sys.stderr)
        return 130

    json.dump(result, sys.stdout, indent=2, sort_keys=True)
    sys.stdout.write("\n")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
