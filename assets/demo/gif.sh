#!/bin/sh
# Writes the README GIF from a recorded case: assets/demo/gif.sh <case>
set -eu

CASE=${1:?usage: gif.sh <case>}
ROOT=$(cd "$(dirname "$0")/../.." && pwd)

ffmpeg -loglevel error -y -i "$ROOT/docs/public/demo/$CASE.mp4" \
  -vf "fps=10,split[a][b];[a]palettegen=max_colors=96:stats_mode=diff[p];[b][p]paletteuse=dither=none:diff_mode=rectangle" \
  "$ROOT/assets/demo.gif"
