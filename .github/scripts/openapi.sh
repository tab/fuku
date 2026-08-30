#!/usr/bin/env bash
# Fails when an HTTP API change leaves spec/openapi.yaml behind (the docs site and the plugin read it)
set -euo pipefail

base="${1:-origin/master}"
changed="$(git diff --name-only "$base"...HEAD)"
status=0

touched() { grep -qE "$1" <<<"$changed"; }

# Handlers carry the routes and the response shapes; tests and mocks carry neither
contract="$(grep -E '^internal/app/api/.*\.go$' <<<"$changed" \
  | grep -vE '_(test|mock)\.go$' || true)"

if [ -n "$contract" ] && [ -z "${ALLOW_SPEC_DRIFT:-}" ] && ! touched '^spec/openapi\.yaml$'; then
  echo "::error::the HTTP API changed without spec/openapi.yaml"
  while IFS= read -r file; do echo "  $file"; done <<<"$contract"
  echo "  no contract moved? label the pull request contract-unchanged, then re-run this job"
  status=1
fi

# The collection is a hand-kept set of requests, so it can lag a field it never sends
if touched '^spec/openapi\.yaml$' && ! touched '^spec/bruno-collections/'; then
  echo "::warning::spec/openapi.yaml changed without spec/bruno-collections. Check whether a request needs updating"
fi

[ "$status" -eq 0 ] && echo "No spec drift."
exit "$status"
