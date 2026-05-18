//go:build windows && pe_noconsolation

package pe

import (
	"bytes"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// E2E suite for runtime/pe.RunExecutable. Only built with the
// `pe_noconsolation` tag — the embedded loader is required for
// the underlying BOF dispatch to do anything useful.
//
// # Why a DLL fixture
//
// Full Windows EXEs always reach ExitProcess at the end of
// main() — either directly or through the CRT _exit cleanup
// chain. No-Consolation tries to hook ExitProcess but in-
// process with the Go runtime sharing the host, the hook is
// unreliable: we observed reproducible host tear-down before
// captured output could be returned.
//
// The DLL path sidesteps it: hello_main is an exported
// function, runs to a normal return, the BOF gets control back,
// RunExecutable returns the captured stdout, the test sees it.
// EXE-path testing is documented in pe-loader.md as a known
// limitation; better exercised by VM-level tests with a
// sacrificial process, out of scope for the in-process suite.
//
// # Fixtures
//
//   - testdata/hello.x64.dll — minimal mingw-built DLL with one
//     exported function hello_main(const char *cmdline) that
//     prints the canary + echoes the cmdline. Source in
//     hello_dll.c. No-Consolation invokes the export with a
//     single arg (the operator-supplied cmdline), NOT
//     (argc, argv).

const (
	helloCanary = "HELLO_FROM_NOCONSOLATION_PE"
	helloExport = "hello_main"
)

// loadHelloDLL reads the vendored hello.x64.dll; t.Skips when
// missing instead of failing — keeps the suite resilient on
// fresh clones before testdata/hello_dll.c has been built.
func loadHelloDLL(t *testing.T) []byte {
	t.Helper()
	bytes, err := os.ReadFile("testdata/hello.x64.dll")
	if err != nil {
		t.Skipf("testdata/hello.x64.dll missing: %v "+
			"(rebuild via testdata/hello_dll.c header)", err)
	}
	if len(bytes) < 1024 {
		t.Fatalf("testdata/hello.x64.dll suspiciously small: %d bytes", len(bytes))
	}
	return bytes
}

// helloOptions returns an Options pre-wired to call
// hello_main, the test fixture's exported entry. Tests layer
// additional knobs via the returned struct.
func helloOptions() Options {
	return Options{Method: helloExport}
}

// Test-wide stdout capture state. msvcrt.dll's stdout handle is
// cached on first use by __iob_func — once printf has resolved
// stdout, subsequent SetStdHandle changes don't reach it. So we
// install ONE pipe at TestMain init, leave it in place for the
// whole test run, and drain it per-test via a mutex-guarded
// buffer that gets snapshot+reset around each RunExecutable.
var (
	captureMu       sync.Mutex
	captureBuf      bytes.Buffer
	capturePipeRead *os.File
	captureOrigOut  windows.Handle
	captureDrainErr error
)

// installStdoutCapture is called once at TestMain. It replaces
// STD_OUTPUT_HANDLE with a pipe write end and launches a reader
// goroutine that appends to captureBuf forever. Tests snapshot
// the buffer around each RunExecutable.
func installStdoutCapture() error {
	r, w, err := os.Pipe()
	if err != nil {
		return err
	}
	orig, err := windows.GetStdHandle(windows.STD_OUTPUT_HANDLE)
	if err != nil {
		_ = r.Close()
		_ = w.Close()
		return err
	}
	if err := windows.SetStdHandle(windows.STD_OUTPUT_HANDLE, windows.Handle(w.Fd())); err != nil {
		_ = r.Close()
		_ = w.Close()
		return err
	}
	capturePipeRead = r
	captureOrigOut = orig
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := r.Read(buf)
			if n > 0 {
				captureMu.Lock()
				captureBuf.Write(buf[:n])
				captureMu.Unlock()
			}
			if err != nil {
				captureDrainErr = err
				return
			}
		}
	}()
	return nil
}

// drainCapture returns and clears whatever the reader goroutine
// has appended since the last call.
func drainCapture() string {
	captureMu.Lock()
	defer captureMu.Unlock()
	s := captureBuf.String()
	captureBuf.Reset()
	return s
}

// runWithStdoutCapture wraps RunExecutable and returns (a) the
// BOF-captured stdout (b) the host stdout drained around the
// call. Either path is a valid "PE printed" witness.
func runWithStdoutCapture(peBytes []byte, opt Options) (string, string, error) {
	_ = drainCapture() // toss anything from a prior test
	out, runErr := RunExecutable(peBytes, opt)
	// msvcrt is asynchronous: small fflushed writes typically
	// arrive before RunExecutable returns, but Windows pipes can
	// hold them briefly. A short settle is enough — the BOF has
	// already finished work, so no PE code is racing us.
	time.Sleep(20 * time.Millisecond)
	return out, drainCapture(), runErr
}

// TestMain wires up the test-wide stdout capture before any
// test runs and tears it down afterwards. Required by the
// msvcrt-stdout-cache workaround documented on
// installStdoutCapture.
func TestMain(m *testing.M) {
	if err := installStdoutCapture(); err != nil {
		// Capture install failed — tests would mis-report. Bail.
		panic(err)
	}
	code := m.Run()
	_ = windows.SetStdHandle(windows.STD_OUTPUT_HANDLE, captureOrigOut)
	_ = capturePipeRead.Close()
	os.Exit(code)
}

// combinedHas reports whether either stream contains the substring.
// Centralises the "find canary in EITHER stream" assertion so the
// individual tests stay readable.
func combinedHas(boReturn, stdoutCap, needle string) bool {
	return strings.Contains(boReturn, needle) || strings.Contains(stdoutCap, needle)
}

// TestE2E_HelloSmoke is the witness: RunExecutable with vanilla
// Options{Method: hello_main} must produce the canary line somewhere
// — either in BeaconOutput-captured return, or in the host's
// own stdout (msvcrt direct-write path).
func TestE2E_HelloSmoke(t *testing.T) {
	out, stdoutCap, err := runWithStdoutCapture(loadHelloDLL(t), helloOptions())
	if err != nil {
		t.Fatalf("RunExecutable: %v", err)
	}
	if !combinedHas(out, stdoutCap, helloCanary) {
		t.Errorf("canary %q absent from both streams\nbofOut:\n%s\nstdout:\n%s",
			helloCanary, out, stdoutCap)
	}
}

// TestE2E_ArgvSingle proves the joinArgs → cmdline → CRT argv
// path round-trips a single token. The DLL re-echoes it as
// "ARGV[1]=<token>".
// TestE2E_CmdlineSingle proves the joinArgs → cmdline → DLL
// entry parameter path round-trips. hello_main echoes its
// cmdline as "CMDLINE=<full string>".
func TestE2E_CmdlineSingle(t *testing.T) {
	const marker = "MALDEV_E2E_MARKER"
	opt := helloOptions()
	opt.Args = []string{marker}
	out, stdoutCap, err := runWithStdoutCapture(loadHelloDLL(t), opt)
	if err != nil {
		t.Fatalf("RunExecutable: %v", err)
	}
	want := "CMDLINE=" + marker
	if !combinedHas(out, stdoutCap, want) {
		t.Errorf("cmdline echo %q absent\nbofOut:\n%s\nstdout:\n%s", want, out, stdoutCap)
	}
}

// TestE2E_ArgvMulti exercises multi-token argv including a
// quoted-token-with-spaces. syscall.EscapeArg quotes "two
// words" with surrounding quotes; CommandLineToArgvW must
// unwrap them so ARGV[2] lands as "two words" without the
// quotes leaking.
// TestE2E_CmdlineMulti verifies multi-token + quoted-token
// joining lands as a single cmdline string. EscapeArg quotes
// "two words" and the BOF passes the full quoted line through.
func TestE2E_CmdlineMulti(t *testing.T) {
	opt := helloOptions()
	opt.Args = []string{"first", "two words", "third"}
	out, stdoutCap, err := runWithStdoutCapture(loadHelloDLL(t), opt)
	if err != nil {
		t.Fatalf("RunExecutable: %v", err)
	}
	// EscapeArg yields `first "two words" third`; both the
	// quotes and tokens must appear in the printed echo.
	want := `CMDLINE=first "two words" third`
	if !combinedHas(out, stdoutCap, want) {
		t.Errorf("cmdline mismatch\nwant: %q\nbofOut:\n%s\nstdout:\n%s",
			want, out, stdoutCap)
	}
}

// TestE2E_NoOutput verifies Options.NoOutput suppresses stdout
// capture. The canary must not surface even though hello_main
// ran (visible side-effect testing would need a file-write
// fixture; for now we just assert the negative).
func TestE2E_NoOutput(t *testing.T) {
	opt := helloOptions()
	opt.NoOutput = true
	out, _, err := runWithStdoutCapture(loadHelloDLL(t), opt)
	if err != nil {
		t.Fatalf("RunExecutable: %v", err)
	}
	// NoOutput suppresses the BOF-captured stream; we don't assert
	// against the host's own stdout because No-Consolation's "no
	// redirect needed" path still lets msvcrt printf flow through
	// untouched — same observable behaviour as the operator would
	// see in a real implant (the BeaconOutput stream is what the
	// operator cares about; that one MUST be empty).
	if strings.Contains(out, helloCanary) {
		t.Errorf("NoOutput=true but canary in BOF output: %q", out)
	}
}

// TestE2E_Local exercises the from-disk path: write the DLL to
// a temp location, set Local=true + Path, pass nil bytes. The
// BOF must read the PE from the filesystem instead of the
// bofdata bytes block.
func TestE2E_Local(t *testing.T) {
	dllBytes := loadHelloDLL(t)
	tmp := t.TempDir() + `\hello.x64.dll`
	if err := os.WriteFile(tmp, dllBytes, 0o644); err != nil {
		t.Fatalf("write tmp: %v", err)
	}
	opt := helloOptions()
	opt.Local = true
	opt.Path = tmp
	out, stdoutCap, err := runWithStdoutCapture(nil, opt)
	if err != nil {
		t.Fatalf("RunExecutable: %v", err)
	}
	if !combinedHas(out, stdoutCap, helloCanary) {
		t.Errorf("from-disk run missing canary\nbofOut:\n%s\nstdout:\n%s",
			out, stdoutCap)
	}
}

// TestE2E_TimeoutGenerous verifies the Timeout knob is
// forwarded and a generous budget completes normally. This is
// the happy-path wire test, not a "kill long PE" assertion —
// that would require a sleeping fixture.
func TestE2E_TimeoutGenerous(t *testing.T) {
	opt := helloOptions()
	opt.Timeout = 5 * time.Second
	out, stdoutCap, err := runWithStdoutCapture(loadHelloDLL(t), opt)
	if err != nil {
		t.Fatalf("RunExecutable: %v", err)
	}
	if !combinedHas(out, stdoutCap, helloCanary) {
		t.Errorf("canary absent with 5s timeout\nbofOut:\n%s\nstdout:\n%s",
			out, stdoutCap)
	}
}

// TestE2E_HeadersOn verifies the Headers flag plumbs through —
// some real PEs need their headers preserved at load time
// (installers re-reading their own resource section). With a
// vanilla DLL this just smoke-tests that toggling the flag
// doesn't break the canonical path.
func TestE2E_HeadersOn(t *testing.T) {
	opt := helloOptions()
	opt.Headers = true
	out, stdoutCap, err := runWithStdoutCapture(loadHelloDLL(t), opt)
	if err != nil {
		t.Fatalf("RunExecutable: %v", err)
	}
	if !combinedHas(out, stdoutCap, helloCanary) {
		t.Errorf("Headers=true broke canonical path\nbofOut:\n%s\nstdout:\n%s",
			out, stdoutCap)
	}
}
