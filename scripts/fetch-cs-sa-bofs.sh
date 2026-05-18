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
# # Curated subset (32 BOFs)
#
# Core 4:
# - dir        — filesystem enum + (string, short) args
# - env        — KERNEL32$GetEnvironmentStrings, no args
# - ipconfig   — IPHLPAPI$GetAdaptersAddresses + heavy .rdata reloc
# - listmods   — PEB Ldr walk, int arg
#
# Network suite:
# - arp        — IPHLPAPI$GetIpNetTable
# - routeprint — IPHLPAPI$GetIpForwardTable
# - listdns    — DNSAPI$DnsGetCacheDataTable (resolver cache)
# - netstat    — IPHLPAPI$GetExtendedTcpTable + int arg
# - nslookup   — DNSAPI$DnsQuery_A (active resolution, not cache)
#
# System / service / security:
# - locale            — KERNEL32$GetLocaleInfoEx
# - netuptime         — NETAPI32$NetStatisticsGet, wstring arg
# - netlocalgroup     — NETAPI32$NetLocalGroupEnum, short + wstring
# - netloggedon       — NETAPI32$NetWkstaUserEnum, wstring arg
# - enumlocalsessions — WTSAPI32$WTSEnumerateSessions
# - sc_enum           — ADVAPI32$EnumServicesStatus, wstring arg
# - list_firewall_rules — HNetCfg COM (firewall policy)
# - driversigs        — SETUPAPI$SetupDiGetDeviceRegistryProperty (drivers)
# - md5               — MSVCRT file I/O + ADVAPI32 CryptCreateHash, string
#
# Together they exercise: PEB walk on a dozen modules
# (kernel32/ntdll/msvcrt/iphlpapi/netapi32/dnsapi/advapi32/wtsapi32/
# setupapi/shlwapi/ole32), export forwarders (HeapAlloc),
# .rdata pointer-table relocations, args of every shape
# (none/int/short/string/wstring), and COM-via-BOF init paths.

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
for bof in \
    dir env ipconfig listmods \
    arp routeprint listdns netstat nslookup \
    locale netuptime netlocalgroup netloggedon enumlocalsessions \
    sc_enum list_firewall_rules driversigs md5 \
    whoami tasklist uptime useridletime windowlist \
    sha1 sha256 cacls nettime schtasksenum aadjoininfo \
    get_session_info netshares get_password_policy \
    adv_audit_policies regsession sc_query vssenum netuser; do
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
