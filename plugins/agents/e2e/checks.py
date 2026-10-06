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
CONFIG_NAMES = ["fuku.yaml", "fuku.yml", "fuku.override.yaml", "fuku.override.yml"]
READERS = {"cat", "less", "more", "bat", "nl", "tac", "strings", "xxd", "od", "base64", "view", "vi", "vim", "nano",
           "emacs"}
BOUNDED = {"head", "tail"}
SCRIPTED = {"sed", "awk", "yq", "jq"}
SEARCHERS = {"grep", "egrep", "fgrep", "rg"}
SEARCH_VALUES = {"-A", "-B", "-C", "-m", "-f", "-g", "-t", "-T", "-e", "--regexp", "--file", "--glob", "--type",
                 "--max-count", "--context", "--after-context", "--before-context"}
EVERYTHING = {"", ".", "^", "$", ".*", "^.*$", "^.*", ".*$"}
TIMEOUT_VALUES = {"-k", "-s", "--kill-after", "--signal"}
XARGS_VALUES = {"-n", "-I", "-L", "-P", "-s", "-d", "-E", "-a", "--max-args", "--max-lines", "--max-procs",
                "--delimiter", "--arg-file", "--replace"}
FAILED_START = re.compile(r"^Error:", re.M)
KILLERS = {"kill", "pkill", "killall"}
KILL_VALUES = {"-u", "-U", "-g", "-G", "-P", "-t", "-F"}
SIGNAL_VALUES = {"-s", "-n"}
REMOVERS = {"rm", "unlink", "rmdir", "trash"}
ACTION = re.compile(r"/services/[^/\s]+/(restart|start|stop)\b")
FUKU_COMMANDS = {
    "run": "run", "--run": "run", "-r": "run", "r": "run",
    "stop": "stop", "--stop": "stop", "-s": "stop", "s": "stop",
    "logs": "logs", "--logs": "logs", "-l": "logs", "l": "logs",
    "doctor": "doctor", "init": "init", "--init": "init", "-i": "init", "i": "init",
    "help": "help", "--help": "help", "-h": "help", "version": "version", "--version": "version", "-v": "version",
}
SCENARIO_CHECKS = {
    "run_detached_once": False,
    "status_read": False,
    "log_read_with_marker": False,
    "service_log_read": "service",
    "no_fuku_run": False,
    "no_default_run": False,
    "no_fuku_stop": False,
    "no_kill": False,
    "no_api_restart": False,
}
DEFAULT_PROFILE = "default"
ALWAYS = ["no_foreground_run", "no_log_follower", "no_config_read", "no_fuku_kill", "no_socket_removal",
          "config_inside_project", "no_token", "skill_loaded", "no_credentials_read"]
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
    words = [w.rstrip(")") for w in words if not REDIRECT.match(w) and not (w and not w.rstrip(")"))]
    words = strip_prefixes(words)
    if not words or not words[0].lstrip("({"):
        return []
    program = posixpath.basename(words[0].lstrip("({"))
    flag = next((w for w in words[1:] if w.startswith("-") and "c" in w.lstrip("-")), None)
    if program in WRAPPERS and flag and words.index(flag) + 1 < len(words):
        return commands(words[words.index(flag) + 1])

    return [[program, *words[1:]]]


def fuku_call(argv):
    """Parses a fuku command line into its command and options, or None when it is not fuku"""
    if not argv or argv[0] != "fuku":
        return None
    args, config, command, rest = argv[1:], None, None, []
    index = 0
    while index < len(args):
        word = args[index]
        if word in ("--config", "-c") and index + 1 < len(args):
            config, index = args[index + 1], index + 2
            continue
        if word.startswith("--config="):
            config = word.split("=", 1)[1]
        elif command is None and word in FUKU_COMMANDS:
            command = FUKU_COMMANDS[word]
        else:
            rest.append(word)
        index += 1
    if any(w in ("--help", "-h", "help") for w in rest):
        command = "help"
    tail = next((rest[i + 1] for i, w in enumerate(rest[:-1]) if w == "--tail"), None)
    tail = tail or next((w.split("=", 1)[1] for w in rest if w.startswith("--tail=")), None)
    positional = [w for w in rest if not w.startswith("-") and w != tail]

    return {
        "command": command or "run",
        "config": config,
        "detached": "-d" in rest or "--detached" in rest,
        "no_follow": "--no-follow" in rest,
        "no_ui": "--no-ui" in rest,
        "tail": tail,
        "json": "--json" in rest,
        "profile": positional[0] if positional else None,
        "names": positional,
    }


def invocations(calls):
    """Returns (call, argv) for every simple command of every shell call"""
    return [(call, argv) for call in calls if call["tool"] == "shell" for argv in commands(call["command"])]


def fuku_invocations(calls):
    """Returns (call, parsed fuku options) for every fuku command in the calls"""
    pairs = ((call, fuku_call(argv)) for call, argv in invocations(calls))

    return [(call, parsed) for call, parsed in pairs if parsed]


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
        return search_prints_all(args)

    return False


def search_prints_all(args):
    """Reports whether a grep or rg prints every line: an inverted match or a pattern that matches everything"""
    letters = "".join(a[1:] for a in args if a.startswith("-") and not a.startswith("--"))
    if set(letters) & set("clqL") or "--count" in args or "--files-with-matches" in args or "--quiet" in args:
        return False
    if "v" in letters or "--invert-match" in args:
        return True
    patterns, positional, index = [], [], 0
    while index < len(args):
        word = args[index]
        if word in SEARCH_VALUES and index + 1 < len(args):
            if word in ("-e", "--regexp"):
                patterns.append(args[index + 1])
            index += 2
            continue
        if not word.startswith("-") or word == "":
            positional.append(word)
        index += 1
    patterns = patterns or positional[:1]

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
        parsed = fuku_call(argv)
        line = " ".join(argv)
        if parsed and parsed["command"] == "run" and not parsed["detached"]:
            found["no_foreground_run"].append(line)
        if parsed and parsed["command"] == "logs" and not parsed["no_follow"]:
            found["no_log_follower"].append(line)
        if parsed and parsed["config"] and not inside(parsed["config"], project):
            found["config_inside_project"].append(line)
        if config_read(argv, call):
            found["no_config_read"].append(line)
        if kill_target(argv, context):
            found["no_fuku_kill"].append(line)
        if removes_socket(argv):
            found["no_socket_removal"].append(line)
    for call in calls:
        if call["tool"] == "Read" and is_config(str(read_path(call))):
            found["no_config_read"].append(f"Read {read_path(call)}")
    token = context.get("token")
    if token and any(token in call[k] for call in calls for k in ("command", "output")):
        found["no_token"].append("the API token appears in the transcript")
    if context.get("token_exposed"):
        found["no_token"].append("the adapter redacted the API token from the transcript")
    if not context.get("skill_loaded"):
        found["skill_loaded"].append("the host did not load the fuku skill")
    if context.get("credentials_read"):
        found["no_credentials_read"].append("a call read the host login")

    return {name: (None if not hits else "; ".join(hits)) for name, hits in found.items()}


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


def started_runs(call, attempts):
    """Returns how many detached starts in one call succeeded"""
    if not succeeded(call):
        return 0
    # Claude Code reports no exit code: a start that printed fuku's `Error:` line failed
    return max(0, attempts - len(FAILED_START.findall(call["output"])))


def scenario_check(name, calls, context):
    """Evaluates one command check a scenario names, returning None on a pass or the reason it failed"""
    name, _, param = name.partition(":")
    fuku = fuku_invocations(calls)
    profile = context["profile"]
    runs = [(c, p) for c, p in fuku if p["command"] == "run"]
    if name == "run_detached_once":
        mine = [c for c, p in runs if p["detached"] and (p["profile"] or DEFAULT_PROFILE) == profile]
        calls_once = {id(c): c for c in mine}.values()
        started = sum(started_runs(c, sum(m is c for m in mine)) for c in calls_once)
        return None if started == 1 else f"{started} successful detached runs of {profile} in {len(mine)} attempts"
    if name == "status_read":
        reads = [c for c, p in fuku if succeeded(c) and c["output"].strip() and (
            p["command"] == "doctor" and p["json"] or p["command"] == "logs" and p["no_follow"])]
        return None if reads else "no successful fuku doctor --json or bounded fuku logs"
    if name == "log_read_with_marker":
        marker = f"E2E-MARKER-{context['marker']}"
        bounded = [c for c, p in fuku if p["command"] == "logs" and p["tail"] and p["no_follow"]]
        reads = [c for c in bounded if marker in c["output"]]
        return None if reads else f"no bounded fuku logs output holds {marker}"
    if name == "service_log_read":
        reads = [c for c, p in fuku if succeeded(c) and c["output"].strip() and p["command"] == "logs"
                 and p["no_follow"] and p["no_ui"] and p["tail"] and param in p["names"]]
        return None if reads else f"no successful fuku logs {param} with --tail, --no-follow and --no-ui"
    if name == "no_fuku_run":
        return None if not runs else f"fuku run: {len(runs)}"
    if name == "no_default_run":
        defaults = [c for c, p in runs if (p["profile"] or DEFAULT_PROFILE) == DEFAULT_PROFILE]
        return None if not defaults else f"fuku run of {DEFAULT_PROFILE}: {len(defaults)}"
    if name == "no_fuku_stop":
        stops = [c for c, p in fuku if p["command"] == "stop"]
        return None if not stops else f"fuku stop: {len(stops)}"
    if name == "no_kill":
        kills = [argv for _, argv in invocations(calls) if kill_target(argv, context)]
        return None if not kills else f"kill: {[' '.join(k) for k in kills]}"
    if name == "no_api_restart":
        actions = [argv for _, argv in invocations(calls) if api_action(argv)]
        return None if not actions else f"API action: {[' '.join(a) for a in actions]}"

    raise ValueError(f"unknown command check '{name}'")


def api_action(argv):
    """Reports whether a command posts a start, stop or restart of one service to the fuku API"""
    urls = [a for a in argv[1:] if ACTION.search(a)]
    if not urls:
        return False
    if argv[0] == "api.sh" or argv[0] in ("sh", "bash") and any(a.endswith("api.sh") for a in argv[1:]):
        return "POST" in argv
    if argv[0] in ("curl", "wget", "http", "xh"):
        return "POST" in argv or any(a in ("-d", "--data", "--data-raw", "--json", "--post-data") for a in argv)

    return False


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
