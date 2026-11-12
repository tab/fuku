#!/bin/sh
# Logs the wall time each scene mark first shows: "name pane count regex" waits for count matching lines
W=/tmp/fuku-demo
out=$W/out/marks
: > "$out"
grep -v '^#' "$W/marks" | while read -r name pane count pattern; do
  [ -z "$name" ] && continue
  until [ "$(tmux -L demo capture-pane -p -S - -t "demo:0.$pane" 2>/dev/null | grep -Ec -- "$pattern")" -ge "$count" ]; do
    sleep 0.05
  done
  echo "$name $(perl -MTime::HiRes=time -e 'printf "%.2f", time')" >> "$out"
done
