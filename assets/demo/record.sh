#!/bin/sh
# Records one demo case into docs/public/demo: assets/demo/record.sh <case> [--prepare]
set -eu

CASE=${1:?usage: record.sh <case> [--prepare]}
ROOT=$(cd "$(dirname "$0")/../.." && pwd)
DEMO=$ROOT/assets/demo
CASE_DIR=$DEMO/cases/$CASE
W=/tmp/fuku-demo
VERSION=${VERSION:-0.23.0}

[ -d "$CASE_DIR" ] || { echo "unknown case: $CASE" >&2; exit 1; }

teardown() {
  (cd "$W/project" && PATH=$W/bin:$PATH fuku stop >/dev/null 2>&1) || true
  tmux -L demo kill-server 2>/dev/null || true
}

teardown
rm -rf "$W"
mkdir -p "$W/bin" "$W/out" "$W/project"
# tracked files only, so no local .env file reaches a video
git -C "$ROOT" archive HEAD fuku.yaml examples/bookstore | tar -x -C "$W/project"
# the playground guide tells the agent the services are fake
rm -f "$W/project/examples/bookstore/CLAUDE.md"
cp "$DEMO/lib/tmux.conf" "$DEMO/lib/rc" "$DEMO/lib/start.sh" "$DEMO/lib/watch.sh" "$DEMO/lib/head.tape" "$W/"
cp "$CASE_DIR/case.env" "$CASE_DIR/marks" "$W/"
sed "s#@CLAUDE@#$(command -v claude)#; s#@PLUGIN@#$ROOT/plugins/agents#" "$DEMO/lib/claude" > "$W/bin/claude"
chmod +x "$W/bin/claude"
# Version is a constant, so an overlay builds the demo binary with the version of the coming release
BUILD=$ROOT/internal/platform/buildinfo/build.go
sed "s/Version = \".*\"/Version = \"$VERSION\"/" "$BUILD" > "$W/build.go"
printf '{"Replace":{"%s":"%s"}}' "$BUILD" "$W/build.go" > "$W/overlay.json"
(cd "$ROOT" && go build -overlay "$W/overlay.json" -o "$W/bin/fuku" ./cmd)

if [ -x "$CASE_DIR/setup.sh" ]; then
  (cd "$W/project" && PATH=$W/bin:$PATH "$CASE_DIR/setup.sh")
fi

[ "${2:-}" = --prepare ] && { echo "prepared $W"; exit 0; }

trap teardown EXIT
(cd "$W" && vhs "$CASE_DIR/demo.tape")
python3 "$DEMO/cut.py" "$W/out/marks" "$CASE_DIR/cut" "$W/out/raw.mp4" "$ROOT/docs/public/demo/$CASE.mp4"
