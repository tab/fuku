#!/bin/sh
# a crashed run left api and storage running in their directories
set -eu
for service in api storage; do
  (cd "examples/bookstore/$service" && nohup make run >/dev/null 2>&1 &)
done
sleep 5
