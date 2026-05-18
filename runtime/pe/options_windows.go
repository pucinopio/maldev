//go:build windows

package pe

import "time"

// Options drives RunExecutable. All fields optional; zero values
// produce a sane "run this EXE with empty cmdline, capture stdout"
// invocation that matches No-Consolation's CLI defaults
// (--timeout 60s, output captured, headers stripped, in-thread
// off, link-to-PEB off, dont-unload off).
//
// Field ordering matches the No-Consolation BeaconData wire
// contract (source/entry.c go() function). Renaming or reordering
// breaks the packer in args_windows.go — keep them in lockstep.
type Options struct {
	// Args is the EXE command line, split into tokens. Joined
	// space-separated into the cmdline / cmdwline fields the BOF
	// reads. Set Args to nil for a bare invocation (argv = {exe}).
	Args []string

	// Method overrides the DLL export to call when the loaded PE
	// is a DLL. Default "DllMain". Ignored for EXEs.
	Method string

	// Timeout caps execution. Zero ⇒ 60 seconds (No-Consolation
	// default). Sub-second values round to the nearest second on
	// the wire — the BOF reads an int.
	Timeout time.Duration

	// UseUnicode picks the UTF-16 cmdline path inside the BOF.
	// When true, cmdwline is honoured and cmdline is ignored.
	UseUnicode bool

	// NoOutput suppresses stdout/stderr capture. RunExecutable
	// still returns "" (no error). Use for fire-and-forget PEs.
	NoOutput bool

	// InThread runs the PE on the BOF's calling thread. Blocks
	// until completion; required for PEs that mutate TLS or
	// expect a stable thread identity.
	InThread bool

	// LinkToPEB chains the loaded image into the PEB.Ldr list so
	// EnumProcessModules / GetModuleHandle finds it. Visible to
	// any tool that walks the loader chain — opsec hot.
	LinkToPEB bool

	// DontUnload keeps the PE mapped after execution. Useful for
	// multi-call DLLs; leaks RWX pages otherwise.
	DontUnload bool

	// AllocConsole calls AllocConsole() before the PE entry so
	// console-mode PEs have a stdout to write to. Spawns a
	// visible console window — opsec hot.
	AllocConsole bool

	// CloseHandles forces handle cleanup on PE exit even when
	// the PE itself fails to call ExitProcess.
	CloseHandles bool

	// UnloadLibs is a comma-separated list of DLL names the BOF
	// should unload after the PE returns. Useful when a transient
	// dependency was LoadLibrary'd into the host and should not
	// linger. Empty ⇒ no extra unloads.
	UnloadLibs string

	// DontSave skips the No-Consolation internal cache (the
	// "PE registry" used by ListPEs).
	DontSave bool

	// ListPEs flips the BOF into list-mode: instead of running
	// a PE it prints the currently-loaded PE table to output.
	// peBytes is ignored in this mode.
	ListPEs bool

	// LoadAllDeps preloads every dependent DLL listed in the
	// PE's import table before calling the entry point.
	LoadAllDeps bool

	// Headers keeps the PE headers mapped in the final image.
	// Required by PEs that read their own headers at runtime
	// (most installers); breaks anti-analysis tooling that
	// strips headers post-load.
	Headers bool

	// Local toggles loading the PE from a path on the target
	// instead of from the bytes pass-through. When true,
	// peBytes is ignored and Path / Name identify the on-disk
	// file. Default false (in-memory).
	Local bool

	// Name overrides the displayed module name. Defaults to
	// "noconsolation.exe" inside the BOF when empty.
	Name string

	// Path is the resolved disk path passed to the BOF — only
	// meaningful when Local is true.
	Path string

	// UnloadPE names a previously-loaded PE to unload before the
	// new one is run. Empty ⇒ no unload step.
	UnloadPE string

	// Username sets the impersonation hint reported back to the
	// PE via GetUserName-style introspection. Cosmetic, does
	// not change actual token state.
	Username string

	// LoadTime is a stamp string the BOF stores alongside the PE
	// cache entry. Free-form.
	LoadTime string

	// LoadAllDepsBut is a comma-separated list of DLL names
	// excluded from LoadAllDeps preloading.
	LoadAllDepsBut string

	// LoadDeps is the inverse of LoadAllDepsBut: a comma-
	// separated explicit allow-list of dependencies to preload.
	LoadDeps string

	// SearchPaths is a comma-separated list of directories the
	// BOF appends to the DLL search order before LoadLibrary.
	SearchPaths string
}

// defaultTimeout is what RunExecutable substitutes when
// Options.Timeout is zero. Matches No-Consolation's CLI default.
const defaultTimeout = 60 * time.Second
