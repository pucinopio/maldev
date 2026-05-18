/*
 * loader.c — slice 1.d phase C step 0 skeleton (rundll32-host model).
 *
 * Compiled into bof_x86_loader.x86.dll; rundll32.exe (SysWOW64) is
 * the launcher:
 *     rundll32 bof_x86_loader.x86.dll,BOFExec v=1 bof=… args=… out=… err=…
 *
 * rundll32 LoadLibrary's us, then calls BOFExec with the post-comma
 * portion of its argv concatenated as lpCmdLine. We parse the
 * key=value tokens, copy the input files into memory, run the BOF
 * (phase C step 1), and write the captured output / errors back to
 * the same files. The thread returns, rundll32 ExitProcess's, and
 * the parent's WaitForSingleObject unblocks.
 *
 * This skeleton compiles, exports BOFExec with the rundll32-required
 * signature, parses the protocol line, and acks via BOF_EXIT_DONE
 * after writing a fixed "x86 BOF loader skeleton (phase C step 0)"
 * banner into the output file. The real COFF parser + Beacon API
 * impl land in step 1.
 *
 * Build: scripts/build-bof-x86-loader.sh (Podman fallback if
 * i686-w64-mingw32-gcc isn't on PATH).
 */

#include <windows.h>
#include <stdint.h>

#include "abi.h"

BOOL WINAPI DllMain(HINSTANCE hinst, DWORD reason, LPVOID reserved)
{
    (void)hinst;
    (void)reason;
    (void)reserved;
    return TRUE;
}

/* find_value scans lpCmdLine for the first occurrence of "key=" and
 * returns a pointer to the start of the value (terminated by a
 * space or NUL). The value is NOT copied; the caller is expected to
 * use it before lpCmdLine goes out of scope (rundll32 keeps the
 * argv block alive until BOFExec returns).
 *
 * key must be NUL-terminated and not contain '=' or ' '. Returns
 * NULL when the token is missing. The "key=" prefix matching is
 * left-anchored to a word boundary so e.g. "user-data=" doesn't
 * accidentally match "data=".
 */
static const char *find_value(const char *line, const char *key, int *outlen)
{
    if (!line || !key) return NULL;

    int klen = 0;
    while (key[klen] != '\0') klen++;

    const char *p = line;
    while (*p) {
        while (*p == ' ') p++;
        if (*p == '\0') break;

        const char *tok = p;
        while (*p != ' ' && *p != '\0') p++;

        /* tok..p is one token. Match "key=…". */
        if ((p - tok) > klen && tok[klen] == '=') {
            int i;
            for (i = 0; i < klen; i++) {
                if (tok[i] != key[i]) break;
            }
            if (i == klen) {
                *outlen = (int)((p - tok) - (klen + 1));
                return tok + klen + 1;
            }
        }
    }
    return NULL;
}

/* copy_value writes value[0..vlen) into dst as a NUL-terminated
 * string. Returns 0 when vlen exceeds dst_cap-1, 1 on success. The
 * orchestrator pins file paths under %TEMP% with 8.3-style short
 * names so MAX_PATH=260 covers every realistic input. */
static int copy_value(char *dst, int dst_cap, const char *value, int vlen)
{
    if (vlen < 0 || vlen >= dst_cap) return 0;
    for (int i = 0; i < vlen; i++) dst[i] = value[i];
    dst[vlen] = '\0';
    return 1;
}

/* read_file maps the contents of `path` into a newly-allocated
 * buffer and returns its length via *outlen. Buffer is owned by
 * the caller (HeapFree on the process heap). Returns NULL on any
 * error; the caller surfaces the appropriate BOF_EXIT_* code. */
static uint8_t *read_file(const char *path, uint32_t *outlen)
{
    HANDLE f = CreateFileA(path, GENERIC_READ, FILE_SHARE_READ, NULL,
                           OPEN_EXISTING, FILE_ATTRIBUTE_NORMAL, NULL);
    if (f == INVALID_HANDLE_VALUE) return NULL;

    LARGE_INTEGER size;
    if (!GetFileSizeEx(f, &size) || size.HighPart != 0 || size.LowPart == 0) {
        CloseHandle(f);
        if (size.HighPart == 0 && size.LowPart == 0) {
            /* Empty file is legal (args may be absent). Return a non-NULL
             * sentinel so callers can distinguish "empty" from "error". */
            *outlen = 0;
            return (uint8_t *)HeapAlloc(GetProcessHeap(), 0, 1);
        }
        return NULL;
    }

    uint8_t *buf = (uint8_t *)HeapAlloc(GetProcessHeap(), 0, size.LowPart);
    if (!buf) {
        CloseHandle(f);
        return NULL;
    }

    DWORD got = 0;
    if (!ReadFile(f, buf, size.LowPart, &got, NULL) || got != size.LowPart) {
        HeapFree(GetProcessHeap(), 0, buf);
        CloseHandle(f);
        return NULL;
    }

    CloseHandle(f);
    *outlen = size.LowPart;
    return buf;
}

/* write_file truncates `path` to zero length then writes `len`
 * bytes from `buf` into it. Returns 1 on success, 0 on any error. */
static int write_file(const char *path, const void *buf, uint32_t len)
{
    HANDLE f = CreateFileA(path, GENERIC_WRITE, 0, NULL,
                           CREATE_ALWAYS, FILE_ATTRIBUTE_NORMAL, NULL);
    if (f == INVALID_HANDLE_VALUE) return 0;

    if (len > 0) {
        DWORD wrote = 0;
        if (!WriteFile(f, buf, len, &wrote, NULL) || wrote != len) {
            CloseHandle(f);
            return 0;
        }
    }
    CloseHandle(f);
    return 1;
}

/* BOFExec — rundll32 entry point.
 *
 * rundll32 ABI: __stdcall, four arguments:
 *   HWND hwnd, HINSTANCE hinst, LPSTR lpCmdLine, int nCmdShow.
 *
 * lpCmdLine carries the protocol line described in abi.h. Return
 * value is the process exit code (rundll32 calls ExitProcess on it).
 */
__declspec(dllexport) void __stdcall BOFExec(HWND hwnd, HINSTANCE hinst,
                                              LPSTR lpCmdLine, int nCmdShow)
{
    (void)hwnd;
    (void)hinst;
    (void)nCmdShow;

    char bof_path[MAX_PATH], args_path[MAX_PATH];
    char out_path[MAX_PATH], err_path[MAX_PATH];
    int vlen;
    const char *v;

    if (lpCmdLine == NULL) ExitProcess(BOF_EXIT_BAD_PROTOCOL);

    /* Version check ------------------------------------------------ */
    v = find_value(lpCmdLine, "v", &vlen);
    if (!v || vlen != 1 || v[0] != ('0' + BOF_PROTO_VERSION)) {
        ExitProcess(BOF_EXIT_BAD_VERSION);
    }

    /* Required paths ----------------------------------------------- */
    v = find_value(lpCmdLine, "bof", &vlen);
    if (!v || !copy_value(bof_path, sizeof(bof_path), v, vlen)) {
        ExitProcess(BOF_EXIT_BAD_PROTOCOL);
    }
    v = find_value(lpCmdLine, "args", &vlen);
    if (!v || !copy_value(args_path, sizeof(args_path), v, vlen)) {
        ExitProcess(BOF_EXIT_BAD_PROTOCOL);
    }
    v = find_value(lpCmdLine, "out", &vlen);
    if (!v || !copy_value(out_path, sizeof(out_path), v, vlen)) {
        ExitProcess(BOF_EXIT_BAD_PROTOCOL);
    }
    v = find_value(lpCmdLine, "err", &vlen);
    if (!v || !copy_value(err_path, sizeof(err_path), v, vlen)) {
        ExitProcess(BOF_EXIT_BAD_PROTOCOL);
    }

    /* Inputs ------------------------------------------------------- */
    uint32_t bof_len = 0, args_len = 0;
    uint8_t *bof_bytes = read_file(bof_path, &bof_len);
    if (!bof_bytes) ExitProcess(BOF_EXIT_OPEN_BOF_FAILED);

    uint8_t *args_bytes = read_file(args_path, &args_len);
    if (!args_bytes) {
        HeapFree(GetProcessHeap(), 0, bof_bytes);
        ExitProcess(BOF_EXIT_OPEN_ARGS_FAILED);
    }

    /* Phase C step 1 will replace this banner with the actual
     * COFF parser + relocation engine + Beacon API call site. Until
     * then we ack so the orchestrator can validate the IPC + the
     * temp-file round trip end to end. */
    const char banner[] = "[x86 BOF loader skeleton — phase C step 0]\n"
                          "bof_len=";
    char buf[256];
    int n = 0;
    for (size_t i = 0; i < sizeof(banner) - 1; i++) buf[n++] = banner[i];

    /* itoa bof_len */
    char num[16];
    int ni = 0;
    uint32_t x = bof_len;
    if (x == 0) num[ni++] = '0';
    else { while (x) { num[ni++] = '0' + (x % 10); x /= 10; } }
    for (int i = ni - 1; i >= 0; i--) buf[n++] = num[i];
    buf[n++] = ' ';
    buf[n++] = 'a'; buf[n++] = 'r'; buf[n++] = 'g'; buf[n++] = 's'; buf[n++] = '_';
    buf[n++] = 'l'; buf[n++] = 'e'; buf[n++] = 'n'; buf[n++] = '=';
    ni = 0;
    x = args_len;
    if (x == 0) num[ni++] = '0';
    else { while (x) { num[ni++] = '0' + (x % 10); x /= 10; } }
    for (int i = ni - 1; i >= 0; i--) buf[n++] = num[i];
    buf[n++] = '\n';

    int ok_out = write_file(out_path, buf, (uint32_t)n);
    int ok_err = write_file(err_path, "", 0);

    HeapFree(GetProcessHeap(), 0, bof_bytes);
    HeapFree(GetProcessHeap(), 0, args_bytes);

    if (!ok_out) ExitProcess(BOF_EXIT_OPEN_OUT_FAILED);
    if (!ok_err) ExitProcess(BOF_EXIT_OPEN_ERR_FAILED);

    ExitProcess(BOF_EXIT_DONE);
}
