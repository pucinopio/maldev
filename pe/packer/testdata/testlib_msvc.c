/*
 * MSVC-compiled test DLL fixture for Item #7
 * (docs/refactor-2026-doc/packer-actions-2026-05-12.md).
 *
 * Exercises the native-DLL packer (Mode 7) against a binary
 * produced by Microsoft's toolchain rather than mingw — different
 * import-table layout, SEH unwind metadata (.pdata/.xdata), CFG
 * tables, /GS stack cookies, all of which the packer must leave
 * intact when it rewrites the entry point and appends the stub
 * section.
 *
 * Build (from the Win10 VM with VS Build Tools installed):
 *   cl /LD /MD /O2 /GS /Gy testlib_msvc.c /Fe:testlib_msvc.dll \
 *      /link /DLL /NOLOGO /DEF:testlib_msvc.def
 *
 * The /MD flag links the dynamic CRT — that pulls in a non-trivial
 * import table (vcruntime140.dll, ucrtbase.dll), exactly the shape
 * the packer would face in real engagements. /Gy + /Gs cookies
 * stress the .pdata + /GS sections.
 *
 * Exports (see testlib_msvc.def):
 *   maldev_ping  — returns 0xC0DEBABE as DWORD. Test calls it
 *                  post-LoadLibrary to confirm the export table is
 *                  intact and the function body actually ran after
 *                  the stub decrypted .text.
 *   maldev_add   — returns a + b. Exercises a function with args.
 */
#include <windows.h>

__declspec(dllexport) DWORD maldev_ping(void) {
    return 0xC0DEBABE;
}

__declspec(dllexport) int maldev_add(int a, int b) {
    return a + b;
}

BOOL APIENTRY DllMain(HINSTANCE hInst, DWORD reason, LPVOID reserved) {
    (void)hInst;
    (void)reason;
    (void)reserved;
    return TRUE;
}
