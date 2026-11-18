#!/bin/sh
W=/tmp/fuku-demo
. "$W/case.env"
T="tmux -f $W/tmux.conf -L demo"
$T new-session -d -s demo -x "$(tput cols)" -y "$(tput lines)" "bash --rcfile $W/rc"
# the left pane needs 76 columns, or the TUI metric headers run together
[ "$LAYOUT" = split ] && $T split-window -h -l 40% -t demo "bash --rcfile $W/rc"
if [ "$AGENT" = 1 ]; then
  pane=0
  [ "$LAYOUT" = split ] && pane=1
  $T send-keys -t "demo:0.$pane" claude Enter
  sleep 8
  # the welcome header names the subscription
  $T send-keys -R -t "demo:0.$pane"
  $T clear-history -t "demo:0.$pane"
fi
$T select-pane -t demo:0.0
(nohup "$W/watch.sh" >/dev/null 2>&1 &)
exec $T attach -t demo
