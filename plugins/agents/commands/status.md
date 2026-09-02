---
description: Show running fuku instances, profile phase and service state
---

Report the current fuku runtime state for this project. Do not start or stop anything.

Use the fuku skill and its `scripts/control.py` helper:

1. Run `discover` to find the API that accepts this project's configured token
2. When the API is reachable, report the phase, profile and every service with its status, pid and uptime
3. When the API is not reachable, report the live socket profiles `discover` attributes to this project, and name the ones serving other directories separately
4. When no project instance is confirmed, state what is missing and name the profiles available in the effective config

Never print a whole config file and never print the API token.
