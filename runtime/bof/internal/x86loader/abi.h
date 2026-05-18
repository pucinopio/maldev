/*
 * abi.h — wire format between the Go-side orchestrator
 * (runtime/bof/x86fork_windows.go, slice 1.d phase B-bis) and the
 * C-side 32-bit shellcode loader (loader.c, this directory).
 *
 * Architecture: no disk, no LoadLibrary. The Go orchestrator
 * spawns a benign WoW64 host (rundll32 with no args / notepad
 * x86 / …) suspended, VirtualAllocEx's three regions in it
 * (loader code RX, the read-only inputs block, the read-write
 * IO block), WriteProcessMemory's the loader shellcode + the
 * BOF .o + the args + a parameter struct, then
 * CreateRemoteThread targets the loader entry with the
 * parameter address as lpThreadParameter. The loader runs the
 * BOF in-process inside the child and writes captured output /
 * errors into the IO block, which the parent reads back via
 * ReadProcessMemory before terminating the child.
 *
 * The struct below is the per-invocation parameter block —
 * mirrored byte-for-byte in Go (`x86fork_windows.go: loaderParams`).
 * Any layout change MUST land in both sides in the same commit.
 * Bump LOADER_ABI_VERSION on any wire-incompatible change.
 */

#ifndef MALDEV_RUNTIME_BOF_X86LOADER_ABI_H
#define MALDEV_RUNTIME_BOF_X86LOADER_ABI_H

#include <stdint.h>

/* Magic stamped by the parent so the shellcode refuses to run on
 * a corrupted / mis-injected param block. 'BC86' little-endian. */
#define LOADER_ABI_MAGIC    0x36384342u

/* Bump on any wire-incompatible change. The shellcode checks magic
 * + version; mismatches surface as a specific status code. */
#define LOADER_ABI_VERSION  1u

/* Status codes the loader writes into params.status before exit.
 * Mirrored as untyped consts on the Go side. */
typedef enum {
    LOADER_STATUS_PENDING       = 0,  /* parent wrote the block, loader hasn't started */
    LOADER_STATUS_RUNNING       = 1,  /* loader entered the BOF execution path */
    LOADER_STATUS_DONE          = 2,  /* loader finished cleanly; out/err lengths valid */
    LOADER_STATUS_ABI_MISMATCH  = 3,  /* magic or version mismatch — refused to run */
    LOADER_STATUS_RESOLVE_FAIL  = 4,  /* PEB walk / ROR13 could not resolve a kernel32 symbol */
    LOADER_STATUS_LOAD_FAIL     = 5,  /* COFF parse / alloc / reloc failed inside the BOF body */
    LOADER_STATUS_BOF_CRASHED   = 6,  /* SEH caught a fault inside the BOF call */
} loader_status_t;

/* Per-invocation parameter block written by the parent into the
 * child's IO region. The loader_addr / loader_len fields are
 * absent because the parent passes the params pointer directly
 * via CreateRemoteThread's lpThreadParameter — the shellcode
 * doesn't need to find itself.
 *
 * Pointer-width fields are uint32_t (everything is 32-bit in
 * WoW64). Multi-byte fields are little-endian.
 */
typedef struct {
    /* Identity --------------------------------------------------- */
    uint32_t magic;             /* LOADER_ABI_MAGIC */
    uint32_t version;           /* LOADER_ABI_VERSION */

    /* Status (loader writes) ------------------------------------- */
    uint32_t status;            /* loader_status_t */
    uint32_t error_code;        /* SEH exception code / GetLastError on failure paths */

    /* Inputs (parent writes) ------------------------------------- */
    uint32_t bof_addr;          /* BOF .o bytes (remote address in child) */
    uint32_t bof_len;
    uint32_t args_addr;         /* BeaconDataPack args (may be 0) */
    uint32_t args_len;
    uint32_t user_data_addr;    /* BeaconGetCustomUserData blob (may be 0) */
    uint32_t user_data_len;
    uint32_t spawn_to_addr;     /* NUL-terminated UTF-8 SpawnTo path (may be 0) */

    /* Outputs (parent allocates buffers, loader fills *_len) ---- */
    uint32_t out_addr;          /* output buffer (BeaconPrintf / BeaconOutput) */
    uint32_t out_cap;
    uint32_t out_len;
    uint32_t err_addr;          /* error buffer (BeaconErrorD / DD / NA) */
    uint32_t err_cap;
    uint32_t err_len;

    /* Reserved tail — keeps struct fixed-size so a v1 parent can
     * read a v2 control block prefix without overrunning. */
    uint32_t reserved[12];
} loader_params_t;

#endif /* MALDEV_RUNTIME_BOF_X86LOADER_ABI_H */
