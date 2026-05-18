#!/usr/bin/env bash
# build-bof-x86-loader.sh — compile the 32-bit BOF loader as a
# proper i386 DLL with relocations preserved. The Go orchestrator
# parses the PE header and applies relocations during the cross-
# process manual reflective load (slice 1.d phase B-bis,
# reflective-DLL model — never invokes the OS PE loader nor
# touches disk).
#
# Output: runtime/bof/internal/x86loader/bof_x86_loader.x86.dll
# (committed; `go build` embeds the bytes via go:embed).
#
# Uses host i686-w64-mingw32-gcc if present; otherwise falls back
# to a Podman container (fedora:42 + mingw32-gcc).
#
# Run from repo root.

set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
DIR="${ROOT}/runtime/bof/internal/x86loader"
OUT_DLL="${DIR}/bof_x86_loader.x86.dll"

CFLAGS=(
    -m32
    -O2 -Wall -Wextra
    -fno-asynchronous-unwind-tables
    -fno-ident
    -nostdlib
    -ffreestanding
    -fno-stack-protector
    -masm=intel
    -shared
    # ASLR + relocations: --dynamicbase + --enable-reloc-section
    # so the DLL ships with a populated .reloc table — the parent
    # Go reflective loader walks it to rebase the image at the
    # VirtualAllocEx-returned address.
    -Wl,--dynamicbase
    -Wl,--enable-reloc-section
    -Wl,--nxcompat
    -Wl,--kill-at
    -Wl,--enable-stdcall-fixup
    -Wl,-s
    -Wl,--entry=_DllMain@12
)

LDLIBS=( -lkernel32 )

build_with_host() {
    echo "[1/1] Compile + link via host i686-w64-mingw32-gcc"
    i686-w64-mingw32-gcc \
        "${CFLAGS[@]}" \
        -o "${OUT_DLL}" \
        "${DIR}/loader.c" \
        "${LDLIBS[@]}"
}

build_with_podman() {
    local img=maldev-bof-x86-builder
    if ! podman image exists "${img}"; then
        echo "[0/1] Building Podman builder image (${img})"
        podman build -t "${img}" -f "${ROOT}/scripts/bof-x86-loader.Containerfile" "${ROOT}/scripts"
    fi

    podman run --rm \
        --userns=keep-id \
        -v "${ROOT}:/src:Z" \
        -w /src \
        "${img}" \
        i686-w64-mingw32-gcc \
        "${CFLAGS[@]}" \
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

echo
echo "Built: ${OUT_DLL}"
file "${OUT_DLL}" || true
ls -lh "${OUT_DLL}"
