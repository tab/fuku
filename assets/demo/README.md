# assets/demo

The recorded demos of the site's `/demo/` pages. Each case is a real fuku session on the bookstore example, recorded with VHS.

## Recording a case

```sh
assets/demo/record.sh agent-fix            # writes docs/public/demo/agent-fix.mp4 and .jpg
assets/demo/record.sh agent-fix --prepare  # sets up /tmp/fuku-demo and stops, for a rehearsal
assets/demo/gif.sh agent-hot-reload        # writes assets/demo.gif, the README demo, from a recorded case
```

It needs `vhs`, `tmux`, `ffmpeg`, `python3` and Go. An agent case also needs a logged-in `claude`.
An agent case spends real tokens and differs on every take. Watch the result before you commit it.

## How it works

1. `record.sh` builds fuku with the version in `VERSION` (default: the coming release) through a `go build -overlay`
2. It copies the tracked `fuku.yaml` and `examples/bookstore` into `/tmp/fuku-demo`, then runs the case's `setup.sh`
3. VHS starts `lib/start.sh` hidden: a tmux session with one pane, or two with `LAYOUT=split`.
   With `AGENT=1` the agent pane starts `claude` with the working-tree plugin, project settings only and no claude.ai connectors
4. `lib/watch.sh` logs the wall time each line of the case's `marks` file first shows on screen
5. `cut.py` turns the marks into speed changes from the case's `cut` file. Waits run faster, nothing is removed

## A case

`cases/<name>/` holds:

- `case.env`: `LAYOUT` (`single` or `split`) and `AGENT` (`0` or `1`)
- `setup.sh` (optional): stages the fault in the workspace copy
- `demo.tape`: the VHS script. It sources the shared `head.tape`
- `marks`: `name pane count regex` per line. A mark fires when `count` lines of the pane's history match
- `cut`: `offset` (the video time of the first mark) and `speed <from> <to> <factor>` lines

The site lists the cases in `docs/src/data/demos.ts`.

## Keep true

- no personal data on screen. The workspace sits in `/tmp`, the prompt is `$`, the tmux status bar is off,
  and the agent's welcome header, which names the subscription, is cleared before the recording starts
- only tracked files reach the workspace, so no local `.env` file shows up
- a demo stays under 40 seconds. Speed up a wait. Never cut one out
