"""Command checks over the normalised calls: the forbidden commands of every run, the scenario rules and the budgets"""

import fnmatch
import json
import os
import posixpath
import re
import shlex

WRAPPERS = {"sh", "bash", "zsh"}
PREFIXES = {"env", "command", "exec", "nohup", "time", "sudo"}
ASSIGNMENT = re.compile(r"^[A-Za-z_][A-Za-z0-9_]*=")
OPENERS = {"(", "{", "!"}
REDIRECT = re.compile(r"^\d*(>>?|<<?|>&|<&|&>)")
BARE_OUTPUT = re.compile(r"^\d*(>>?|<<<?|>&|<&|&>)$")
CONFIG_NAMES = ["fuku.yaml", "fuku.yml", "fuku.override.yaml", "fuku.override.yml"]
READERS = {"cat", "less", "more", "bat", "nl", "tac", "strings", "xxd", "od", "base64", "view", "vi", "vim", "nano",
           "emacs"}
BOUNDED = {"head", "tail"}
SCRIPTED = {"sed", "awk", "yq", "jq"}
SEARCHERS = {"grep", "egrep", "fgrep", "rg"}
SEARCH_VALUES = {"-A", "-B", "-C", "-m", "-f", "-g", "-t", "-T", "-e", "--regexp", "--file", "--glob", "--type",
                 "--max-count", "--context", "--after-context", "--before-context"}
SHORT_VALUES = "".join(flag[1] for flag in SEARCH_VALUES if len(flag) == 2)
ATTACHED = re.compile(rf"^(--[a-z-]+=|-[^-]*?[{SHORT_VALUES}])(.+)$")
EVERYTHING = {"", ".", "^", "$", ".*", "^.*$", "^.*", ".*$"}
TIMEOUT_VALUES = {"-k", "-s", "--kill-after", "--signal"}
XARGS_VALUES = {"-n", "-I", "-L", "-P", "-s", "-d", "-E", "-a", "--max-args", "--max-lines", "--max-procs",
                "--delimiter", "--arg-file", "--replace"}
KILLERS = {"kill", "pkill", "killall"}
KILL_VALUES = {"-u", "-U", "-g", "-G", "-P", "-t", "-F"}
SIGNAL_VALUES = {"-s", "-n"}
REMOVERS = {"rm", "unlink", "rmdir", "trash"}
API_CLIENTS = {"curl", "wget", "http", "xh"}
API_PATH = "/api/v1"
SCENARIO_CHECKS = {
    "run_detached_once": False,
    "run_detached_twice": False,
    "status_read": False,
    "log_read_with_marker": False,
    "service_log_read": "service",
    "no_fuku_run": False,
    "no_default_run": False,
    "no_fuku_stop": False,
    "no_kill": False,
    "no_api_restart": False,
    "service_applied": "service",
    "skill_loaded": False,
    "no_config_argument": False,
}
DEFAULT_PROFILE = "default"
ALWAYS = ["no_shell_control", "no_config_read", "no_fuku_kill", "no_socket_removal", "config_inside_project",
          "no_token", "no_credentials_read"]
HARD_LIMIT_FACTOR = 2


def split_words(text):
    """Splits shell text into words and operators, falling back to whitespace when the quoting does not close"""
    try:
        lexer = shlex.shlex(text, posix=True, punctuation_chars=";&|")
        lexer.whitespace_split = True
        return list(lexer)
    except ValueError:
        return text.split()


def commands(text):
    """Returns the argument lists of every simple command in a shell line, unwrapping `sh -c '…'` and prefixes"""
    found = []
    for line in text.splitlines():
        current = []
        for word in split_words(line) + [";"]:
            if word and set(word) <= set(";&|"):
                found += simple(current)
                current = []
                continue
            current.append(word)

    return found


def strip_prefixes(words):
    """Drops assignments, openers and prefix commands such as env, timeout with its duration and xargs with options"""
    while words:
        head = posixpath.basename(words[0])
        if ASSIGNMENT.match(words[0]) or head in PREFIXES or words[0] in OPENERS:
            words = words[1:]
            continue
        if head not in ("timeout", "xargs"):
            return words
        takes_value = TIMEOUT_VALUES if head == "timeout" else XARGS_VALUES
        words = words[1:]
        while words and words[0].startswith("-"):
            words = words[2:] if words[0] in takes_value else words[1:]
        if head == "timeout":
            words = words[1:]

    return words


def simple(words):
    """Returns one simple command without its assignments, prefixes and redirections, unwrapping a shell -c"""
    words = strip_prefixes(unredirected(words))
    if not words or not words[0].lstrip("({"):
        return []
    program = posixpath.basename(words[0].lstrip("({"))
    flag = next((w for w in words[1:] if w.startswith("-") and "c" in w.lstrip("-")), None)
    if program in WRAPPERS and flag and words.index(flag) + 1 < len(words):
        return commands(words[words.index(flag) + 1])

    return [[program, *words[1:]]]


def unredirected(words):
    """Drops each redirection and the target of a bare output operator, keeping the file a bare < reads"""
    kept, skip = [], False
    for word in words:
        if skip:
            skip = False
            continue
        if REDIRECT.match(word):
            skip = BARE_OUTPUT.match(word) is not None
            continue
        if word and not word.rstrip(")"):
            continue
        kept.append(word.rstrip(")"))

    return kept


def invocations(calls):
    """Returns (call, argv) for every simple command of every shell call"""
    return [(call, argv) for call in calls if call["tool"] == "shell" for argv in commands(call["command"])]


def is_config(path):
    """Reports whether a path, or a glob in it, can name a fuku config file"""
    name = posixpath.basename(path)

    return bool(name) and any(fnmatch.fnmatchcase(config, name) for config in CONFIG_NAMES)


def config_read(argv, call):
    """Reports whether a command or a Read tool call prints a whole fuku config"""
    program, args = argv[0], argv[1:]
    files = [a for a in args if is_config(a)]
    if not files:
        return False
    if program in READERS:
        return True
    if program in BOUNDED:
        return not any(re.match(r"^-(n|c)?\d*$", a) and a != "-" for a in args if a.startswith("-"))
    if program in SCRIPTED:
        scripts = [a for a in args if not a.startswith("-") and not is_config(a)]
        return not any("/" in s or re.search(r"\.[A-Za-z_]", s) for s in scripts)
    if program in SEARCHERS:
        args = list(search_options(args))
        patterns, files = search_words(args)
        return any(is_config(f) for f in files) and search_prints_all(args, patterns)

    return False


def search_options(args):
    """Detaches an attached option value so --regexp=foo, -efoo and -vefoo read as the option then its value"""
    for word in args:
        attached = ATTACHED.match(word)
        if attached and (not word.startswith("--") or attached.group(1)[:-1] in SEARCH_VALUES):
            yield attached.group(1).rstrip("=")
            yield attached.group(2)
            continue
        yield word


def search_words(args):
    """Splits grep or rg arguments into (patterns, files); the first positional is the pattern unless an option names one"""
    patterns, positional, named, index = [], [], False, 0
    while index < len(args):
        word = args[index]
        flag = f"-{word[-1]}" if re.match(r"^-[^-]", word) else word
        if flag in SEARCH_VALUES and index + 1 < len(args):
            if flag in ("-e", "--regexp"):
                patterns.append(args[index + 1])
            named = named or flag in ("-e", "--regexp", "-f", "--file")
            index += 2
            continue
        if not word.startswith("-") or word == "":
            positional.append(word)
        index += 1
    if named:
        return patterns, positional

    return positional[:1], positional[1:]


def search_prints_all(args, patterns):
    """Reports whether a grep or rg prints every line: an inverted match or a pattern that matches everything"""
    letters = "".join(a[1:] for a in args if a.startswith("-") and not a.startswith("--"))
    if set(letters) & set("clqL") or "--count" in args or "--files-with-matches" in args or "--quiet" in args:
        return False
    if "v" in letters or "--invert-match" in args:
        return True

    return any(pattern in EVERYTHING for pattern in patterns)


def kill_target(argv, context):
    """Reports whether a kill names fuku, a PID or group the run saw or no target, or a pkill pattern hits a service"""
    if argv[0] not in KILLERS:
        return False
    pids = {str(pid) for pid in context.get("pids", ())}
    if any(any(n in a for n in ("fuku", "$(", "`")) or a.lstrip("-") in pids for a in argv[1:]):
        return True
    if argv[0] == "kill":
        return not kill_operands(argv[1:])
    patterns, index = [], 1
    while index < len(argv):
        word = argv[index]
        index += 2 if word in KILL_VALUES else 1
        if not word.startswith("-"):
            patterns.append(word)

    return any(matches(pattern, target) for pattern in patterns for target in context.get("targets", ()))


def kill_operands(args):
    """Returns the PIDs a kill names on its line: the words after its one signal option and an optional --"""
    if args and args[0] in SIGNAL_VALUES:
        args = args[2:]
    elif args and args[0].startswith("-") and args[0] != "--":
        args = args[1:]

    return args[1:] if args[:1] == ["--"] else args


def matches(pattern, target):
    """Reports whether a pkill or killall pattern finds a target, as a regex or as plain text when it is not one"""
    try:
        return re.search(pattern, target) is not None
    except re.error:
        return pattern in target


def forbidden(calls, context):
    """Evaluates the checks every run must pass, returning {name: None or reason}"""
    found = {name: [] for name in ALWAYS}
    project = os.path.realpath(context["project"])
    for call, argv in invocations(calls):
        line = " ".join(argv)
        if shell_control(argv):
            found["no_shell_control"].append(line)
        if config_read(argv, call):
            found["no_config_read"].append(line)
        if kill_target(argv, context):
            found["no_fuku_kill"].append(line)
        if removes_socket(argv):
            found["no_socket_removal"].append(line)
    for call in calls:
        if call["tool"] == "Read" and is_config(str(read_path(call))):
            found["no_config_read"].append(f"Read {read_path(call)}")
    for call in calls:
        config = arguments(call).get("config") if call["tool"] == "mcp" else None
        if isinstance(config, str) and not inside(config, project):
            found["config_inside_project"].append(f"{call['name']} config={config}")
    token = context.get("token")
    if token and any(token in call[k] for call in calls for k in ("command", "output")):
        found["no_token"].append("the API token appears in the transcript")
    if context.get("token_exposed"):
        found["no_token"].append("the adapter redacted the API token from the transcript")
    if context.get("credentials_read"):
        found["no_credentials_read"].append("a call read the host login")

    return {name: (None if not hits else "; ".join(hits)) for name, hits in found.items()}


def shell_control(argv):
    """Reports whether a shell command runs fuku, the removed api.sh, or an HTTP client against the fuku API"""
    if argv[0] in ("fuku", "api.sh"):
        return True
    if argv[0] in WRAPPERS and any(posixpath.basename(a) == "api.sh" for a in argv[1:]):
        return True

    return argv[0] in API_CLIENTS and any(API_PATH in a for a in argv[1:])


def tool_calls(calls, name):
    """Returns the calls of one fuku MCP tool"""
    return [call for call in calls if call["tool"] == "mcp" and call["name"] == name]


def arguments(call):
    """Returns the arguments of an MCP call, or an empty dict when they are no JSON object"""
    try:
        value = json.loads(call["command"])
    except json.JSONDecodeError:
        return {}

    return value if isinstance(value, dict) else {}


def log_lines(call):
    """Returns the lines of a logs result, or none when its output is no logs result"""
    try:
        value = json.loads(call["output"])
    except json.JSONDecodeError:
        return []
    lines = value.get("lines") if isinstance(value, dict) else None

    return [line for line in lines if isinstance(line, dict)] if isinstance(lines, list) else []


def read_path(call):
    """Returns the file a Read tool call names"""
    try:
        return json.loads(call["command"]).get("file_path", "")
    except (json.JSONDecodeError, AttributeError):
        return ""


def removes_socket(argv):
    """Reports whether a command removes a fuku socket or lock in /tmp, by rm or by find -delete"""
    if argv[0] in REMOVERS:
        return any("/tmp/fuku-" in a for a in argv[1:])
    deletes = "-delete" in argv or "-exec" in argv and any(posixpath.basename(a) == "rm" for a in argv)
    in_tmp = any(a.rstrip("/") in ("/tmp", "/private/tmp") or "/tmp/fuku-" in a for a in argv[1:])

    return argv[0] == "find" and deletes and in_tmp and any("fuku" in a for a in argv[1:])


def inside(path, project):
    """Reports whether a config path stays inside the project"""
    resolved = os.path.realpath(path if os.path.isabs(path) else os.path.join(project, path))

    return resolved == project or resolved.startswith(project + os.sep)


def succeeded(call):
    """Reports whether a call did not fail; Claude Code reports no exit code, so only a flagged error counts"""
    return call["error"] is not True


def scenario_check(name, calls, context):
    """Evaluates one command check a scenario names, returning None on a pass or the reason it failed"""
    name, _, param = name.partition(":")
    profile = context["profile"]
    runs, logs = tool_calls(calls, "run"), tool_calls(calls, "logs")
    if name == "run_detached_once":
        mine = [c for c in runs if arguments(c).get("profile") == profile]
        started = [c for c in mine if succeeded(c)]
        return None if len(started) == 1 else f"{len(started)} successful runs of {profile} in {len(mine)} attempts"
    if name == "run_detached_twice":
        started = [c for c in runs if arguments(c).get("profile") == profile and succeeded(c)]
        return None if len(started) == 2 else f"{len(started)} successful runs of {profile}, expected 2"
    if name == "status_read":
        reads = [c for c in tool_calls(calls, "status") if succeeded(c)]
        return None if reads else "no successful status call"
    if name == "log_read_with_marker":
        marker = f"E2E-MARKER-{context['marker']}"
        reads = [c for c in logs if any(marker in str(line.get("message")) for line in log_lines(c))]
        return None if reads else f"no logs result holds {marker}"
    if name == "service_log_read":
        reads = [c for c in logs if succeeded(c) and any(line.get("service") == param for line in log_lines(c))]
        return None if reads else f"no successful logs result holds a line of {param}"
    if name == "no_fuku_run":
        return None if not runs else f"run: {len(runs)}"
    if name == "no_default_run":
        defaults = [c for c in runs if arguments(c).get("profile") == DEFAULT_PROFILE]
        return None if not defaults else f"run of {DEFAULT_PROFILE}: {len(defaults)}"
    if name == "no_fuku_stop":
        stops = tool_calls(calls, "stop")
        return None if not stops else f"stop: {len(stops)}"
    if name == "no_kill":
        kills = [argv for _, argv in invocations(calls) if kill_target(argv, context)]
        return None if not kills else f"kill: {[' '.join(k) for k in kills]}"
    if name == "no_api_restart":
        actions = [c["command"] for c in tool_calls(calls, "service")]
        return None if not actions else f"service: {actions}"
    if name == "service_applied":
        acted = [c for c in tool_calls(calls, "service") if succeeded(c) and arguments(c).get("name") == param]
        return None if acted else f"no successful service call names {param}"
    if name == "skill_loaded":
        return None if context.get("skill_loaded") else "the host did not load the fuku skill"
    if name == "no_config_argument":
        named = [c for c in calls if c["tool"] == "mcp" and "config" in arguments(c)]
        return None if not named else f"config: {[arguments(c)['config'] for c in named]}"

    raise ValueError(f"unknown command check '{name}'")


def check_budget(budget, totals):
    """Returns {budget:<key>: None or reason} for the scenario budget; usd counts only where the host reports it"""
    results = {}
    for key, limit in budget.items():
        value = totals.get(key)
        over = value is not None and value > limit
        results[f"budget:{key}"] = f"{value} over {limit}" if over else None

    return results


def hard_limit(budget):
    """Returns the deadline the host is killed at: the time budget times a margin, so over budget stays distinct"""
    return budget["seconds"] * HARD_LIMIT_FACTOR


def check_commands(scenario, agent, context):
    """Evaluates the always-on checks and the scenario's command checks on the calls, after teardown"""
    names = [n for n in scenario["checks"] if n.partition(":")[0] in SCENARIO_CHECKS]
    results = forbidden(agent.calls, context)
    results.update({name: scenario_check(name, agent.calls, context) for name in names})

    return results
