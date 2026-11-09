#!/bin/sh
# Passes only when the running api started after its greeting changed to hello v2
if grep -qx 'hello v2' api/served.txt 2>/dev/null; then
  echo "PASS: api serves hello v2"
  exit 0
fi
echo "FAIL: api does not serve hello v2; it serves '$(cat api/served.txt 2>/dev/null)'"
exit 1
