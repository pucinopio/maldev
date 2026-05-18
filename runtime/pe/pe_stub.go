//go:build !windows

package pe

import (
	"errors"
	"time"
)

// errUnsupported is returned by every exported entry point on
// non-Windows builds. The package compiles on Linux/macOS so the
// implant code that imports it (typically guarded itself by
// runtime.GOOS == "windows") still type-checks.
var errUnsupported = errors.New("runtime/pe: not supported on this platform")

// Options mirrors the Windows surface so cross-platform code
// can build the struct unconditionally. Fields documented in
// options_windows.go.
type Options struct {
	Args            []string
	Method          string
	Timeout         time.Duration
	UseUnicode      bool
	NoOutput        bool
	InThread        bool
	LinkToPEB       bool
	DontUnload      bool
	AllocConsole    bool
	CloseHandles    bool
	UnloadLibs      string
	DontSave        bool
	ListPEs         bool
	LoadAllDeps     bool
	Headers         bool
	Local           bool
	Name            string
	Path            string
	UnloadPE        string
	Username        string
	LoadTime        string
	LoadAllDepsBut  string
	LoadDeps        string
	SearchPaths     string
}

// RunExecutable returns errUnsupported on non-Windows.
func RunExecutable(_ []byte, _ Options) (string, error) {
	return "", errUnsupported
}
