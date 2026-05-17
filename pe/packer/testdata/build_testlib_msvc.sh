#!/usr/bin/env bash
# build_testlib_msvc.sh — drive the Win10 VM to compile testlib_msvc.dll
# using the installed VS Build Tools, then scp the artefact back to
# pe/packer/testdata/.
#
# Prerequisite: scripts/vm-provision.sh has installed MSVC on the VM
# (Item #7). Run from the maldev repo root.
set -euo pipefail

cd "$(dirname "$0")/../../.."
TESTDATA="pe/packer/testdata"

WIN_IP="${MALDEV_VM_WINDOWS_SSH_HOST:-192.168.122.122}"
KEY="$HOME/.ssh/vm_windows_key"
SSH=(-i "$KEY" -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null)

echo "[+] Pushing testlib_msvc sources to VM"
scp "${SSH[@]}" \
    "$TESTDATA/testlib_msvc.c" \
    "$TESTDATA/testlib_msvc.def" \
    "$TESTDATA/build_testlib_msvc.cmd" \
    "test@$WIN_IP:C:/Users/Public/" >/dev/null

# vcvars64.bat sets up cl.exe / link.exe / Windows SDK paths.
VCVARS='C:\Program Files (x86)\Microsoft Visual Studio\2022\BuildTools\VC\Auxiliary\Build\vcvars64.bat'

echo "[+] Compiling on VM"
ssh "${SSH[@]}" "test@$WIN_IP" \
    "cmd /c \"call \\\"$VCVARS\\\" && C:\\Users\\Public\\build_testlib_msvc.cmd\"" 2>&1 | tail -10

echo "[+] Pulling testlib_msvc.dll back"
scp "${SSH[@]}" "test@$WIN_IP:C:/Users/Public/testlib_msvc.dll" "$TESTDATA/testlib_msvc.dll"
ls -la "$TESTDATA/testlib_msvc.dll"
file "$TESTDATA/testlib_msvc.dll"
