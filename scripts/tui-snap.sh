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
out="ignore/snapshots/${view}.png"

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

# shellcheck disable=SC2086
go run ./cmd/tui-snap \
    -view   "${view}" \
    -width  "${width}" \
    -height "${height}" \
    ${seed_arg} \
| "${freeze_cmd}" \
    --output "${out}" \
    --window \
    --shadow \
    --margin 10 \
    --padding 20 \
    --font.family "JetBrains Mono" \
    --font.size 14 \
    --theme "dracula"

echo "wrote ${out}"
