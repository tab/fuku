#!/usr/bin/env bash
# Fails when ARCHITECTURE.md, the backlog, a feature document or a package README links to a file or directory that does not exist
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

status=0

while IFS= read -r file; do
  dir="$(dirname "$file")"

  # every markdown link target that is not a URL or a bare anchor, with its own anchor stripped
  while IFS= read -r target; do
    [ -e "$dir/$target" ] && continue

    echo "$file: broken link $target"
    status=1
  done < <(grep -oE '\]\([^)]+\)' "$file" | sed -E 's/^\]\(//; s/\)$//; s/#.*$//' | grep -vE '^(https?:|mailto:|$)' | sort -u)
done < <(printf '%s\n' ARCHITECTURE.md docs/features/backlog.md; find docs/features internal \( -name feature.md -o -name plan.md -o -name README.md \) | sort)

[ "$status" -eq 0 ] && echo "All links resolve."
exit "$status"
