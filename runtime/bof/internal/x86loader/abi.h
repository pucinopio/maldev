/*
 * abi.h — wire format between the Go-side orchestrator
 * (runtime/bof/x86fork_windows.go, slice 1.d phase B) and the C-side
 * 32-bit loader DLL (this directory, slice 1.d phase C).
 *
 * Architecture: the Go orchestrator writes four temp files (the BOF
 * .o, the BeaconDataPack args blob, an empty output file, an empty
 * error file), spawns rundll32.exe (SysWOW64) with a composed
 * command line of the form
 *     rundll32 <loader.dll>,BOFExec <protocol-line>
 * and waits for rundll32 to exit. The loader DLL parses
 * <protocol-line> from lpCmdLine, reads the inputs, runs the BOF
 * in-process, writes the outputs back to the same files, and
 * returns. The parent reads the output files and unlinks all four.
 *
 * Why rundll32-as-host: rundll32 handles LoadLibrary + GetProcAddress
 * + the calling convention for us, so the orchestrator needs only
 * CreateProcess + WaitForSingleObject — no remote-memory walking,
 * no chasing kernel32!LoadLibraryA's address inside a WoW64 child.
 * Phase D (queued) will swap rundll32 for a reflective injector
 * once step 1 (the real COFF parser + Beacon API impl) lands; the
 * loader body stays the same.
 *
 * Wire format (single ASCII line passed to lpCmdLine):
 *     v=1 bof=<utf8-path> args=<utf8-path> out=<utf8-path> err=<utf8-path>
 *     [spawnto=<utf8-path>] [user-data=<utf8-path>] [entry=<symbol>]
 *
 * Tokens are space-separated; values do not contain spaces (the
 * orchestrator owns the temp paths and uses 8.3-style file names
 * under %TEMP% to dodge quoting hell). Unknown tokens are ignored
 * for forward-compat.
 *
 * Status: the loader's return value goes through rundll32's
 * ExitProcess; the orchestrator interprets the GetExitCodeProcess
 * value as a bof_status_t. 0 = DONE, non-zero = an error code from
 * the enum below.
 */

#ifndef MALDEV_RUNTIME_BOF_X86LOADER_ABI_H
#define MALDEV_RUNTIME_BOF_X86LOADER_ABI_H

/* Protocol version. Bump on any wire-incompatible change so the
 * orchestrator + loader can refuse mismatched pairings instead of
 * crashing into raw memory. */
#define BOF_PROTO_VERSION  1

/* Exit codes the loader feeds rundll32 → GetExitCodeProcess. */
typedef enum {
    BOF_EXIT_DONE             = 0,
    BOF_EXIT_BAD_PROTOCOL     = 1,
    BOF_EXIT_BAD_VERSION      = 2,
    BOF_EXIT_OPEN_BOF_FAILED  = 3,
    BOF_EXIT_OPEN_ARGS_FAILED = 4,
    BOF_EXIT_OPEN_OUT_FAILED  = 5,
    BOF_EXIT_OPEN_ERR_FAILED  = 6,
    BOF_EXIT_LOAD_FAILED      = 7,
    BOF_EXIT_BOF_CRASHED      = 8,
} bof_exit_t;

#endif /* MALDEV_RUNTIME_BOF_X86LOADER_ABI_H */
