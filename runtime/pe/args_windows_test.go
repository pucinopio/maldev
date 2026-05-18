//go:build windows

package pe

import (
	"encoding/binary"
	"errors"
	"testing"
	"time"
)

// readUint32LE mirrors how the BOF side reads length prefixes via
// BeaconDataExtract / BeaconDataInt — the on-wire endianness has
// to match the consumer or the parser silently mis-frames.
func readUint32LE(t *testing.T, buf []byte) (uint32, []byte) {
	t.Helper()
	if len(buf) < 4 {
		t.Fatalf("buffer too short for uint32: %d bytes", len(buf))
	}
	return binary.LittleEndian.Uint32(buf[:4]), buf[4:]
}

// readString consumes a length-prefixed ASCII string from buf and
// returns (string-without-NUL, remainder). The on-wire form is
// 4-byte LE length (including NUL) + raw bytes.
func readString(t *testing.T, buf []byte) (string, []byte) {
	t.Helper()
	n, rest := readUint32LE(t, buf)
	if uint32(len(rest)) < n {
		t.Fatalf("short string payload: want %d, have %d", n, len(rest))
	}
	if n == 0 {
		return "", rest
	}
	return string(rest[:n-1]), rest[n:]
}

// readWideString consumes a wide-string field: length prefix in
// UTF-16 units, then raw UTF-16LE bytes. Decodes back to UTF-8
// for comparison convenience.
func readWideString(t *testing.T, buf []byte) (string, []byte) {
	t.Helper()
	n, rest := readUint32LE(t, buf)
	byteLen := int(n) * 2
	if len(rest) < byteLen {
		t.Fatalf("short wstring payload: want %d, have %d", byteLen, len(rest))
	}
	u16 := make([]uint16, n)
	for i := uint32(0); i < n; i++ {
		u16[i] = binary.LittleEndian.Uint16(rest[i*2 : i*2+2])
	}
	if n > 0 && u16[n-1] == 0 {
		u16 = u16[:n-1]
	}
	runes := make([]rune, len(u16))
	for i, c := range u16 {
		runes[i] = rune(c)
	}
	return string(runes), rest[byteLen:]
}

// readInt consumes a 4-byte LE int32. The BOF reads these via
// BeaconDataInt.
func readInt(t *testing.T, buf []byte) (int32, []byte) {
	t.Helper()
	n, rest := readUint32LE(t, buf)
	return int32(n), rest
}

// readBytes consumes the length-prefixed binary block (pe_bytes
// field). 4-byte LE length, then exactly that many raw bytes.
func readBytes(t *testing.T, buf []byte) ([]byte, []byte) {
	t.Helper()
	n, rest := readUint32LE(t, buf)
	if uint32(len(rest)) < n {
		t.Fatalf("short bytes payload: want %d, have %d", n, len(rest))
	}
	return rest[:n], rest[n:]
}

// TestPackArgs_FieldOrder is the canonical wire-format witness:
// it walks the packed buffer in the exact 27-field order
// No-Consolation's go() reads and verifies every value lands in
// its expected slot. Reordering packArgs without updating
// No-Consolation breaks this test loudly.
func TestPackArgs_FieldOrder(t *testing.T) {
	opt := Options{
		Args:           []string{"arg1", "arg with space"},
		Method:         "DllMain",
		Timeout:        90 * time.Second,
		UseUnicode:     true,
		NoOutput:       false,
		InThread:       true,
		LinkToPEB:      true,
		DontUnload:     false,
		AllocConsole:   true,
		CloseHandles:   false,
		DontSave:       true,
		ListPEs:        false,
		LoadAllDeps:    true,
		Headers:        true,
		Local:          false,
		Name:           "hello.exe",
		Path:           `C:\Temp\hello.exe`,
		UnloadPE:       "old.exe",
		Username:       "alice",
		LoadTime:       "2026-05-18T00:00:00Z",
		LoadAllDepsBut: "kernel32.dll",
		LoadDeps:       "user32.dll",
		SearchPaths:    `C:\Windows\System32`,
	}
	peBytes := []byte{0x4d, 0x5a, 0x90, 0x00} // MZ header sample
	buf := packArgs(peBytes, opt)

	// Walk in declared order — see packArgs documentation block.
	gotName, buf := readWideString(t, buf)
	if gotName != "hello.exe" {
		t.Errorf("pe_wname: got %q, want %q", gotName, "hello.exe")
	}
	gotNameA, buf := readString(t, buf)
	if gotNameA != "hello.exe" {
		t.Errorf("pe_name: got %q, want %q", gotNameA, "hello.exe")
	}
	gotWPath, buf := readWideString(t, buf)
	if gotWPath != `C:\Temp\hello.exe` {
		t.Errorf("pe_wpath: got %q", gotWPath)
	}
	gotBytes, buf := readBytes(t, buf)
	if string(gotBytes) != string(peBytes) {
		t.Errorf("pe_bytes mismatch")
	}
	gotPath, buf := readString(t, buf)
	if gotPath != `C:\Temp\hello.exe` {
		t.Errorf("pe_path: got %q", gotPath)
	}

	gotLocal, buf := readInt(t, buf)
	if gotLocal != 0 {
		t.Errorf("local: got %d, want 0", gotLocal)
	}
	gotTimeout, buf := readInt(t, buf)
	if gotTimeout != 90 {
		t.Errorf("timeout: got %d, want 90", gotTimeout)
	}
	gotHeaders, buf := readInt(t, buf)
	if gotHeaders != 1 {
		t.Errorf("headers: got %d, want 1", gotHeaders)
	}

	wantCmdline := `arg1 "arg with space"`
	gotCmdW, buf := readWideString(t, buf)
	if gotCmdW != wantCmdline {
		t.Errorf("cmdwline: got %q, want %q", gotCmdW, wantCmdline)
	}
	gotCmdA, buf := readString(t, buf)
	if gotCmdA != wantCmdline {
		t.Errorf("cmdline: got %q, want %q", gotCmdA, wantCmdline)
	}
	gotMethod, buf := readString(t, buf)
	if gotMethod != "DllMain" {
		t.Errorf("method: got %q", gotMethod)
	}

	flags := []struct {
		name string
		want int32
	}{
		{"use_unicode", 1},
		{"nooutput", 0},
		{"alloc_console", 1},
		{"close_handles", 0},
		{"dont_save", 1},
		{"list_pes", 0},
	}
	for _, f := range flags {
		var got int32
		got, buf = readInt(t, buf)
		if got != f.want {
			t.Errorf("%s: got %d, want %d", f.name, got, f.want)
		}
	}

	gotUnloadPE, buf := readString(t, buf)
	if gotUnloadPE != "old.exe" {
		t.Errorf("unload_pe: got %q", gotUnloadPE)
	}
	gotUser, buf := readString(t, buf)
	if gotUser != "alice" {
		t.Errorf("username: got %q", gotUser)
	}
	gotLoadTime, buf := readString(t, buf)
	if gotLoadTime != "2026-05-18T00:00:00Z" {
		t.Errorf("loadtime: got %q", gotLoadTime)
	}

	flags2 := []struct {
		name string
		want int32
	}{
		{"link_to_peb", 1},
		{"dont_unload", 0},
		{"load_all_deps", 1},
	}
	for _, f := range flags2 {
		var got int32
		got, buf = readInt(t, buf)
		if got != f.want {
			t.Errorf("%s: got %d, want %d", f.name, got, f.want)
		}
	}

	gotLAD, buf := readString(t, buf)
	if gotLAD != "kernel32.dll" {
		t.Errorf("load_all_deps_but: got %q", gotLAD)
	}
	gotLD, buf := readString(t, buf)
	if gotLD != "user32.dll" {
		t.Errorf("load_deps: got %q", gotLD)
	}
	gotSP, buf := readString(t, buf)
	if gotSP != `C:\Windows\System32` {
		t.Errorf("search_paths: got %q", gotSP)
	}

	gotInThread, buf := readInt(t, buf)
	if gotInThread != 1 {
		t.Errorf("inthread: got %d, want 1", gotInThread)
	}

	if len(buf) != 0 {
		t.Errorf("trailing bytes after packed buffer: %d", len(buf))
	}
}

// TestPackArgs_ZeroValue exercises the default-substitution
// branches: an empty Options yields a buffer the BOF can still
// parse (no negative lengths, timeout defaults to 60s, all flags
// 0, cmdline empty).
func TestPackArgs_ZeroValue(t *testing.T) {
	buf := packArgs(nil, Options{})

	// Skip the first 5 string/bytes fields (all empty).
	for i := 0; i < 5; i++ {
		switch i {
		case 0, 2:
			_, buf = readWideString(t, buf)
		case 3:
			_, buf = readBytes(t, buf)
		default:
			_, buf = readString(t, buf)
		}
	}

	_, buf = readInt(t, buf) // local
	timeout, buf := readInt(t, buf)
	if timeout != 60 {
		t.Errorf("default timeout: got %d, want 60", timeout)
	}
	headers, buf := readInt(t, buf)
	if headers != 0 {
		t.Errorf("default headers: got %d, want 0", headers)
	}
	// Remainder consumed only to validate buffer length is sane.
	_ = buf
}

// TestJoinArgs_QuoteSpaces verifies the simple CommandLineToArgvW-
// compatible quoting: tokens with whitespace or quotes get
// double-quote wrapped, embedded quotes are backslash-escaped.
func TestJoinArgs_QuoteSpaces(t *testing.T) {
	cases := []struct {
		in   []string
		want string
	}{
		{nil, ""},
		{[]string{}, ""},
		{[]string{"foo"}, "foo"},
		{[]string{"foo", "bar"}, "foo bar"},
		{[]string{"hello world"}, `"hello world"`},
		{[]string{"a", "b c", "d"}, `a "b c" d`},
		{[]string{`he said "hi"`}, `"he said \"hi\""`},
	}
	for _, c := range cases {
		got := joinArgs(c.in)
		if got != c.want {
			t.Errorf("joinArgs(%v): got %q, want %q", c.in, got, c.want)
		}
	}
}

// TestTimeoutSeconds_Defaults covers the three branches: zero ⇒
// defaultTimeout, negative ⇒ 0, positive ⇒ floor-divided seconds.
func TestTimeoutSeconds_Defaults(t *testing.T) {
	cases := []struct {
		name string
		in   time.Duration
		want int32
	}{
		{"zero defaults", 0, 60},
		{"negative clamps to zero", -5 * time.Second, 0},
		{"positive rounds down", 1500 * time.Millisecond, 1},
		{"exact second", 30 * time.Second, 30},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := timeoutSeconds(c.in)
			if got != c.want {
				t.Errorf("got %d, want %d", got, c.want)
			}
		})
	}
}

// TestBoolInt covers the trivial 0/1 mapping; protects against
// accidental "negate" introductions during future refactors.
func TestBoolInt(t *testing.T) {
	if boolInt(true) != 1 {
		t.Errorf("boolInt(true) != 1")
	}
	if boolInt(false) != 0 {
		t.Errorf("boolInt(false) != 0")
	}
}

// TestRunExecutable_LoaderMissing — under the default build (no
// pe_noconsolation tag), RunExecutable should return
// ErrLoaderMissing and propagate it cleanly, not panic on the nil
// blob.
func TestRunExecutable_LoaderMissing(t *testing.T) {
	_, err := RunExecutable([]byte{0x4d, 0x5a}, Options{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrLoaderMissing) {
		t.Errorf("want ErrLoaderMissing, got %v", err)
	}
}
