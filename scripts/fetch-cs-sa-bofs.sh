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
# # Curated subset (10 BOFs)
#
# Core 4 (initial set):
# - dir        — filesystem enum + args (string + short)
# - env        — KERNEL32$GetEnvironmentStrings, no args
# - ipconfig   — IPHLPAPI$GetAdaptersAddresses + heavy .rdata reloc
# - listmods   — loaded-module enum, walks PEB Ldr
#
# Network suite (no-args, exercises distinct IPHLPAPI/DNSAPI/etc.):
# - arp        — ARP cache via IPHLPAPI$GetIpNetTable
# - routeprint — routing table via IPHLPAPI$GetIpForwardTable
# - listdns    — DNS cache via DNSAPI$DnsGetCacheDataTable
# - netstat    — TCP/UDP tables + flag arg
#
# System suite:
# - locale     — system locale info via KERNEL32$GetLocaleInfo*
# - netuptime  — NetServerGetInfo via NETAPI32, takes wstring (empty=self)
#
# Together they exercise: PEB walk on multiple modules
# (kernel32/ntdll/msvcrt/iphlpapi/netapi32/dnsapi), forwarders
# (HeapAlloc), .rdata pointer-table relocations, args of every
# shape (none/int/short/string/wstring), and dollar-form imports.

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
for bof in dir env ipconfig listmods arp routeprint listdns netstat locale netuptime; do
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
