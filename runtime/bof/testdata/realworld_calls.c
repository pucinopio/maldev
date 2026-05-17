// realworld_calls — clean-room equivalent of the typical
// "post-ex enumeration" BOF (think TrustedSec SA whoami / netuser /
// arp) that exercises the full Beacon API surface a real CS module
// reaches for. NOT a copy of any GPL-licensed BOF; original code that
// happens to call the same Windows APIs.
//
// Surface exercised:
//   - BeaconPrintf with multiple varargs (%s, %d, %x)
//   - BeaconOutput for raw bytes
//   - BeaconIsAdmin (helpers)
//   - BeaconUseToken + BeaconRevertToken (token impersonation)
//   - BeaconGetSpawnTo (operator-configured path)
//   - BeaconAddValue / BeaconGetValue (KV store)
//   - toWideChar (UTF-8 → UTF-16)
//   - Live Win32 imports: GetComputerNameA, GetCurrentProcessId,
//     GetTickCount, OpenProcessToken
//
// Build:
//   x86_64-w64-mingw32-gcc -c realworld_calls.c -o realworld_calls.o \
//       -O2 -Wall -ffreestanding -fno-stack-protector

#include <windows.h>

__declspec(dllimport) void  BeaconPrintf(int type, const char *fmt, ...);
__declspec(dllimport) void  BeaconOutput(int type, char *data, int len);
__declspec(dllimport) int   BeaconIsAdmin(void);
__declspec(dllimport) BOOL  BeaconUseToken(HANDLE token);
__declspec(dllimport) void  BeaconRevertToken(void);
__declspec(dllimport) char *BeaconGetSpawnTo(BOOL x86);
__declspec(dllimport) void  BeaconAddValue(const char *key, void *ptr);
__declspec(dllimport) void *BeaconGetValue(const char *key);
__declspec(dllimport) int   toWideChar(char *src, wchar_t *dst, int max);

__declspec(dllimport) BOOL  KERNEL32$GetComputerNameA(char *lpBuffer, DWORD *nSize);
__declspec(dllimport) DWORD KERNEL32$GetCurrentProcessId(void);
__declspec(dllimport) DWORD KERNEL32$GetTickCount(void);
__declspec(dllimport) HANDLE KERNEL32$GetCurrentProcess(void);
__declspec(dllimport) BOOL  ADVAPI32$OpenProcessToken(HANDLE proc, DWORD access, HANDLE *tok);
__declspec(dllimport) BOOL  KERNEL32$CloseHandle(HANDLE h);

#define CALLBACK_OUTPUT 0x0

void go(char *args, int len) {
    (void)args; (void)len;

    char name[256];
    DWORD sz = sizeof(name);
    KERNEL32$GetComputerNameA(name, &sz);

    DWORD pid     = KERNEL32$GetCurrentProcessId();
    DWORD ticks   = KERNEL32$GetTickCount();
    int   isAdmin = BeaconIsAdmin();

    BeaconPrintf(CALLBACK_OUTPUT, "host=%s pid=%d admin=%d ticks=0x%x\n",
                 name, pid, isAdmin, ticks);

    // Token roundtrip: open + impersonate self + revert.
    HANDLE tok = 0;
    if (ADVAPI32$OpenProcessToken(KERNEL32$GetCurrentProcess(), 0x00020008 /* TOKEN_QUERY|TOKEN_DUPLICATE */, &tok)) {
        if (BeaconUseToken(tok)) {
            BeaconPrintf(CALLBACK_OUTPUT, "impersonate=ok\n");
            BeaconRevertToken();
        }
        KERNEL32$CloseHandle(tok);
    }

    // SpawnTo readback.
    char *spawn = BeaconGetSpawnTo(FALSE);
    if (spawn != 0) {
        BeaconPrintf(CALLBACK_OUTPUT, "spawnto=%s\n", spawn);
    }

    // KV roundtrip.
    int sentinel = 0xC0FFEE;
    BeaconAddValue("marker", &sentinel);
    int *got = (int *)BeaconGetValue("marker");
    if (got == &sentinel) {
        BeaconPrintf(CALLBACK_OUTPUT, "kv=ok value=0x%x\n", *got);
    }

    // Wide-char conversion + raw output.
    wchar_t wide[32];
    int n = toWideChar("widearg", wide, 32);
    BeaconPrintf(CALLBACK_OUTPUT, "widelen=%d wide=%s\n", n, wide);

    // Raw output channel — non-ASCII bytes survive intact.
    unsigned char raw[4] = {0xDE, 0xAD, 0xBE, 0xEF};
    BeaconOutput(CALLBACK_OUTPUT, (char *)raw, 4);
}
