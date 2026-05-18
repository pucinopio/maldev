/*
 * hello_beacon.x86.c — i386 BOF fixture exercising BeaconPrintf.
 *
 * Build (committed as hello_beacon.x86.o):
 *   i686-w64-mingw32-gcc -m32 -O2 -c hello_beacon.x86.c -o hello_beacon.x86.o
 *
 * The DECLSPEC_IMPORT forces the compiler to emit an indirect
 * call through __imp__BeaconPrintf — the loader resolves that
 * external symbol against its in-DLL Beacon table via ROR13.
 *
 * Expected output: "hello from x86 BOF\n" written to the
 * parent-allocated out buffer.
 */

__declspec(dllimport) void __cdecl BeaconPrintf(int type, const char *fmt, ...);

void __cdecl go(char *args, int alen)
{
    (void)args;
    (void)alen;
    BeaconPrintf(0, "hello from x86 BOF\n");
}
