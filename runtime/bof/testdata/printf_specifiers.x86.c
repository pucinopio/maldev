/*
 * printf_specifiers.x86.c — i386 BOF fixture exercising the
 * vararg expansion in BeaconPrintf (step 1.g).
 *
 * Expected output:
 *   int=-42 hex=CAFE u=4294967254 s=abc c=Z pct=%
 *
 * Build (committed as printf_specifiers.x86.o):
 *   i686-w64-mingw32-gcc -m32 -O2 -fno-asynchronous-unwind-tables \
 *     -fno-unwind-tables -fno-ident -ffreestanding -c \
 *     printf_specifiers.x86.c -o printf_specifiers.x86.o
 */

__declspec(dllimport) void __cdecl BeaconPrintf(int type, const char *fmt, ...);

void __cdecl go(char *args, int alen)
{
    (void)args;
    (void)alen;
    BeaconPrintf(0, "int=%d hex=%X u=%u s=%s c=%c pct=%%",
                 -42, 0xCAFE, (unsigned)-42, "abc", 'Z');
}
