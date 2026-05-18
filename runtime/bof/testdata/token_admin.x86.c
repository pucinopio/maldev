/*
 * token_admin.x86.c — i386 BOF fixture exercising the Token +
 * IsAdmin family (step 1.f).
 *
 * Output is semicolon-separated assertions for the parent:
 *   isadmin=<0|1>;revert=done
 *
 * Build (committed as token_admin.x86.o):
 *   i686-w64-mingw32-gcc -m32 -O2 -fno-asynchronous-unwind-tables \
 *     -fno-unwind-tables -fno-ident -ffreestanding -c \
 *     token_admin.x86.c -o token_admin.x86.o
 */

__declspec(dllimport) void __cdecl BeaconPrintf(int type, const char *fmt, ...);
__declspec(dllimport) int  __cdecl BeaconIsAdmin(void);
__declspec(dllimport) void __cdecl BeaconRevertToken(void);

void __cdecl go(char *args, int alen)
{
    (void)args;
    (void)alen;

    int admin = BeaconIsAdmin();
    BeaconPrintf(0, "isadmin=");
    BeaconPrintf(0, admin ? "1" : "0");
    BeaconPrintf(0, ";");

    /* Safe no-op when no impersonation is active. */
    BeaconRevertToken();
    BeaconPrintf(0, "revert=done");
}
