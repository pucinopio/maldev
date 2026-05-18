// hello.c — minimal Windows console EXE used by runtime/pe E2E tests.
//
// Behaviour:
//   - prints a stable canary ("HELLO_FROM_NOCONSOLATION_PE")
//   - if argc > 1, additionally prints "ARGV[i]=<token>" lines so tests
//     can assert the No-Consolation cmdline path forwards args correctly
//   - exits 0
//
// Build (host with mingw-w64):
//   x86_64-w64-mingw32-gcc -O2 -s -o hello.x64.exe hello.c
//
// The resulting binary is ~4-5 KB stripped; it's vendored alongside
// the source so VM tests don't need a mingw toolchain at run time.

#include <stdio.h>

int main(int argc, char **argv) {
    printf("HELLO_FROM_NOCONSOLATION_PE\n");
    fflush(stdout);
    for (int i = 1; i < argc; i++) {
        printf("ARGV[%d]=%s\n", i, argv[i]);
    }
    fflush(stdout);
    return 0;
}
