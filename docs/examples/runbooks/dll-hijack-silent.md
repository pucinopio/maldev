# DLL hijack succeeded but silent

## When you see this

You planted `hijackme.dll` in the writable target directory, the
SYSTEM-context process (scheduled task / service) loaded it
successfully (you see the `LoadLibrary` in Procmon / ETW), but
your payload **never wrote its marker file** or never connected
back to C2. No crash, no Defender event, just silence.

## Most likely causes (ranked)

1. **DllMain returned TRUE but the spawned OEP thread crashed
   silently** (≈45%) — most common. The Go runtime aborts on init
   failure without exception bubbling to the host process.
2. **Marker write path requires permissions the SYSTEM context
   doesn't have** (≈20%) — yes, SYSTEM can be denied (Mandatory
   Integrity Level mismatch on certain HKCU paths, or AppLocker
   block on the marker directory).
3. **DLL host (victim.exe) exits before the OEP thread flushes
   the marker** (≈15%) — race we hit during 1.B.2 development;
   victim sleeps 5 s post-LoadLibrary to give the thread time.
4. **PEB.CommandLine patch corrupted host loader state** (≈10%) —
   the symptom was the canonical 0x000C000B bug fixed in the
   label-collision ADR. Should be a non-issue post-v0.135.0.
5. **Defender silently terminated the spawned thread** (≈10%) —
   no quarantine event, no logged catch, just thread death.

## Diagnostic steps

1. **Add a stdout breadcrumb at the very first line of the OEP.**
   Open `examples/privesc-dll-hijack/probe/main.go`, add
   `fmt.Println("OEP reached")` as line 2 of `main()`. Re-pack.
   Re-drop. Watch the victim's stdout (`schtasks /Query /v` →
   note the action stdout path).
   - *pass* (you see "OEP reached"): step 2.
   - *fail*: thread didn't reach OEP — cause is the spawn block
     itself. Check the [RunWithArgs export godoc](https://pkg.go.dev/github.com/oioio-space/maldev/pe/packer)
     for the ERROR_INVALID_HANDLE diagnostic chain.
2. **Add a write to a writable-by-everyone path.** Replace the
   marker write with one to `C:\Users\Public\maldev-debug.txt`.
   - *pass* (file appears): your marker dir wasn't writable.
     Re-plant via `icacls C:\ProgramData\maldev-marker /grant Everyone:F`.
   - *fail* (no file): host process is dying before flush.
     Step 3.
3. **Force the host to stay alive.** Insert
   `time.Sleep(30 * time.Second)` in `examples/privesc-dll-hijack/victim/victim.c`
   right after `LoadLibrary`. Recompile victim, re-deploy.
   - *pass* (marker shows up): race condition. Keep the sleep in
     prod or use the WaitForSingleObject pattern in RunWithArgs
     (see ADR-0001 caller pattern).
   - *fail*: thread is being killed externally. Step 4.
4. **Check Defender's behavioural log.** On the target:
   `Get-MpThreatDetection | Select Resources, InitialDetectionTime`
   and `Get-WinEvent -LogName Microsoft-Windows-Windows
   Defender/Operational -MaxEvents 50`.
   - *pass* (defender entry): you're being caught by behavioural
     analysis. Follow [defender-catch](defender-catch.md).
   - *fail* (nothing): step 5.
5. **Attach ProcMon (`procmon.exe`) with filter on victim's PID.**
   Look for `Thread Exit` with non-zero exit code.
   - non-zero exit: Go init failure. Compile probe with
     `GOOS=windows go build -gcflags="all=-N -l"` for unstripped
     stack traces.
   - clean exit: marker dir was unwritable even though step 2
     said writable. Re-check ACL.

## Mitigation

Ordered cheapest first:

1. Verify the marker dir ACL grants Modify rights to the SYSTEM
   context (NOT just Read).
2. Add 5-second victim sleep post-LoadLibrary (it's already the
   `examples/privesc-dll-hijack/victim/victim.c` default since
   slice 9.8.a).
3. Pre-flush the probe's stdout buffer: `fmt.Println` then
   `os.Stdout.Sync()`.
4. Use [Caller=MethodIndirect](../../techniques/syscalls/direct-indirect.md)
   to dodge Defender's `kernel32.LoadLibrary` hook.

## Prevention

- Always add an "I started" breadcrumb at the OEP that writes to a
  path *outside* your final marker dir. Two write sites = two
  diagnostic anchors.
- Use the `examples/privesc-dll-hijack/` chain as a golden
  reference; deviate one thing at a time.

## Related

- Cookbook: [Full chain](../full-chain.md).
- Technique: [`recon/dllhijack`](../../techniques/recon/dll-hijack.md).
- ADR: [0001 — wsyscall.Caller pattern](../../concepts/decisions/0001-wsyscall-caller-pattern.md).
