---
description: Restart one service in the running fuku profile
argument-hint: <service>
---

Restart this service: $ARGUMENTS

Use the fuku skill:

1. Resolve the project instance with the `discover` helper, and stop when its authenticated API is not reachable
2. Confirm the exact service name is in the active profile
3. Restart it with the `restart` helper command, which maps the name to its runtime uuid and waits for the final state
4. Report the resulting state and, when the service does not come back, read its recent logs with
   `python3 scripts/control.py logs --tail 50 --since 2m <service>`

Restart only the named service. Do not restart the profile, and do not edit config to enable the API unless asked.

A fuku config change cannot be applied this way, because a running instance keeps the config it loaded at startup. Say so and ask before restarting the whole profile.
