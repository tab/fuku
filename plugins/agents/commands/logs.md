---
description: Read focused logs from the running fuku profile
argument-hint: [service...]
---

Collect logs for: $ARGUMENTS

When no service is named, read every service, or only the failed ones when some have failed.

Use the fuku skill:

1. Resolve the project instance with the `discover` helper
2. Validate every requested name against the active profile, because an unknown name reports nothing and proves nothing
3. Read the buffered output with `python3 scripts/control.py logs --tail <n> [service...]`
4. When the authenticated API is not reachable, read the same buffer with
   `fuku logs --profile <profile> --tail <n> --no-follow [service...]`, using a profile `discover` reported for this
   project and validating names against the effective config
5. Use `--summary` first when the buffer is noisy, then re-read a smaller `--tail` for the lines that matter
6. Use `--since <duration>` when only output after a restart or a code change is relevant

Never stream logs with a follower process. `fuku logs` without `--no-follow` stays attached, and the helper reads the
same buffer in one bounded request.

Report what the logs show, with short excerpts only where they explain readiness or a failure. Do not paste the raw
buffer into the conversation.
