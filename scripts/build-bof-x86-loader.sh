#!/usr/bin/env bash
# build-bof-x86-loader.sh — compile the 32-bit BOF loader DLL.
#
# Uses host i686-w64-mingw32-gcc when on PATH; otherwise falls
# back to a Podman container (fedora:42 + mingw32-gcc) so any
# host with podman/docker can rebuild reproducibly. Output:
# runtime/bof/internal/x86loader/bof_x86_loader.x86.dll.
#
# Run from repo root. The .dll is committed to the repo (same
# model as NoConsolation.x64.o); this script exists for rebuilds
# when the ABI bumps or features land in phase C.

set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
DIR="${ROOT}/runtime/bof/internal/x86loader"
OUT="${DIR}/bof_x86_loader.x86.dll"

CFLAGS_COMMON=(
    -O2 -Wall -Wextra
    -fno-asynchronous-unwind-tables
    -fno-ident
    -nostdlib
    -Wl,--enable-stdcall-fixup
    -Wl,--kill-at
    # ASLR + relocations: --dynamicbase + --enable-reloc-section so
    # the OS loader can rebase if the preferred base (0x66000000 on
    # current mingw32) collides. --nxcompat marks the DLL as DEP-
    # aware. -s strips symbols / debug data so the final blob stays
    # under ~4 KB.
    -Wl,--dynamicbase
    -Wl,--enable-reloc-section
    -Wl,--nxcompat
    -Wl,-s
    -shared
    -Wl,--entry=_DllMain@12
)

# Libraries go AFTER the source file in the ld command line —
# ld is single-pass left-to-right. Putting -lkernel32 before
# loader.c leaves the kernel32 imports unresolved.
LDLIBS=( -lkernel32 )

build_with_host() {
    echo "[1/1] Compiling via host i686-w64-mingw32-gcc"
    i686-w64-mingw32-gcc \
        "${CFLAGS_COMMON[@]}" \
        -o "${OUT}" \
        "${DIR}/loader.c" \
        "${LDLIBS[@]}"
}

build_with_podman() {
    echo "[1/2] Building Podman builder image"
    local img=maldev-bof-x86-builder
    if ! podman image exists "${img}"; then
        podman build -t "${img}" -f "${ROOT}/scripts/bof-x86-loader.Containerfile" "${ROOT}/scripts"
    fi

    echo "[2/2] Compiling via Podman (${img})"
    podman run --rm \
        --userns=keep-id \
        -v "${ROOT}:/src:Z" \
        -w /src \
        "${img}" \
        i686-w64-mingw32-gcc \
        "${CFLAGS_COMMON[@]}" \
        -o "runtime/bof/internal/x86loader/bof_x86_loader.x86.dll" \
        "runtime/bof/internal/x86loader/loader.c" \
        "${LDLIBS[@]}"
}

if command -v i686-w64-mingw32-gcc >/dev/null 2>&1; then
    build_with_host
elif command -v podman >/dev/null 2>&1; then
    build_with_podman
else
    echo "error: neither i686-w64-mingw32-gcc nor podman is available." >&2
    echo "Install one of:" >&2
    echo "  - Fedora:        sudo dnf install mingw32-gcc" >&2
    echo "  - Debian/Ubuntu: sudo apt install gcc-mingw-w64-i686" >&2
    echo "  - macOS:         brew install mingw-w64" >&2
    echo "  - Podman:        sudo dnf install podman    (any host)" >&2
    exit 1
fi

echo "Built: ${OUT}"
file "${OUT}" || true
ls -lh "${OUT}"
