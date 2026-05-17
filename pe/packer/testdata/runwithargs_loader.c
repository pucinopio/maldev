// Minimal non-Go loader for the RunWithArgs export.
//
// Used by TestPackBinary_ConvertEXEtoDLL_RunWithArgs_E2E: the Go
// test runner itself cannot host the RunWithArgs call because the
// packed Go OEP terminates the calling process via ExitProcess(0)
// when its main returns — killing the test runner before the marker
// file can be read. A subprocess loader absorbs that termination;
// the test only inspects the marker after the subprocess exits.
//
// Cross-build from Linux:
//   x86_64-w64-mingw32-gcc -O0 -o runwithargs_loader.exe runwithargs_loader.c
//
// Invocation:
//   runwithargs_loader.exe <packed-dll-path>
#include <windows.h>
#include <stdio.h>

typedef ULONG_PTR(WINAPI *RunWithArgsFn)(LPCWSTR);

int main(int argc, char *argv[]) {
    if (argc < 2) {
        printf("usage: %s <dll-path>\n", argv[0]);
        return 2;
    }
    HMODULE h = LoadLibraryA(argv[1]);
    if (!h) {
        printf("LoadLibrary failed: %lu\n", GetLastError());
        return 1;
    }
    FARPROC proc = GetProcAddress(h, "RunWithArgs");
    if (!proc) {
        printf("GetProcAddress(RunWithArgs) failed: %lu\n", GetLastError());
        return 1;
    }
    RunWithArgsFn fn = (RunWithArgsFn)proc;
    LPCWSTR args = L"operator.exe runtime alpha beta";
    fn(args); // never returns — packed Go OEP calls ExitProcess.
    return 0;
}
