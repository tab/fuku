"""Claude Code adapter: runs `claude -p` headless with the working-tree plugin and reads its stream-json events"""

import json
import os
import pathlib
import subprocess

import hosts

EXECUTABLE = "claude"
MIN_VERSION = (2, 1, 319)
VERSION_SECONDS = 30
TOOLS = ["Bash", "Read", "Skill"]
FUKU_SERVER = "mcp__plugin_fuku_fuku"
FUKU_TOOL = FUKU_SERVER + "__"
FUKU_SERVER_NAME = "plugin:fuku:fuku"
NO_CONNECTORS = {"ENABLE_CLAUDEAI_MCP_SERVERS": "false"}
UNAVAILABLE = {"failed", "needs-auth"}
MAX_TURNS = 30
EVENTS = {"system", "assistant", "user", "result", "rate_limit_event"}
SKILL = "fuku:fuku-config"
SKILLS = {"fuku:fuku", SKILL}
LOGIN_SOURCES = [r"\.credentials\.json", r"\.claude\.json", r"find-generic-password", r"Claude Code-credentials"]


def command(prompt, plugin_dir, usd):
    """Returns the headless command line: the working-tree plugin with its MCP server, no user settings, no prompts"""
    return [
        EXECUTABLE, "-p", prompt,
        "--plugin-dir", str(plugin_dir),
        "--output-format", "stream-json", "--verbose",
        "--setting-sources", "project",
        "--tools", ",".join(TOOLS),
        "--allowed-tools", *TOOLS, FUKU_SERVER,
        "--permission-mode", "dontAsk",
        "--max-budget-usd", str(usd),
        "--max-turns", str(MAX_TURNS),
        "--no-session-persistence",
    ]


def text_of(content):
    """Returns a tool result's content as text, joined from its text blocks when it is a list"""
    if isinstance(content, str):
        return content

    return "".join(block.get("text", "") for block in content or [] if isinstance(block, dict))


def parse(lines, plugin_dir):
    """Reads stream-json lines into the normalised result; the skill counts only when it came from plugin_dir"""
    init, final, calls, by_id, last_text = None, None, [], {}, ""
    for number, line in enumerate(lines, 1):
        if not line.strip():
            continue
        try:
            event = json.loads(line)
            kind = event["type"]
        except (json.JSONDecodeError, TypeError, KeyError) as err:
            raise hosts.ParseError(f"line {number}: not a Claude Code event", line) from err
        if kind not in EVENTS:
            raise hosts.ParseError(f"line {number}: unknown Claude Code event type '{kind}'")
        if kind == "system" and event.get("subtype") == "init":
            init = event
        if kind == "result":
            final = event
        if kind not in ("assistant", "user") or not isinstance(event.get("message", {}).get("content"), list):
            continue
        for block in event["message"]["content"]:
            if kind == "assistant" and block.get("type") == "text":
                last_text = block.get("text", "")
            if block.get("type") == "tool_use":
                tool_input = block.get("input") or {}
                name = block.get("name") or ""
                shell, fuku = name == "Bash", name.startswith(FUKU_TOOL)
                call = {
                    "tool": "shell" if shell else "mcp" if fuku else name,
                    "command": tool_input.get("command", "") if shell else json.dumps(tool_input, sort_keys=True),
                    "output": "",
                    "error": None,
                }
                if fuku:
                    call["name"] = name.removeprefix(FUKU_TOOL)
                by_id[block.get("id")] = call
                calls.append(call)
            if block.get("type") == "tool_result" and block.get("tool_use_id") in by_id:
                call = by_id[block["tool_use_id"]]
                call["output"] = text_of(block.get("content"))
                call["error"] = bool(block.get("is_error"))

    if init is None:
        raise hosts.ParseError("no system/init event")
    plugins = [p for p in init.get("plugins") or [] if p.get("name") == "fuku"]
    plugin = plugins[0] if len(plugins) == 1 else {}
    from_tree = plugin.get("path") == str(plugin_dir)
    loads = [c for c in calls if c["tool"] == "Skill" and json.loads(c["command"]).get("skill") in SKILLS
             and c["error"] is False]
    invoked = any(json.loads(c["command"]).get("skill") == SKILL for c in loads)
    usage = (final or {}).get("usage") or {}
    totals = {
        "tool_calls": len(calls) - len({json.loads(c["command"]).get("skill") for c in loads}),
        "turns": (final or {}).get("num_turns"),
        "input_tokens": sum(usage.get(k, 0) for k in
                            ("input_tokens", "cache_creation_input_tokens", "cache_read_input_tokens")),
        "cached_input_tokens": usage.get("cache_read_input_tokens", 0),
        "output_tokens": usage.get("output_tokens", 0),
        "usd": (final or {}).get("total_cost_usd"),
        "model": init.get("model"),
        "host_version": init.get("claude_code_version"),
        "plugin_path": plugin.get("path"),
        "outcome": (final or {}).get("subtype"),
    }
    answer = (final or {}).get("result")
    answer = answer if isinstance(answer, str) else last_text

    result = hosts.AgentResult(calls, totals, from_tree and invoked, answer=answer)
    if len(plugins) > 1:
        result.fail("isolation", f"{len(plugins)} plugins named fuku: {[p.get('path') for p in plugins]}")
    others = [s.get("name") for s in init.get("mcp_servers") or [] if s.get("name") != FUKU_SERVER_NAME]
    if others:
        result.fail("isolation", f"MCP servers other than fuku: {others}")
    down = [s.get("status") for s in init.get("mcp_servers") or []
            if s.get("name") == FUKU_SERVER_NAME and s.get("status") in UNAVAILABLE]
    if down:
        result.fail("mcp", f"the fuku MCP server is {down[0]}, so the agent never had its tools")
    kind, message = host_failure(final)
    if kind:
        result.fail(kind, message)

    return result


def host_failure(final):
    """Returns the kind and reason of a host failure from the result event, or (None, None) for an answer or a cap"""
    if final is None:
        return "crash", "no result event"
    text = str(final.get("result"))
    if final.get("is_error") and "login" in text.lower():
        return "login", f"not logged in: {text}"
    if final.get("api_error_status") or final.get("terminal_reason") == "api_error":
        return "api", f"API error {final.get('api_error_status')}: {text}"
    if final.get("subtype") == "error_during_execution":
        return "crash", f"error during execution: {text}"
    if final.get("is_error") and final.get("subtype") == "success":
        return "api", f"host error: {text}"

    return None, None


def version(env):
    """Returns what `claude --version` prints, bounded and without stdin"""
    try:
        return subprocess.run([EXECUTABLE, "--version"], env=env, stdin=subprocess.DEVNULL, capture_output=True,
                              text=True, timeout=VERSION_SECONDS, check=False).stdout
    except (OSError, subprocess.TimeoutExpired) as err:
        raise hosts.VersionError(f"{EXECUTABLE} --version: {err}") from err


class ClaudeAdapter:
    """Runs one scenario prompt in Claude Code with the plugin from the working tree"""

    name = "claude"

    def __init__(self, plugin_dir, environ=None):
        self.plugin_dir = pathlib.Path(plugin_dir)
        self.environ = dict(os.environ if environ is None else environ)

    def __enter__(self):
        hosts.require_version(EXECUTABLE, version(hosts.host_env(self.environ, {})[0]), MIN_VERSION)
        return self

    def __exit__(self, *_):
        return False

    def run(self, project, prompt, env, budget=None, timeout=None, secrets=()):
        usd = (budget or {}).get("usd", 1)
        run_env, passed = hosts.host_env(env, NO_CONNECTORS)
        host = hosts.run_host(command(prompt, self.plugin_dir, usd), project, run_env, timeout)
        try:
            result = parse(hosts.complete_lines(host), self.plugin_dir)
        except hosts.ParseError as err:
            result = hosts.AgentResult()
            result.fail("unreadable", f"unreadable events: {err.describe([*secrets, *passed])}")
        if host.timed_out:
            result.host_error, result.host_error_kind, result.timed_out = None, None, True
        crashed = result.host_error is not None or result.totals.get("outcome") is None
        if host.returncode not in (0, None) and crashed:
            if result.host_error_kind == "unreadable":
                result.host_error_kind = None
            stderr = hosts.scrub(host.stderr.strip(), hosts.secret_values([*secrets, *passed]))
            result.fail("crash", f"claude exited {host.returncode}: {stderr[:300]}")
        if host.left_behind:
            result.fail("leftover", "host process left behind")
        result.totals["seconds"] = host.seconds
        result.credentials_read = hosts.mark_credential_reads(result, LOGIN_SOURCES)
        result.token_exposed = hosts.redact(result, secrets, host.stdout)
        hosts.redact(result, passed)

        return result
