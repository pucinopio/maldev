#!/usr/bin/env bash
# Pure-Go GIF recorder for the TUI tapes — works on Windows (no ttyd, no
# ffmpeg). Reads tapes from vhs/tui-gif/ by default, writes GIFs to vhs/out/.
#
# Usage:
#   scripts/tui-gif-record.sh                # all tapes
#   scripts/tui-gif-record.sh foo.tape       # one specific tape
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

mkdir -p vhs/out

tapes=()
if [ $# -gt 0 ]; then
  tapes+=("$1")
else
  shopt -s nullglob
  for f in vhs/tui-gif/*.tape; do
    tapes+=("$f")
  done
fi
if [ "${#tapes[@]}" -eq 0 ]; then
  echo "no .tape files found in vhs/tui-gif/" >&2
  exit 1
fi

fail=0
for t in "${tapes[@]}"; do
  echo "==> $t"
  if ! go run ./cmd/tui-gif "$t"; then
    echo "FAILED: $t" >&2
    fail=$((fail+1))
  fi
done

if [ "$fail" -gt 0 ]; then
  echo "$fail tape(s) failed" >&2
  exit 1
fi
echo "all tapes encoded → vhs/out/"
