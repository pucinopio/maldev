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

# Wrap the vcvars64 invocation in a small remote .bat so all
# quoting/escaping happens on the Windows side. Avoids the
# shell-passes-through-ssh quoting hell.
cat > /tmp/run_msvc_build.bat << 'BAT'
@echo off
call "C:\Program Files (x86)\Microsoft Visual Studio\2022\BuildTools\VC\Auxiliary\Build\vcvars64.bat" >nul
if errorlevel 1 (echo vcvars64 failed & exit /b 1)
call C:\Users\Public\build_testlib_msvc.cmd
BAT
scp "${SSH[@]}" /tmp/run_msvc_build.bat "test@$WIN_IP:C:/Users/Public/run_msvc_build.bat" >/dev/null

echo "[+] Compiling on VM"
ssh "${SSH[@]}" "test@$WIN_IP" 'cmd /c C:\Users\Public\run_msvc_build.bat' 2>&1 | tail -15

echo "[+] Pulling testlib_msvc.dll back"
scp "${SSH[@]}" "test@$WIN_IP:C:/Users/Public/testlib_msvc.dll" "$TESTDATA/testlib_msvc.dll"
ls -la "$TESTDATA/testlib_msvc.dll"
file "$TESTDATA/testlib_msvc.dll"
