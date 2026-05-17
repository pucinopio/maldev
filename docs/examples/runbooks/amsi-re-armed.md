# AMSI re-armed mid-flight

## When you see this

You called `amsi.PatchAll(caller)` at process start (it returned
`nil`), but a later `Assembly.Load` / PowerShell `IEX` still
triggers Defender on a payload that should now bypass AMSI. The
patch silently became ineffective.

## Most likely causes (ranked)

1. **`amsi.dll` was re-mapped or reloaded after the patch** (≈50%) —
   `LoadLibrary("amsi.dll")` post-patch can map a fresh copy if
   the original handle was closed; the new copy has unpatched
   bytes.
2. **A new process inherited from the patched one** (≈25%) —
   AMSI patches are *per-process*, not per-token. Spawning
   PowerShell via `CreateProcess` gets a clean `amsi.dll`.
3. **Defender pushed a `WerFault.exe`-style recovery** (≈10%) —
   modern Defender can re-protect AMSI in monitored processes
   via a kernel callback writing back the original bytes.
4. **Wrong process was patched** (≈10%) — your evasion stack ran
   in the launcher, but the `clr.LoadAndExecute` spawned a child
   that did the loading.
5. **AMSI provider chain has more than Defender** (≈5%) — third-
   party AV registered its own COM provider, untouched by your
   `AmsiScanBuffer` patch.

## Diagnostic steps

1. **Check the patched bytes are still in place.** Read the first
   3 bytes of `AmsiScanBuffer` and compare to `31 C0 C3`.
   ```go
   addr, _ := windows.LoadLibrary("amsi.dll")
   proc, _ := windows.GetProcAddress(addr, "AmsiScanBuffer")
   var bytes [3]byte
   // ReadProcessMemory on own PID via wsyscall.Caller
   if bytes != [3]byte{0x31, 0xC0, 0xC3} {
       // re-arm detected
   }
   ```
   - *bytes match*: AMSI patch is intact; cause is elsewhere
     (probably step 4). Continue to step 4.
   - *bytes differ*: AMSI was re-armed. Step 2.
2. **Check `amsi.dll` base address.** Has it changed since the
   original patch? Capture base at patch time, compare now.
   - *same base*: the bytes were rewritten in place (Defender
     kernel-callback or anti-tamper). Mitigation: see step 6.
   - *different base*: a fresh map happened. Patch again now and
     consider lazy re-patching on every COM call.
3. **Check for additional providers.** Enumerate
   `HKLM\SOFTWARE\Microsoft\AMSI\Providers\*`. Each subkey is a
   CLSID of another provider DLL.
   - if there are non-Defender providers: those are bypassing
     `amsi.dll` entirely. Step 5.
4. **Capture the loader child PID.** When `clr.LoadAndExecute`
   spawns a sub-process, get its PID and check Defender for
   detection events on *that* PID, not yours.
   - *match*: confirmed cause #2. See mitigation #2.
5. **Manual patching of provider DLLs** (last resort) — same
   prologue trick on the registered provider's `Scan` function.
   See `evasion/amsi` godoc.

## Mitigation

1. **Re-patch periodically.** Wrap suspicious calls with
   `amsi.PatchAll(caller)` immediately before them. Patches are
   idempotent (ADR-0002).
2. **Patch every child you spawn.** If your launcher chains to
   PowerShell, inject `evasion/amsi.PatchAll` before the script
   runs (PowerShell host process).
3. **Compose `unhook` first** (see
   [ntdll-unhooking](../../techniques/evasion/ntdll-unhooking.md)).
   Defender's behavioural counters depend on ntdll hooks; remove
   them and you reduce the chance of re-arm.
4. **Use `preset.Aggressive`** which adds ACG + BlockDLLs to
   prevent later `amsi.dll` reloads.

## Prevention

- Always `defer` a re-patch after any `LoadLibrary` you make.
- Verify the patch is alive before any high-value AMSI call (the
  cost is 3 bytes of read, negligible).
- Avoid spawning child processes for sensitive operations —
  prefer in-process injection or reflective load.

## Related

- Technique: [evasion/amsi](../../techniques/evasion/amsi-bypass.md).
- Technique: [evasion/preset](../../techniques/evasion/preset.md).
- Runbook: [Defender catch on dropper](defender-catch.md).
