#!/usr/bin/env bash
# fetch-cs-sa-bofs.sh — pin-checkout the TrustedSec
# CS-Situational-Awareness-BOF repo and copy a curated subset of
# prebuilt .o files into runtime/bof/testdata/cs-sa/ for E2E tests.
#
# # Why fetch instead of vendor
#
# CS-SA-BOF is GPL-2.0; maldev is MIT. Committing GPL-licensed
# binaries into the MIT tree creates a licensing tangle even when
# the .o is only test input. Operators run this script locally;
# the .o files land in a .gitignored directory and never reach
# the maldev distribution. Tests skip when the fixtures are absent.
#
# # Curated subset (4 BOFs)
#
# - dir       — filesystem enum + args parsing + MSVCRT$strlen/strcat/etc.
# - env       — KERNEL32$GetEnvironmentStrings, no args
# - ipconfig  — IPHLPAPI$GetAdaptersInfo network surface
# - listmods  — loaded-module enum, walks PEB / kernel32 modules
#
# Each exercises a distinct slice of the Beacon API + import
# resolver, so the four-test suite catches regressions in PEB-walk
# resolution, MSVCRT/KERNEL32 dollar-form imports, forwarders, and
# output capture.

set -euo pipefail

UPSTREAM_REPO="https://github.com/trustedsec/CS-Situational-Awareness-BOF.git"
UPSTREAM_REF="master"
PINNED_COMMIT="ee9459cc4f42c6b025797bad22ffe8d9f1cf6487"  # 2024-05 snapshot

ROOT="$(git rev-parse --show-toplevel)"
DEST="${ROOT}/runtime/bof/testdata/cs-sa"
WORK="${ROOT}/ignore/cs-sa-bof"

mkdir -p "${DEST}" "${WORK}"

if [ ! -d "${WORK}/.git" ]; then
    echo "[1/3] Cloning ${UPSTREAM_REPO} @ ${UPSTREAM_REF}"
    git clone --depth=1 --branch="${UPSTREAM_REF}" "${UPSTREAM_REPO}" "${WORK}"
fi

echo "[2/3] Pinning to commit ${PINNED_COMMIT}"
git -C "${WORK}" fetch --depth=1 origin "${PINNED_COMMIT}" 2>/dev/null || true
git -C "${WORK}" checkout "${PINNED_COMMIT}" 2>&1 | tail -2

echo "[3/3] Copying curated subset into ${DEST}"
for bof in dir env ipconfig listmods; do
    src="${WORK}/SA/${bof}/${bof}.x64.o"
    if [ ! -f "${src}" ]; then
        echo "  ! missing ${src}"
        exit 1
    fi
    cp "${src}" "${DEST}/${bof}.x64.o"
    echo "  + ${bof}.x64.o ($(wc -c < "${src}") bytes)"
done

# Drop the upstream LICENSE alongside so the GPL-2 attribution
# travels with the artefacts even though they're git-ignored.
cp "${WORK}/LICENSE" "${DEST}/UPSTREAM_LICENSE"

echo
echo "Done. Test E2E with:"
echo "  go test -count=1 -v -run TestCS_SA ./runtime/bof/"
