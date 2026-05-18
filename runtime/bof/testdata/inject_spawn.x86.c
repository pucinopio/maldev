/*
 * inject_spawn.x86.c — i386 BOF fixture exercising the Inject /
 * Spawn family (step 1.h).
 *
 * The BOF asks the loader to spawn its configured SpawnTo (a
 * suspended process) via BeaconSpawnTemporaryProcess, reads the
 * PID back, then immediately tears the child down via
 * BeaconCleanupProcess. Output:
 *   spawn=ok pid=<decimal>;cleanup=done
 *
 * Build (committed as inject_spawn.x86.o):
 *   i686-w64-mingw32-gcc -m32 -O2 -fno-asynchronous-unwind-tables \
 *     -fno-unwind-tables -fno-ident -ffreestanding -c \
 *     inject_spawn.x86.c -o inject_spawn.x86.o
 */

typedef struct {
    void *hProcess;
    void *hThread;
    unsigned int dwProcessId;
    unsigned int dwThreadId;
} PROCESS_INFO;

typedef struct {
    unsigned int cb;
    char *lpReserved;
    char *lpDesktop;
    char *lpTitle;
    unsigned int dwX, dwY, dwXSize, dwYSize;
    unsigned int dwXCountChars, dwYCountChars, dwFillAttribute;
    unsigned int dwFlags;
    unsigned short wShowWindow, cbReserved2;
    unsigned char *lpReserved2;
    void *hStdInput, *hStdOutput, *hStdError;
} STARTUP_INFO;

__declspec(dllimport) void __cdecl BeaconPrintf(int type, const char *fmt, ...);
__declspec(dllimport) int  __cdecl BeaconSpawnTemporaryProcess(
    int bIgnoreToken, int bAlloc, STARTUP_INFO *si, PROCESS_INFO *pi);
__declspec(dllimport) void __cdecl BeaconCleanupProcess(PROCESS_INFO *pi);

void __cdecl go(char *args, int alen)
{
    (void)args;
    (void)alen;

    STARTUP_INFO si;
    PROCESS_INFO pi;
    /* Zero both structs — required by CreateProcessA. */
    char *zs = (char *)&si;
    char *zp = (char *)&pi;
    for (int i = 0; i < (int)sizeof(si); i++) zs[i] = 0;
    for (int i = 0; i < (int)sizeof(pi); i++) zp[i] = 0;
    si.cb = sizeof(si);

    int ok = BeaconSpawnTemporaryProcess(0, 0, &si, &pi);
    if (!ok || !pi.hProcess) {
        BeaconPrintf(0, "spawn=fail;");
        return;
    }
    BeaconPrintf(0, "spawn=ok pid=%u;", pi.dwProcessId);

    BeaconCleanupProcess(&pi);
    BeaconPrintf(0, "cleanup=done");
}
