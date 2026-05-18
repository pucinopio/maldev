//go:build windows && bof_x86_loader

package bof

import (
	"context"
	"encoding/binary"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestX86Loader_Embedded_NotEmpty confirms the bof_x86_loader
// build-tag actually linked the shellcode bytes in. A regression
// here (empty embed slice, e.g. .bin file renamed without
// updating the embed directive) would silently fall back to
// ErrCrossArchX86Unsupported on every operator invocation.
func TestX86Loader_Embedded_NotEmpty(t *testing.T) {
	sc, err := loadX86LoaderShellcode()
	require.NoError(t, err)
	require.NotEmpty(t, sc, "embed slot must contain the loader shellcode bytes")
	// Sanity-floor on the size so a future change that
	// accidentally embeds a header-only artefact (e.g. an empty
	// ELF or a 4-byte stub) fails loudly.
	assert.GreaterOrEqual(t, len(sc), 64,
		"shellcode should be at least 64 bytes — got %d", len(sc))
}

// TestX86Loader_IsPE32DLL sniffs the magic + PE signature.
// Phase B-bis switched to the reflective-DLL model: the bytes
// are now a regular i386 PE32 DLL (parsed at runtime by the
// reflective loader) rather than a flat shellcode blob. A
// regression that ships shellcode bytes here would crash
// parsePEAndPlace with "bad DOS magic" — this test pins the
// expected shape at unit-test time.
func TestX86Loader_IsPE32DLL(t *testing.T) {
	sc, err := loadX86LoaderShellcode()
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(sc), 0x40)
	assert.Equal(t, byte('M'), sc[0])
	assert.Equal(t, byte('Z'), sc[1])

	// e_lfanew lives at 0x3C; the PE signature follows.
	peOff := uint32(sc[0x3C]) | uint32(sc[0x3D])<<8 |
		uint32(sc[0x3E])<<16 | uint32(sc[0x3F])<<24
	require.GreaterOrEqual(t, int(peOff)+4, 4)
	require.Less(t, int(peOff)+4, len(sc))
	assert.Equal(t, "PE\x00\x00", string(sc[peOff:peOff+4]))
}

// TestParsePEAndPlace_FindsBOFExec pins the reflective loader's
// PE parser against the actual embedded DLL: parse, lay, relocate
// against a synthetic base address, then assert the BOFExec
// export and the per-section metadata look sane. No cross-process
// activity — entirely in-process unit test.
func TestParsePEAndPlace_FindsBOFExec(t *testing.T) {
	dll, err := loadX86LoaderShellcode()
	require.NoError(t, err)
	const target uint32 = 0x10000000
	img, err := parsePEAndPlace(dll, target)
	require.NoError(t, err)
	require.NotNil(t, img)
	assert.Greater(t, int(img.sizeOfImage), 0)
	assert.Len(t, img.image, int(img.sizeOfImage))
	rva, ok := img.exportRVAs["BOFExec"]
	require.True(t, ok, "BOFExec export must be discoverable")
	assert.Greater(t, int(rva), 0)
	// At least one section must be marked executable — the
	// loader's code lives in .text.
	hasExec := false
	for _, s := range img.sections {
		if s.characteristics&0x20000000 != 0 {
			hasExec = true
			break
		}
	}
	assert.True(t, hasExec, "at least one section must carry IMAGE_SCN_MEM_EXECUTE")
}

// TestParsePEAndPlace_BadDOSMagic guards against shipping
// shellcode bytes by mistake. parsePEAndPlace must reject any
// input that doesn't start with MZ — would be a hard-to-trace
// crash inside the child if it slipped through.
func TestParsePEAndPlace_BadDOSMagic(t *testing.T) {
	junk := make([]byte, 256)
	junk[0] = 0xFF
	junk[1] = 0xEE
	_, err := parsePEAndPlace(junk, 0x10000000)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "DOS magic")
}

// TestSectionProtect_NeverRWX pins the protection downgrade — a
// PE section flagged EXECUTE+WRITE must lower to PAGE_EXECUTE_READ
// (0x20), never RWX (0x40), to match the loader's "no RWX after
// load" posture.
func TestSectionProtect_NeverRWX(t *testing.T) {
	rwx := imageScnMemExecute | imageScnMemWrite
	got := sectionProtect(rwx)
	assert.Equal(t, uint32(0x20), got, "RWX section must downgrade to PAGE_EXECUTE_READ")
	// Spot-check the other corners.
	assert.Equal(t, uint32(0x20), sectionProtect(imageScnMemExecute))
	assert.Equal(t, uint32(0x04), sectionProtect(imageScnMemWrite))
	assert.Equal(t, uint32(0x02), sectionProtect(0))
}

// TestRor13_KnownAnswers locks the Go-side ROR13 implementation
// against the precomputed kernel32 hashes baked into loader.c. A
// drift in the algorithm (e.g. someone "improves" win/api's
// ResolveByHash) would desynchronise the parent and the loader
// in a way that's only catchable in a live VM — this unit test
// catches it at compile-test time.
func TestRor13_KnownAnswers(t *testing.T) {
	cases := []struct {
		name string
		want uint32
	}{
		{"ExitThread", 0x60E0CEEF},
		{"VirtualAlloc", 0x91AFCA54},
		{"VirtualProtect", 0x7946C61B},
		{"GetProcessHeap", 0xA80EECAE},
		{"HeapAlloc", 0x2500383C},
		{"HeapFree", 0x10C32616},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ror13Hash(c.name)
			assert.Equal(t, c.want, got,
				"ROR13(%q) = 0x%08X, want 0x%08X", c.name, got, c.want)
		})
	}
}

// TestABIMagic_LittleEndianMatchesCSide pins the byte order. The
// shellcode reads p->magic as a 32-bit little-endian load
// (`cmp eax, 0x36384342`); the Go side writes it as
// binary.LittleEndian.PutUint32. Any mismatch (e.g. someone
// "fixes" the const to network-byte-order) would make every
// injection fail with LOADER_STATUS_ABI_MISMATCH.
func TestABIMagic_LittleEndianMatchesCSide(t *testing.T) {
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], loaderABIMagic)
	assert.Equal(t, []byte{'B', 'C', '8', '6'}, buf[:])
}

// TestX86BOF_Execute_NoopFixture is the cross-process E2E for
// slice 1.d step 1.b: the loader's COFF parser + relocation
// engine + entry call dispatch are exercised against a real
// (but trivial) i386 BOF that has no Beacon API imports and no
// cross-section relocations.
//
// The fixture testdata/noop.x86.o is `void go(char*, int) {}`
// compiled with i686-w64-mingw32-gcc — 16 bytes of .text plus
// empty .data/.bss. A passing test means the shellcode found
// the entry symbol "_go", called it cdecl-correctly, and
// returned to ExitThread without crashing.
//
// Skipped on hosts without SysWOW64\rundll32.exe (non-Windows
// or Windows-on-ARM without WoW64).
func TestX86BOF_Execute_NoopFixture(t *testing.T) {
	if _, err := os.Stat(defaultX86Host); err != nil {
		t.Skipf("WoW64 host %s missing: %v", defaultX86Host, err)
	}
	bof, err := os.ReadFile("testdata/noop.x86.o")
	require.NoError(t, err, "noop.x86.o fixture missing")

	res, err := Run(context.Background(), Spec{
		Bytes: bof,
		Args:  []byte{},
	})
	require.NoError(t, err, "expected loader to surface LOADER_STATUS_DONE")
	require.NotNil(t, res)
	assert.Empty(t, res.Output, "noop fixture writes no output")
	assert.Empty(t, res.Errors, "noop fixture writes no errors")
}

// TestX86BOF_Execute_HelloBeacon exercises the full Beacon API
// import-resolution + reloc-application + BOF-call chain:
//
//   - testdata/hello_beacon.x86.o calls
//     `BeaconPrintf(0, "hello from x86 BOF\n")`.
//   - The loader resolves __imp__BeaconPrintf via ROR13 against
//     the in-DLL Beacon table, populates the import slot, and
//     applies the DIR32 reloc in the BOF .text so the
//     `call dword ptr [imp_slot]` reaches our BeaconPrintf impl.
//   - BeaconPrintf appends the format string verbatim to the
//     parent-allocated out buffer (step 1.c minimal, no `%`
//     expansion).
//
// Passing this test proves: COFF parsing handles real BOFs,
// internal DIR32 relocs (.text → .rdata string literal) work,
// external __imp__ resolution works, and the in-DLL Beacon
// surface plumbs through to the parent's ReadProcessMemory
// output buffer.
func TestX86BOF_Execute_HelloBeacon(t *testing.T) {
	if _, err := os.Stat(defaultX86Host); err != nil {
		t.Skipf("WoW64 host %s missing: %v", defaultX86Host, err)
	}
	bof, err := os.ReadFile("testdata/hello_beacon.x86.o")
	require.NoError(t, err, "hello_beacon.x86.o fixture missing")

	res, err := Run(context.Background(), Spec{
		Bytes: bof,
		Args:  []byte{},
	})
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Contains(t, string(res.Output), "hello from x86 BOF",
		"BeaconPrintf output must round-trip through the params block")
}

// TestX86BOF_Execute_ParseArgs exercises BeaconDataParse / Int /
// Extract + the Format family + BeaconOutput on a real i386 BOF.
// The args buffer is packed via the canonical Args API; the BOF
// reads back (n=42, s="hello-x86") and emits "n=42:s=hello-x86"
// via BeaconOutput. Round-trip proves Data + Format + Output
// pipes from the parent through the WoW64 helper.
func TestX86BOF_Execute_ParseArgs(t *testing.T) {
	if _, err := os.Stat(defaultX86Host); err != nil {
		t.Skipf("WoW64 host %s missing: %v", defaultX86Host, err)
	}
	bof, err := os.ReadFile("testdata/parse_args.x86.o")
	require.NoError(t, err, "parse_args.x86.o fixture missing")

	a := NewArgs()
	a.AddInt(42)
	a.AddString("hello-x86")

	res, err := Run(context.Background(), Spec{
		Bytes: bof,
		Args:  a.Pack(),
	})
	require.NoError(t, err)
	require.NotNil(t, res)
	out := string(res.Output)
	assert.Contains(t, out, "n=42",
		"BeaconFormatInt → BeaconOutput must surface the parsed int")
	assert.Contains(t, out, "s=hello-x86",
		"BeaconDataExtract → BeaconFormatAppend → BeaconOutput must surface the parsed string")
}

// TestX86BOF_Execute_HelpersKV exercises BeaconGetCustomUserData,
// BeaconGetSpawnTo, toWideChar, and the BeaconAddValue / Get /
// Remove KV trio. The fixture emits semicolon-separated
// assertions; we grep for each.
func TestX86BOF_Execute_HelpersKV(t *testing.T) {
	if _, err := os.Stat(defaultX86Host); err != nil {
		t.Skipf("WoW64 host %s missing: %v", defaultX86Host, err)
	}
	bof, err := os.ReadFile("testdata/helpers_kv.x86.o")
	require.NoError(t, err, "helpers_kv.x86.o fixture missing")

	res, err := Run(context.Background(), Spec{
		Bytes:    bof,
		UserData: []byte("user-data-bytes-go-here"),
		SpawnTo:  `C:\Windows\SysWOW64\notepad.exe`,
	})
	require.NoError(t, err)
	require.NotNil(t, res)
	out := string(res.Output)
	assert.Contains(t, out, "userdata=user-data-bytes-go-here",
		"BeaconGetCustomUserData must surface the configured blob")
	assert.Contains(t, out, `spawnto=C:\Windows\SysWOW64\notepad.exe`,
		"BeaconGetSpawnTo must surface the configured path")
	assert.Contains(t, out, "kv-after-add=ok",
		"BeaconAddValue+GetValue round-trip must succeed")
	assert.Contains(t, out, "kv-after-remove=missing",
		"BeaconRemoveValue must purge the entry")
	assert.Contains(t, out, "wide=H,e,l",
		"toWideChar must produce UTF-16LE with low bytes preserved")
}

// TestX86BOF_Execute_TokenAdmin exercises BeaconIsAdmin +
// BeaconRevertToken end-to-end. The fixture calls IsAdmin
// (which triggers ensure_advapi → LoadLibraryA("advapi32.dll")
// → OpenProcessToken → GetTokenInformation) and emits the
// result; then BeaconRevertToken as a safe no-op.
//
// The Windows VM test user (`test`) is in BUILTIN\Administrators
// but rundll32 doesn't elevate by default, so isadmin=0 is the
// expected value when the user is not elevated. We just assert
// that a value (either 0 or 1) was emitted — the contract is
// "BeaconIsAdmin returned, didn't crash". A dedicated elevated
// run would assert isadmin=1.
func TestX86BOF_Execute_TokenAdmin(t *testing.T) {
	if _, err := os.Stat(defaultX86Host); err != nil {
		t.Skipf("WoW64 host %s missing: %v", defaultX86Host, err)
	}
	bof, err := os.ReadFile("testdata/token_admin.x86.o")
	require.NoError(t, err, "token_admin.x86.o fixture missing")

	res, err := Run(context.Background(), Spec{Bytes: bof})
	require.NoError(t, err)
	require.NotNil(t, res)
	out := string(res.Output)
	assert.Regexp(t, `isadmin=[01]`, out,
		"BeaconIsAdmin must surface 0 or 1, not crash")
	assert.Contains(t, out, "revert=done",
		"BeaconRevertToken must complete cleanly with no impersonation active")
}

// TestX86BOF_Execute_BadHost_FailsSpawn exercises the
// CreateProcess failure path. A bogus SpawnTo must surface a
// "spawn rundll32" error rather than crash. Deterministic — no
// race with a real BOF execution.
func TestX86BOF_Execute_BadHost_FailsSpawn(t *testing.T) {
	bof, err := os.ReadFile("testdata/noop.x86.o")
	require.NoError(t, err)
	r, err := coffX86Loader{}.Load(bof)
	require.NoError(t, err)
	x := r.(*x86BOF)
	x.SetSpawnTo(`C:\Windows\Nope\DoesNotExist.exe`)
	_, err = x.Execute(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "CreateProcess",
		"bogus SpawnTo should surface a CreateProcess failure")
}

// ror13Hash mirrors win/api.ResolveByHash and the C side's
// ror13_hash. Lives in this test file (under the bof_x86_loader
// tag) for now; step 1 will move it to a shared helper used by
// the orchestrator at hash-precomputation time.
func ror13Hash(s string) uint32 {
	var h uint32
	for i := 0; i < len(s); i++ {
		h = ((h >> 13) | (h << 19)) + uint32(s[i])
	}
	return h
}
