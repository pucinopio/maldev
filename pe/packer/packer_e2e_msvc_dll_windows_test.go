//go:build windows && maldev_packer_run_e2e

package packer_test

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/oioio-space/maldev/pe/packer"
)

// TestPackBinary_FormatWindowsDLL_MSVC_E2E packs an MSVC-built test
// DLL and exercises its exports post-LoadLibrary. Closes Item #7
// from docs/refactor-2026-doc/packer-actions-2026-05-12.md — the
// fixture was previously absent because the Win10 VM lacked an MSVC
// toolchain. Provisioned via scripts/vm-provision.sh extension.
//
// What this validates that the synthetic BuildDLLWithReloc fixture
// can't:
//   - MSVC import-table layout (vcruntime140 / ucrtbase imports
//     stay resolvable after .text mutation + stub append).
//   - .pdata + .xdata SEH unwind metadata survive the section-
//     table append (the loader will refuse a DLL whose unwind
//     tables RVA is past SizeOfImage or overlaps the new stub).
//   - Real `__declspec(dllexport)` exports are reachable via
//     GetProcAddress and their bodies execute after stub decryption.
//   - /GS stack-cookie initialisation works in DllMain context
//     (the CRT init runs before our exported functions).
func TestPackBinary_FormatWindowsDLL_MSVC_E2E(t *testing.T) {
	in, err := os.ReadFile(filepath.Join("testdata", "testlib_msvc.dll"))
	if err != nil {
		t.Skipf("testlib_msvc.dll missing — build via testdata/build_testlib_msvc.cmd on a VM with VS Build Tools: %v", err)
	}
	packed, _, err := packer.PackBinary(in, packer.PackBinaryOptions{
		Format:       packer.FormatWindowsDLL,
		Stage1Rounds: 3,
		Seed:         42,
	})
	if err != nil {
		t.Fatalf("PackBinary FormatWindowsDLL: %v", err)
	}
	tmpDir := t.TempDir()
	dllPath := filepath.Join(tmpDir, "packed_msvc.dll")
	if err := os.WriteFile(dllPath, packed, 0o755); err != nil {
		t.Fatalf("write packed: %v", err)
	}

	h, err := syscall.LoadLibrary(dllPath)
	if err != nil {
		t.Fatalf("LoadLibrary packed MSVC DLL: %v", err)
	}
	defer syscall.FreeLibrary(h)

	pingAddr, err := syscall.GetProcAddress(h, "maldev_ping")
	if err != nil {
		t.Fatalf("GetProcAddress maldev_ping: %v", err)
	}
	ret, _, _ := syscall.SyscallN(pingAddr)
	const wantPing uint32 = 0xC0DEBABE
	if uint32(ret) != wantPing {
		t.Errorf("maldev_ping() = %#x, want %#x", uint32(ret), wantPing)
	}

	addAddr, err := syscall.GetProcAddress(h, "maldev_add")
	if err != nil {
		t.Fatalf("GetProcAddress maldev_add: %v", err)
	}
	ret, _, _ = syscall.SyscallN(addAddr, 3, 4)
	if uint32(ret) != 7 {
		t.Errorf("maldev_add(3, 4) = %d, want 7", uint32(ret))
	}
}

// TestPackBinary_FormatWindowsDLL_MSVC_Compress_E2E mirrors the above
// with Compress=true — proves Mode 7 LZ4 inflate works on a realistic
// MSVC binary, not just a synthetic mingw fixture.
func TestPackBinary_FormatWindowsDLL_MSVC_Compress_E2E(t *testing.T) {
	in, err := os.ReadFile(filepath.Join("testdata", "testlib_msvc.dll"))
	if err != nil {
		t.Skipf("testlib_msvc.dll missing: %v", err)
	}
	packed, _, err := packer.PackBinary(in, packer.PackBinaryOptions{
		Format:       packer.FormatWindowsDLL,
		Stage1Rounds: 3,
		Seed:         42,
		Compress:     true,
	})
	if err != nil {
		t.Fatalf("PackBinary FormatWindowsDLL+Compress: %v", err)
	}
	tmpDir := t.TempDir()
	dllPath := filepath.Join(tmpDir, "packed_msvc_compressed.dll")
	if err := os.WriteFile(dllPath, packed, 0o755); err != nil {
		t.Fatalf("write packed: %v", err)
	}
	h, err := syscall.LoadLibrary(dllPath)
	if err != nil {
		t.Fatalf("LoadLibrary compressed MSVC DLL: %v", err)
	}
	defer syscall.FreeLibrary(h)
	pingAddr, err := syscall.GetProcAddress(h, "maldev_ping")
	if err != nil {
		t.Fatalf("GetProcAddress maldev_ping: %v", err)
	}
	ret, _, _ := syscall.SyscallN(pingAddr)
	const want uint32 = 0xC0DEBABE
	if uint32(ret) != want {
		t.Errorf("maldev_ping after Compress decode = %#x, want %#x", uint32(ret), want)
	}
}
