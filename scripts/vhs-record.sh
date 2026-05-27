#!/usr/bin/env bash
# Records all VHS tapes in vhs/*.tape into GIFs under vhs/out/.
#
# Auto-installs vhs (charmbracelet/vhs) via `go install` if not on PATH —
# requires Go toolchain. Builds ./cmd/license-manager into ./bin/license-manager
# so every tape can run a real instance of the TUI without polluting the
# operator's $PATH.
#
# Usage:
#   scripts/vhs-record.sh           # all tapes
#   scripts/vhs-record.sh foo.tape  # one specific tape
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

# ── tooling ────────────────────────────────────────────────────────────────
if ! command -v vhs >/dev/null 2>&1; then
  echo "==> installing charmbracelet/vhs..."
  go install github.com/charmbracelet/vhs@latest
  if ! command -v vhs >/dev/null 2>&1; then
    echo "vhs not on PATH after install; check \$(go env GOPATH)/bin" >&2
    exit 1
  fi
fi

# ── build the TUI binary the tapes drive ───────────────────────────────────
mkdir -p bin
echo "==> building ./cmd/license-manager → ./bin/license-manager"
go build -o "./bin/license-manager" ./cmd/license-manager

# ── pick the tape set ──────────────────────────────────────────────────────
mkdir -p vhs/out
tapes=()
if [ $# -gt 0 ]; then
  tapes+=("$1")
else
  shopt -s nullglob
  for f in vhs/*.tape; do
    tapes+=("$f")
  done
fi
if [ "${#tapes[@]}" -eq 0 ]; then
  echo "no .tape files found in vhs/" >&2
  exit 1
fi

# ── record ────────────────────────────────────────────────────────────────
fail=0
for t in "${tapes[@]}"; do
  echo "==> recording $t"
  if ! vhs "$t"; then
    echo "FAILED: $t" >&2
    fail=$((fail+1))
  fi
done

if [ "$fail" -gt 0 ]; then
  echo "$fail tape(s) failed" >&2
  exit 1
fi
echo "all tapes recorded to vhs/out/"
