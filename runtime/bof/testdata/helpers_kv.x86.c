/*
 * helpers_kv.x86.c — i386 BOF fixture exercising the Helpers
 * (BeaconGetCustomUserData, BeaconGetSpawnTo, toWideChar) and KV
 * (BeaconAddValue / GetValue / RemoveValue) families.
 *
 * Output is one line per assertion, semicolon-separated, so the
 * parent can grep for each contract:
 *   userdata=<bytes>;spawnto=<path>;kv-after-add=ok;kv-after-remove=missing;wide=H,e,l
 *
 * Build (committed as helpers_kv.x86.o):
 *   i686-w64-mingw32-gcc -m32 -O2 -fno-asynchronous-unwind-tables \
 *     -fno-unwind-tables -fno-ident -ffreestanding -c \
 *     helpers_kv.x86.c -o helpers_kv.x86.o
 */

__declspec(dllimport) void  __cdecl BeaconPrintf(int type, const char *fmt, ...);
__declspec(dllimport) void  __cdecl BeaconOutput(int type, const char *data, int len);
__declspec(dllimport) void  __cdecl BeaconGetCustomUserData(char **buf, int *len);
__declspec(dllimport) void  __cdecl BeaconGetSpawnTo(int x86, char *buf, int len);
__declspec(dllimport) int   __cdecl toWideChar(const char *src, unsigned short *dst, int max);

__declspec(dllimport) void  __cdecl BeaconAddValue(const char *key, void *val);
__declspec(dllimport) void *__cdecl BeaconGetValue(const char *key);
__declspec(dllimport) void  __cdecl BeaconRemoveValue(const char *key);

void __cdecl go(char *args, int alen)
{
    (void)args;
    (void)alen;

    /* 1. UserData round-trip */
    char *ud = 0;
    int ud_len = 0;
    BeaconGetCustomUserData(&ud, &ud_len);
    BeaconPrintf(0, "userdata=");
    if (ud_len > 0 && ud) {
        BeaconOutput(0, ud, ud_len);
    } else {
        BeaconPrintf(0, "(none)");
    }
    BeaconPrintf(0, ";");

    /* 2. SpawnTo path */
    char st[64];
    BeaconGetSpawnTo(0, st, sizeof(st));
    BeaconPrintf(0, "spawnto=");
    BeaconPrintf(0, st[0] ? st : "(empty)");
    BeaconPrintf(0, ";");

    /* 3. KV — add then read back */
    char one = '1';
    BeaconAddValue("kv-test", &one);
    void *got = BeaconGetValue("kv-test");
    BeaconPrintf(0, "kv-after-add=");
    BeaconPrintf(0, got == &one ? "ok" : "missing");
    BeaconPrintf(0, ";");

    /* 4. KV — remove, expect missing */
    BeaconRemoveValue("kv-test");
    got = BeaconGetValue("kv-test");
    BeaconPrintf(0, "kv-after-remove=");
    BeaconPrintf(0, got == 0 ? "missing" : "still-present");
    BeaconPrintf(0, ";");

    /* 5. toWideChar — convert "Hel" → 6 bytes UTF-16LE, output low
     * bytes (the high bytes are zero for ASCII so visible chars
     * are just H,e,l separated by commas). */
    unsigned short w[8];
    toWideChar("Hel", w, 8);
    BeaconPrintf(0, "wide=");
    char comma = ',';
    for (int i = 0; w[i] != 0 && i < 8; i++) {
        char c = (char)(w[i] & 0xFF);
        BeaconOutput(0, &c, 1);
        if (w[i + 1] != 0 && i < 7) BeaconOutput(0, &comma, 1);
    }
}
