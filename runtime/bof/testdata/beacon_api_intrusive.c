// beacon_api_intrusive.c — exercises the process-spawn and
// shellcode-injection Beacon API. Gated MALDEV_INTRUSIVE=1
// on the Go side because each invocation:
//   - creates a real child process (rundll32 by default)
//   - writes shellcode into the child
//   - resumes the thread which executes the payload
//   - terminates + closes the child handles
//
// Beacon API covered:
//   - BeaconSpawnTemporaryProcess (suspended child)
//   - BeaconInjectTemporaryProcess (write payload + resume)
//   - BeaconCleanupProcess (terminate + handle close)
//   - BeaconInjectProcess (write into an existing handle)
//
// Payload: a single 0xC3 (RET) instruction. The spawned thread
// returns immediately and the child exits cleanly with whatever
// the CRT does after a 0-byte main. Tests the dispatch path, not
// the payload behaviour.
//
// Build:
//   x86_64-w64-mingw32-gcc -c beacon_api_intrusive.c \
//       -o beacon_api_intrusive.o -O2 -Wall \
//       -ffreestanding -fno-stack-protector

#include <windows.h>

__declspec(dllimport) void  BeaconPrintf(int type, const char *fmt, ...);
__declspec(dllimport) BOOL  BeaconSpawnTemporaryProcess(BOOL bIgnoreToken, BOOL bAlloc, void *si, void *pi);
__declspec(dllimport) void  BeaconInjectTemporaryProcess(void *pi, char *payload, int p_len, int p_offset, char *arg, int a_len);
__declspec(dllimport) void  BeaconCleanupProcess(void *pi);
__declspec(dllimport) void  BeaconInjectProcess(HANDLE proc, int pid, char *payload, int p_len, int p_offset, char *arg, int a_len);

__declspec(dllimport) DWORD  KERNEL32$GetCurrentProcessId(void);
__declspec(dllimport) HANDLE KERNEL32$GetCurrentProcess(void);

#define CALLBACK_OUTPUT 0x0

// processInfo mirrors the loader's struct (runtime/bof/beacon_api_extra_windows.go).
typedef struct {
    HANDLE hProcess;
    HANDLE hThread;
    DWORD  pid;
    DWORD  tid;
} BeaconProcInfo;

// Single-byte RET — the simplest valid x64 thread entry.
// CreateRemoteThread invokes this; the thread returns immediately.
static unsigned char payload_ret[] = { 0xC3 };

void go(char *args, int len) {
    (void)args; (void)len;

    // --- 1. SpawnTemporaryProcess -----------------------------------
    BeaconProcInfo pi = {0};
    BOOL spawned = BeaconSpawnTemporaryProcess(FALSE, TRUE, NULL, &pi);
    BeaconPrintf(CALLBACK_OUTPUT,
                 "spawn_temp=%d pid=%d tid=%d hProc_nonnull=%d\n",
                 spawned, pi.pid, pi.tid, pi.hProcess != NULL);
    if (!spawned) {
        return;
    }

    // --- 2. InjectTemporaryProcess ----------------------------------
    // Writes payload_ret into pi.hProcess and resumes pi.hThread.
    // No arg buffer for the 0xC3 payload.
    BeaconInjectTemporaryProcess(&pi,
                                  (char *)payload_ret,
                                  sizeof(payload_ret),
                                  0,    // offset
                                  NULL, // arg buffer
                                  0);   // arg length
    BeaconPrintf(CALLBACK_OUTPUT, "inject_temp=dispatched\n");

    // --- 3. CleanupProcess ------------------------------------------
    // Terminate the child (in case the RET didn't bring it down) and
    // close the cached handles.
    BeaconCleanupProcess(&pi);
    BeaconPrintf(CALLBACK_OUTPUT, "cleanup_temp=done\n");

    // --- 4. InjectProcess into our OWN handle -----------------------
    // BeaconInjectProcess writes into an arbitrary process handle.
    // Using the current process is the safest fixture: the payload
    // 0xC3 (RET) runs in a fresh CreateRemoteThread context inside
    // our own process and exits the thread without disturbing the
    // host. PID is informational on our side; the handle drives the
    // VirtualAllocEx / WPM / CreateRemoteThread chain.
    BeaconInjectProcess(KERNEL32$GetCurrentProcess(),
                         (int)KERNEL32$GetCurrentProcessId(),
                         (char *)payload_ret,
                         sizeof(payload_ret),
                         0, NULL, 0);
    BeaconPrintf(CALLBACK_OUTPUT, "inject_proc_self=dispatched\n");
}
