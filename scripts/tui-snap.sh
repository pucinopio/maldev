#!/usr/bin/env bash
# tui-snap.sh — render a license-manager TUI view to PNG via charmbracelet/freeze.
#
# Usage:
#   scripts/tui-snap.sh [view] [width] [height] [seed.json]
#
# Examples:
#   scripts/tui-snap.sh dashboard
#   scripts/tui-snap.sh dashboard 160 48
#   scripts/tui-snap.sh dashboard 144 44 scripts/tui-snap-seeds/dashboard.json
#
# Output: ignore/snapshots/<view>.png
#
# Prerequisites:
#   go install github.com/charmbracelet/freeze@latest
#   A JetBrains Mono (or similar monospace) font installed on the system.
set -euo pipefail

view="${1:-dashboard}"
width="${2:-144}"
height="${3:-44}"
seed="${4:-scripts/tui-snap-seeds/${view}.json}"
out="ignore/snapshots/${view}.svg"
tmp_ansi="ignore/snapshots/${view}.ansi"

mkdir -p ignore/snapshots

# Seed file is optional — omit -seed flag if the file does not exist.
seed_arg=""
if [ -f "${seed}" ]; then
    seed_arg="-seed ${seed}"
fi

freeze_bin="$(go env GOPATH)/bin/freeze"
if ! command -v freeze &>/dev/null && [ ! -x "${freeze_bin}" ]; then
    echo "freeze not found — install with: go install github.com/charmbracelet/freeze@latest" >&2
    exit 1
fi

# Prefer PATH freeze, fall back to GOPATH/bin/freeze.
if ! command -v freeze &>/dev/null; then
    freeze_cmd="${freeze_bin}"
else
    freeze_cmd="freeze"
fi

# Use prebuilt binary if available; falls back to `go run` (slower).
if [ -x "bin/tui-snap" ] || [ -x "bin/tui-snap.exe" ]; then
    tui_snap_bin="bin/tui-snap"
    [ -x "bin/tui-snap.exe" ] && tui_snap_bin="bin/tui-snap.exe"
    # shellcheck disable=SC2086
    "${tui_snap_bin}" -view "${view}" -width "${width}" -height "${height}" ${seed_arg} > "${tmp_ansi}"
else
    # shellcheck disable=SC2086
    go run ./cmd/tui-snap -view "${view}" -width "${width}" -height "${height}" ${seed_arg} > "${tmp_ansi}"
fi

# SVG output — freeze's PNG path crashes on Windows (v0.2.2 GC bug).
# Use Cascadia Code: shipped with Windows, has all box-drawing glyphs.
"${freeze_cmd}" "${tmp_ansi}" -l ansi \
    --output "${out}" \
    --window \
    --margin 10 \
    --padding 20 \
    --font.family "Cascadia Code" \
    --font.size 14 \
    --theme "dracula"

# Optional PNG conversion via headless Chrome (auto-detected).
chrome=""
for c in "/c/Program Files/Google/Chrome/Application/chrome.exe" \
         "/c/Program Files (x86)/Google/Chrome/Application/chrome.exe" \
         "/c/Program Files/Microsoft/Edge/Application/msedge.exe" \
         "chrome" "google-chrome" "chromium"; do
    if command -v "$c" &>/dev/null || [ -x "$c" ]; then chrome="$c"; break; fi
done
if [ -n "${chrome}" ]; then
    png="${out%.svg}.png"
    pwd_w=$(pwd -W 2>/dev/null || pwd)
    abs_svg="$(realpath "${out}" | sed 's|^/c/|C:/|; s|/mnt/c/|C:/|')"
    abs_png="$(echo "${pwd_w}/${png}" | sed 's|\\|/|g')"
    "${chrome}" --headless --disable-gpu --no-sandbox --hide-scrollbars \
      --screenshot="${abs_png}" \
      --window-size=1600,1400 \
      "file:///${abs_svg}" >/dev/null 2>&1 || true
fi

rm -f "${tmp_ansi}"
echo "wrote ${out}"
[ -f "${png:-}" ] && echo "wrote ${png}"
