// beacon_api_complete.c — exercises the Beacon API functions
// that no existing in-tree BOF touches:
//
//   - BeaconGetCustomUserData (Spec.UserData round-trip)
//   - BeaconRemoveValue (KV store delete + idempotent re-delete)
//   - BeaconGetOutputData (introspect accumulated output)
//
// Together with parse_args / data_extras / format_output /
// format_extras / error_spawnto / realworld_calls the in-tree
// suite reaches every documented Beacon API symbol.
//
// Build:
//   x86_64-w64-mingw32-gcc -c beacon_api_complete.c \
//       -o beacon_api_complete.o -O2 -Wall \
//       -ffreestanding -fno-stack-protector

#include <windows.h>

typedef struct { char *original; char *buffer; int length; int size; } datap;

__declspec(dllimport) void  BeaconPrintf(int type, const char *fmt, ...);
__declspec(dllimport) void  BeaconGetCustomUserData(char **buf, int *len);
__declspec(dllimport) BOOL  BeaconAddValue(const char *key, void *ptr);
__declspec(dllimport) void *BeaconGetValue(const char *key);
__declspec(dllimport) BOOL  BeaconRemoveValue(const char *key);
__declspec(dllimport) char *BeaconGetOutputData(int *outlen);

#define CALLBACK_OUTPUT 0x0

void go(char *args, int len) {
    (void)args; (void)len;

    // --- BeaconGetCustomUserData ------------------------------------
    // The Spec.UserData bytes the operator supplied at Run time come
    // back via this call. Empty payload yields (NULL, 0). The BOF
    // surfaces both as %d to keep the assertion stable across runs.
    char *ud = NULL;
    int   udlen = 0;
    BeaconGetCustomUserData(&ud, &udlen);
    BeaconPrintf(CALLBACK_OUTPUT, "userdata_len=%d ptr_nonnull=%d\n",
                 udlen, ud != NULL);

    // --- BeaconRemoveValue ------------------------------------------
    // Add then Remove then re-Get: the post-condition is that the key
    // is absent regardless of whether it was present. We also exercise
    // the idempotent path (re-remove a missing key) — the No-Consolation
    // BOOL extension makes Remove return TRUE on the empty path too.
    int sentinel = 0x5ECAFE;
    BOOL added   = BeaconAddValue("complete_test", &sentinel);
    void *before = BeaconGetValue("complete_test");
    BOOL removed = BeaconRemoveValue("complete_test");
    void *after  = BeaconGetValue("complete_test");
    BOOL reremove = BeaconRemoveValue("complete_test");
    BeaconPrintf(CALLBACK_OUTPUT,
                 "kv_add=%d before=%d removed=%d after=%d reremove=%d\n",
                 added, before == &sentinel, removed,
                 after == NULL, reremove);

    // --- BeaconGetOutputData ----------------------------------------
    // Returns the buffer captured by BeaconPrintf / BeaconOutput so
    // far. Since this BOF has already printed two lines, outlen > 0
    // and the buffer's first bytes should match "userdata_len=".
    int outlen = 0;
    char *outdata = BeaconGetOutputData(&outlen);
    BeaconPrintf(CALLBACK_OUTPUT, "outdata_len_pos=%d outdata_nonnull=%d\n",
                 outlen > 0, outdata != NULL);
}
