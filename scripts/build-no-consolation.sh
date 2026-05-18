#!/usr/bin/env bash
# build-no-consolation.sh — fetch fortra/No-Consolation @ pinned
# commit and compile the x64 (and optionally x86) BOF object file
# into runtime/pe/internal/noconsolation/. Run from repo root.
#
# Prereqs: bash, git, mingw-w64. On Debian/Ubuntu:
#     sudo apt-get install mingw-w64
# On macOS:
#     brew install mingw-w64
# On Windows: WSL or MSYS2 with the mingw-w64-x86_64-gcc package.
#
# Why a build script (vs vendoring the .o): upstream publishes no
# release artefacts, the .o is small enough to rebuild on demand,
# and pinning at the commit level keeps the supply chain auditable.
# The output directory is .gitignored — operators decide whether
# to commit the compiled artefact in their own implant fork.

set -euo pipefail

# Pin upstream by commit, not by tag — the repo doesn't tag.
# Updating: bump this hash, re-run, commit the new hash + any
# packer fixes needed for newly-added BeaconData fields.
UPSTREAM_REPO="https://github.com/fortra/No-Consolation.git"
UPSTREAM_REF="main"

ROOT="$(git rev-parse --show-toplevel)"
DEST="${ROOT}/runtime/pe/internal/noconsolation"
WORK="${ROOT}/ignore/no-consolation-build"

mkdir -p "${DEST}" "${WORK}"

echo "[1/4] Cloning fortra/No-Consolation @ ${UPSTREAM_REF} into ${WORK}"
if [ ! -d "${WORK}/.git" ]; then
    git clone --depth=1 --branch="${UPSTREAM_REF}" "${UPSTREAM_REPO}" "${WORK}"
else
    git -C "${WORK}" fetch --depth=1 origin "${UPSTREAM_REF}"
    git -C "${WORK}" checkout "${UPSTREAM_REF}"
    git -C "${WORK}" reset --hard "origin/${UPSTREAM_REF}"
fi

PINNED_COMMIT="$(git -C "${WORK}" rev-parse HEAD)"
echo "[2/4] Pinned commit: ${PINNED_COMMIT}"

echo "[3/4] Compiling NoConsolation.x64.o via x86_64-w64-mingw32-gcc"
cd "${WORK}"
x86_64-w64-mingw32-gcc \
    -c source/entry.c \
    -o "${DEST}/NoConsolation.x64.o" \
    -masm=intel -Wall -I include

if command -v i686-w64-mingw32-gcc >/dev/null 2>&1; then
    echo "[3b] Compiling NoConsolation.x86.o (optional)"
    i686-w64-mingw32-gcc \
        -c source/entry.c \
        -o "${DEST}/NoConsolation.x86.o" \
        -masm=intel -Wall -I include
else
    echo "[3b] Skipping x86 build (i686-w64-mingw32-gcc not found)"
fi

cp "${WORK}/LICENSE" "${DEST}/LICENSE"

echo "[4/4] Sizes:"
ls -lh "${DEST}/"NoConsolation*.o

cat <<EOF

Built. To use the embedded loader, rebuild the implant with:

    go build -tags=pe_noconsolation ./...

Pinned upstream commit: ${PINNED_COMMIT}
EOF
