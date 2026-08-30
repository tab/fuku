#!/usr/bin/env bash
# Rejects AI attribution in a commit message file, or in the text on stdin
set -euo pipefail

# Matched on the attribution forms, not the bare word, so `docs(claude):` still passes
attribution='^[[:space:]]*co-authored-by:.*(claude|anthropic|copilot|chatgpt|codex|cursor)|^[[:space:]]*claude-session:|generated (with|by) .*(claude|anthropic|copilot|chatgpt|codex|cursor)|https://claude\.(ai|com)/|🤖'

hits="$(grep -vE '^#' "${1:--}" | grep -inE "$attribution" || true)"

[ -z "$hits" ] && exit 0

# ::error:: makes it an annotation on the pull request; a hook wants the plain line
if [ -n "${GITHUB_ACTIONS:-}" ]; then
  echo "::error::remove the AI tooling attribution"
else
  echo "remove the AI tooling attribution" >&2
fi

sed 's/^/  /' <<<"$hits"
exit 1
