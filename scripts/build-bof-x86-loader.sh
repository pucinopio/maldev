#!/usr/bin/env bash
# build-bof-x86-loader.sh — compile the 32-bit BOF loader as a
# flat PIC shellcode blob.
#
# Output: runtime/bof/internal/x86loader/bof_x86_loader.x86.bin
# (raw .text bytes, no PE wrapper, runs from any address via the
# parent's VirtualAllocEx + WriteProcessMemory + CreateRemoteThread
# injection — slice 1.d phase B-bis).
#
# Uses host i686-w64-mingw32-gcc if present; otherwise falls back
# to a Podman container (fedora:42 + mingw32-gcc) so any host
# with podman/docker rebuilds reproducibly. The committed .bin
# is the source of truth for operators — `go build` requires
# only Go, never the C toolchain.
#
# Run from repo root.

set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
DIR="${ROOT}/runtime/bof/internal/x86loader"
OUT_BIN="${DIR}/bof_x86_loader.x86.bin"
OUT_O="${DIR}/.build.loader.o"
OUT_ELF="${DIR}/.build.loader.elf"

CFLAGS=(
    -m32
    -O2 -Wall -Wextra
    -fno-asynchronous-unwind-tables
    -fno-ident
    -nostdlib
    -ffreestanding
    -fno-stack-protector
    -fno-pic -fno-pie     # no GOT, no PLT; refs are absolute, resolved at run time
    -masm=intel
)

build_with_host() {
    echo "[1/3] Compile loader.c -> loader.o"
    i686-w64-mingw32-gcc "${CFLAGS[@]}" -c "${DIR}/loader.c" -o "${OUT_O}"

    echo "[2/3] Link loader.o -> loader.elf (flat .text via loader.ld)"
    i686-w64-mingw32-ld -T "${DIR}/loader.ld" -o "${OUT_ELF}" "${OUT_O}"

    echo "[3/3] objcopy -O binary -j .text -> bof_x86_loader.x86.bin"
    i686-w64-mingw32-objcopy -O binary -j .text "${OUT_ELF}" "${OUT_BIN}"
}

build_with_podman() {
    local img=maldev-bof-x86-builder
    if ! podman image exists "${img}"; then
        echo "[0/3] Building Podman builder image (${img})"
        podman build -t "${img}" -f "${ROOT}/scripts/bof-x86-loader.Containerfile" "${ROOT}/scripts"
    fi

    podman run --rm \
        --userns=keep-id \
        -v "${ROOT}:/src:Z" \
        -w /src \
        "${img}" \
        bash -c "
            set -euo pipefail
            i686-w64-mingw32-gcc ${CFLAGS[*]} -c runtime/bof/internal/x86loader/loader.c -o runtime/bof/internal/x86loader/.build.loader.o
            i686-w64-mingw32-ld -T runtime/bof/internal/x86loader/loader.ld -o runtime/bof/internal/x86loader/.build.loader.elf runtime/bof/internal/x86loader/.build.loader.o
            i686-w64-mingw32-objcopy -O binary -j .text runtime/bof/internal/x86loader/.build.loader.elf runtime/bof/internal/x86loader/bof_x86_loader.x86.bin
        "
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

rm -f "${OUT_O}" "${OUT_ELF}"

echo
echo "Built: ${OUT_BIN}"
file "${OUT_BIN}" || true
ls -lh "${OUT_BIN}"
