// crasher.c — deliberately access-violates inside the BOF
// mapping. Used by TestBOF_SacrificialThread_CrashIsolated to
// verify that sacrificial-mode catches the fault + lets the
// implant continue.
//
// In inline mode, running this BOF terminates the host process
// — which is exactly the contract sacrificial mode mitigates.
//
// Build (host with mingw-w64):
//   x86_64-w64-mingw32-gcc -c crasher.c -o crasher.o \
//       -O2 -Wall -ffreestanding -fno-stack-protector

#include <windows.h>

__declspec(dllimport) void BeaconPrintf(int type, const char *fmt, ...);

#define CALLBACK_OUTPUT 0x0

void go(char *args, int len) {
    (void)args;
    (void)len;
    BeaconPrintf(CALLBACK_OUTPUT, "about to crash\n");
    // Deliberate NULL deref. ExceptionAddress points at the
    // mov instruction inside this function — inside the BOF
    // mapping — so the sacrificial VEH should catch it.
    volatile int *bad = (int *)0;
    *bad = 42;
    BeaconPrintf(CALLBACK_OUTPUT, "should never reach here\n");
}
