// hello_dll.c — minimal Windows DLL used by runtime/pe E2E tests.
//
// Why a DLL: full Windows EXEs always reach ExitProcess at the
// end of main() — either directly or through the CRT _exit
// cleanup chain. No-Consolation tries to hook ExitProcess but
// the in-process loader model is fundamentally fragile when
// the Go runtime is sharing the host. A DLL with an explicit
// exported function returns cleanly to the BOF, which returns
// to the Go test, which sees the captured output.
//
// # Entry contract
//
// No-Consolation invokes the named export with a single arg:
// the operator-supplied cmdline string. This is NOT the typical
// `(int argc, char **argv)` shape — `runner.c run_pe_inthread`
// passes `cmdline` (or `cmdwline` when use_unicode) as the
// first arg, NULL as the second and third. hello_main matches
// that signature.
//
// Build (host with mingw-w64):
//   x86_64-w64-mingw32-gcc -shared -O2 -s -o hello.x64.dll \
//       -Wl,--export-all-symbols hello_dll.c

#include <stdio.h>
#include <windows.h>

__declspec(dllexport) int hello_main(const char *cmdline) {
    printf("HELLO_FROM_NOCONSOLATION_PE\n");
    fflush(stdout);
    if (cmdline && *cmdline) {
        printf("CMDLINE=%s\n", cmdline);
        fflush(stdout);
    }
    return 0;
}

BOOL WINAPI DllMain(HINSTANCE hi, DWORD reason, LPVOID lp) {
    (void)hi;
    (void)reason;
    (void)lp;
    return TRUE;
}
