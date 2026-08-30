#!/usr/bin/env bash
# Grades one commit or pull request subject against the CLAUDE.md commit rules
set -euo pipefail

subject="${1:-}"

# Content git writes or rewrites, none of it authored here
case "$subject" in
  Merge\ *|Revert\ *|fixup!*|squash!*|"") exit 0 ;;
esac

# ::error:: makes it an annotation on the pull request; a hook wants the plain line
fail() { if [ -n "${GITHUB_ACTIONS:-}" ]; then echo "::error::$*"; else echo "$*" >&2; fi; }

status=0

if ! grep -qE '^(feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert)\([a-z0-9.-]+\): [A-Z]' <<<"$subject"; then
  fail "needs a scoped Conventional Commit, capitalized after the colon: $subject"
  status=1
fi

if grep -qE '\.$' <<<"$subject"; then
  fail "drop the trailing period: $subject"
  status=1
fi

exit "$status"
