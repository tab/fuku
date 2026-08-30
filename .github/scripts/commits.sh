#!/usr/bin/env bash
# Grades every commit in a range: subject format, then AI attribution over the whole message
set -euo pipefail

base="$1"
root="$(git rev-parse --show-toplevel)"
status=0

# Merge commits carry a subject git wrote, and merging master in is normal here
while IFS= read -r subject; do
  "$root/.github/scripts/subject.sh" "$subject" || status=1
done < <(git log --no-merges --format='%s' "$base..HEAD")

git log --format='%H %s%n%b' "$base..HEAD" | "$root/.github/scripts/attribution.sh" - || status=1

exit "$status"
