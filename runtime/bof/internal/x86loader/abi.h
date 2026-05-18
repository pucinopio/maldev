/*
 * abi.h — shared control block between the Go-side orchestrator
 * (runtime/bof/x86fork_windows.go, slice 1.d phase B) and the C-side
 * 32-bit loader DLL (this directory, slice 1.d phase C).
 *
 * The Go side mirrors this layout byte-for-byte in
 * runtime/bof/x86control_windows.go. Any change here MUST land in
 * the Go mirror in the same commit, and the version bump rule
 * applies (see BOF_CTRL_VERSION below).
 *
 * Field order is fixed. Use only fixed-width integers (uint32_t)
 * so the layout is identical on both sides of the IPC. Pointer-
 * width fields are 32-bit because the loader runs in a WoW64
 * (x86) process — every remote address fits in a uint32_t.
 *
 * All multi-byte fields are little-endian (Windows-native).
 */

#ifndef MALDEV_RUNTIME_BOF_X86LOADER_ABI_H
#define MALDEV_RUNTIME_BOF_X86LOADER_ABI_H

#include <stdint.h>

/* 'BC86' in little-endian — parent stamps this before injection so the
 * loader can refuse to run on a corrupted / mis-injected block. */
#define BOF_CTRL_MAGIC      0x36384342u

/* Bump on every wire-incompatible change. The loader checks both magic
 * and version; mismatches surface as BOF_STATUS_ABI_MISMATCH so the
 * parent gets a specific, actionable error rather than a generic
 * "remote thread exited with 0xC0000005". */
#define BOF_CTRL_VERSION    1u

/* status word (loader → parent). 32-bit little-endian. */
typedef enum {
    BOF_STATUS_PENDING       = 0,  /* parent wrote the block, loader hasn't touched it yet */
    BOF_STATUS_RUNNING       = 1,  /* loader started execution */
    BOF_STATUS_DONE          = 2,  /* loader finished, output buffers are valid */
    BOF_STATUS_ABI_MISMATCH  = 3,  /* magic or version did not match */
    BOF_STATUS_LOAD_FAILED   = 4,  /* parse/alloc/reloc failed inside the loader */
    BOF_STATUS_BOF_CRASHED   = 5,  /* loader caught a SEH exception from the BOF body */
} bof_status_t;

/* Shared control block. Parent allocates this in the child via
 * VirtualAllocEx, writes the layout below, then passes its address
 * to BOFExec as the thread parameter (CreateRemoteThread's
 * lpParameter). Loader writes status + output lengths in place and
 * the parent ReadProcessMemory's the result after WaitForSingleObject.
 *
 * Buffer layout for inputs/outputs: the parent VirtualAllocEx's a
 * SEPARATE region per buffer (bof_*, args_*, out_*, err_*) so the
 * loader and the parent can freely VirtualProtect/Free them
 * independently. The control block holds the remote addresses, not
 * the bytes.
 */
typedef struct {
    /* Identity ---------------------------------------------------- */
    uint32_t magic;            /* BOF_CTRL_MAGIC */
    uint32_t version;          /* BOF_CTRL_VERSION */

    /* Status (loader writes) -------------------------------------- */
    uint32_t status;           /* bof_status_t */
    uint32_t error_code;       /* GetLastError() / SEH code on failure paths */

    /* Inputs (parent writes) -------------------------------------- */
    uint32_t bof_addr;         /* remote address of the BOF .o bytes */
    uint32_t bof_len;
    uint32_t args_addr;        /* remote address of the BeaconDataPack args (may be 0) */
    uint32_t args_len;
    uint32_t entry_off;        /* reserved — 0 = use "go" symbol; non-zero = explicit offset */
    uint32_t spawn_to_addr;    /* remote address of NUL-terminated UTF-8 SpawnTo path (0 = none) */
    uint32_t user_data_addr;   /* remote address of BeaconGetCustomUserData blob (0 = none) */
    uint32_t user_data_len;

    /* Outputs (parent allocates, loader fills lengths) ------------ */
    uint32_t out_addr;         /* remote address of the output buffer */
    uint32_t out_capacity;     /* bytes the parent allocated */
    uint32_t out_len;          /* bytes the loader wrote */

    uint32_t err_addr;         /* remote address of the error buffer (BeaconErrorD/DD/NA) */
    uint32_t err_capacity;
    uint32_t err_len;

    /* Reserved for future use. Keeps the struct fixed-size so a
     * v1 parent can read a v2 control block (or vice-versa) up to
     * the shared prefix without overflow. */
    uint32_t reserved[16];
} bof_control_t;

/* BOFExec is the loader's single entry point. CreateRemoteThread
 * targets this address with `ctrl` as the thread parameter. The
 * return value is mirrored into ctrl->status before exit; callers
 * should rely on the control block, not the thread exit code, for
 * structured error reporting.
 *
 * Calling convention: __stdcall (the Win32 thread proc ABI).
 */
__declspec(dllexport) uint32_t __stdcall BOFExec(bof_control_t *ctrl);

#endif /* MALDEV_RUNTIME_BOF_X86LOADER_ABI_H */
