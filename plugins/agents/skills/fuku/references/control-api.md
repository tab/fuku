# fuku control API

Read this before you read service state or start, stop or restart one service
The full contract is the published API reference: https://getfuku.sh/docs/api/

## When the API runs

The API runs while fuku runs, unless the effective config sets `server.listen` to `""` or `none` or `FUKU_API_DISABLED=1` is set
Without `server.listen`, it binds `127.0.0.1:3858`
The token is optional. Without `server.auth.token`, every call works without one
When the port is busy, fuku binds the first free port of the next 9, so up to 10 ports in all
When all 10 are busy, fuku runs without the API

## Find the address

- After `fuku run <profile> -d`, the summary line `API <host:port>` names the bound address
- Otherwise call `GET http://<host:port>/api/v1/live` on the configured port, or 3858 without one, then on the next ports in turn
- Read the configured port with `grep -n '^ *listen:' fuku*.y*ml`, which prints no secret. With `--config`, name that file instead
- `/live` needs no token and answers `{"status", "product", "instance", "fingerprint"}`
- Probe `/live` with plain `curl -s`, never with `api.sh`, because the script sends the token when the config sets one
- Accept an address only when `product` is `fuku` and `fingerprint` is this project's

This project's fingerprint is in `fuku doctor <profile> --json`
`checks["runtime.instance"].details.socket` is `/tmp/fuku-<fingerprint>.sock`, and its 16 hex characters are the fingerprint
fuku derives it from the project directory: the working directory, or the directory of a `--config` path that names one

Never send the token to an address whose `/live` fingerprint is not this project's
Another project's fuku, or any other local server, may hold a port in the range

## The token rule

The token never reaches output: not in a command line, a file, the environment, a log or your reply
Never print, echo or export it, and never read it with `grep`, `sed` or `cat`
Send every other request with `scripts/api.sh`, run from the project root
`<skill>` below is the folder that holds this skill's `SKILL.md`

- `<skill>/scripts/api.sh GET http://127.0.0.1:3858/api/v1/services`
- `<skill>/scripts/api.sh POST http://127.0.0.1:3858/api/v1/services/<id>/restart`
- `<skill>/scripts/api.sh --config <path> GET <url>` when the user gave `--config`

How it behaves:

- It reads `server.auth.token` as fuku does: the override's value when it sets one, else the base config's
- With `--config`, it reads only that file
- An override that sets `server`, `server.auth` or the token to `null` deletes the token, as in fuku
- Without a token, it sends no `Authorization` header
- It pipes the header into `curl` on stdin, so the token is in no command line, environment variable, file or output
- It prints the response body, then the HTTP status on the last line
- It takes only `GET` or `POST`, and only a URL on `http://127.0.0.1:<port>/`, `http://localhost:<port>/` or `http://[::1]:<port>/`
- A config that listens on `ip6-localhost` is called as `http://[::1]:<port>/`. The script reaches no other loopback host
- It sends nothing and prints a one-line reason when it refuses: exit `2` for a bad method or URL, `1` for a token form it cannot read or a missing config
- Otherwise it exits with curl's code, so a `4xx` answer still exits `0`. Read the status line
- fuku takes the token literally and expands no environment variables in it. The script does the same

The script refuses a token it cannot read rather than guess
That covers a block scalar (`|` or `>`), a flow mapping such as `auth: {token: ...}`, a double-quoted value with an escape, an anchor, an alias and a tag
Inside `server`, it also refuses a list, a complex or escaped key and a value continued on the next line
It refuses a merge key at the top level or inside `server`, an indented top level and a second document
It reads plain, single-quoted and double-quoted keys
Report that, and ask the user before you rewrite the token as a plain or quoted scalar
Never read the token another way, and never add `-v` or a trace option to a `curl` that carries it

## The calls

All paths sit under `/api/v1`. When the config sets a token, only `/live` and `/ready` work without it

- `GET /status`: the version, the instance, the project path, the profile, the phase and the service counts
- `GET /services`: every service of the profile with `id`, `name`, `tier`, `status`, `watching`, `pid`, `cpu`, `memory` and `uptime`
- `GET /services/{id}`: one service
- `POST /services/{id}/start`, `POST /services/{id}/stop`, `POST /services/{id}/restart`: one action

Map a service name to its `id` with `GET /services` before any call that takes an `id`
IDs are new on every fuku start. Never reuse one from an earlier run

An action answers `202` with `{"id", "name", "action", "status"}` and returns before the work ends
Poll `GET /services/{id}` until start or restart reaches `running` or `failed`, and stop reaches `stopped`

## When an action is allowed

- Actions open once the profile is resolved, while the phase is `startup` or `running`
- Start needs a service without a live process, so `stopped` or `failed`
- Stop needs a live process
- Restart works in any state once tier startup has reached the service
- A service still waiting for its tier, or with another action or a watch restart in flight, refuses with `409`

Do not retry a refused action in a loop. Read the state, then wait or report it

## `/ready` and `/status`

`GET /ready` answers `200` once the profile is resolved, and `503` before that
It does not mean that every service runs. Use `/status` and `/services` for that

## Errors

The body is `{"error": "<text>"}`

| Status | Text                                | Meaning                                                                         |
|--------|-------------------------------------|---------------------------------------------------------------------------------|
| `401`  | `unauthorized`                      | The config sets a token the request lacks or got wrong. Check the config choice |
| `403`  | `forbidden`                         | A non-loopback `Host` or an `Origin` header, which a browser sends              |
| `404`  | `service not found`                 | The `id` is not in this run. List again                                         |
| `409`  | `service cannot be started`         | Start refused by the rules above                                                |
| `409`  | `service is not running`            | Stop refused by the rules above                                                 |
| `409`  | `service cannot be restarted`       | Restart refused by the rules above                                              |
| `409`  | `instance is not accepting actions` | Before the profile resolves, or while stopping                                  |
| `500`  | `instance is overloaded`            | fuku could not queue the action. Retry once                                     |

The API defines no `400`
In a sandbox, a refused loopback connection or `operation not permitted` is not a `4xx`. Ask for local network access
