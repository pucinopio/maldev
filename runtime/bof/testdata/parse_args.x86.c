/*
 * parse_args.x86.c — i386 BOF fixture exercising the Data parsing
 * + Format families of the Beacon API.
 *
 * Args wire format (BeaconDataPack-compatible):
 *   uint32 totalLength
 *   uint32 n (int)
 *   uint32 strLen
 *   bytes  s (strLen bytes)
 *
 * The BOF reads n + s, builds a format buffer "n=<n>:s=<s>", and
 * emits it via BeaconOutput.
 *
 * Build (committed as parse_args.x86.o):
 *   i686-w64-mingw32-gcc -m32 -O2 -fno-asynchronous-unwind-tables \
 *     -fno-unwind-tables -fno-ident -ffreestanding -c \
 *     parse_args.x86.c -o parse_args.x86.o
 */

typedef struct { char *o; char *b; int l; int s; } datap_t;
typedef struct { char *o; char *b; int l; int s; } formatp_t;

__declspec(dllimport) void __cdecl BeaconDataParse(datap_t *p, char *buf, int size);
__declspec(dllimport) int  __cdecl BeaconDataInt(datap_t *p);
__declspec(dllimport) char *__cdecl BeaconDataExtract(datap_t *p, int *size);

__declspec(dllimport) void  __cdecl BeaconFormatAlloc(formatp_t *fp, int maxsz);
__declspec(dllimport) void  __cdecl BeaconFormatFree(formatp_t *fp);
__declspec(dllimport) void  __cdecl BeaconFormatAppend(formatp_t *fp, const char *src, int len);
__declspec(dllimport) void  __cdecl BeaconFormatInt(formatp_t *fp, int value);
__declspec(dllimport) char *__cdecl BeaconFormatToString(formatp_t *fp, int *size);

__declspec(dllimport) void __cdecl BeaconOutput(int type, const char *data, int len);

void __cdecl go(char *args, int alen)
{
    datap_t p;
    formatp_t f;

    BeaconDataParse(&p, args, alen);
    int n = BeaconDataInt(&p);
    int strLen;
    char *s = BeaconDataExtract(&p, &strLen);

    BeaconFormatAlloc(&f, 128);
    static const char p1[] = "n=";
    static const char p2[] = ":s=";
    BeaconFormatAppend(&f, p1, sizeof(p1) - 1);
    BeaconFormatInt(&f, n);
    BeaconFormatAppend(&f, p2, sizeof(p2) - 1);
    BeaconFormatAppend(&f, s, strLen);

    int outLen;
    char *out = BeaconFormatToString(&f, &outLen);
    BeaconOutput(0, out, outLen);

    BeaconFormatFree(&f);
}
