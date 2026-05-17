# Defender catch on dropper

## When you see this

You drop the packed binary on the target host and Windows Defender
quarantines it within seconds, before any code runs. You see one of:

- `Trojan:Win32/Wacatac.B!ml` (Defender ML signature — generic).
- `Trojan:Win32/Meterpreter` (your shellcode payload was sigged).
- The file vanishes between `scp` finishing and your trigger.
- `Get-MpThreatDetection` on the target lists the binary.

## Most likely causes (ranked)

1. **String-based signature on the Go runtime** (≈40%) — Go binaries
   leak ~20 KB of pclntab + import strings even after packing if
   you didn't run `garble` / strip first.
2. **The packer's stub itself is now sigged** (≈25%) — the SGN
   decoder body is small and has been seen in the wild long enough
   for ML models to learn it. Default stub is the *baseline* signal.
3. **Authenticode / cert mismatch** (≈15%) — your packed binary
   claims a SECURITY directory pointing into garbage. Defender
   treats "signed-but-tampered" as a strong tell.
4. **Sandbox detonation** (≈10%) — if Defender's cloud submitted
   the file, it might have run and observed the payload doing
   suspicious things. Mostly preventable by anti-sandbox checks.
5. **Network metadata** (≈5%) — your C2 URL or shellcode hash is
   already in MS threat intel.
6. **Other** (≈5%) — IOC the team carries from a previous engagement.

## Diagnostic steps

1. **Strip the Go-ness first.** Run `strings packed.exe | grep -ic go`.
   - *pass* (< 50 hits): not the cause, go to step 2.
   - *fail* (≥ 50): rebuild with `garble build` (see
     [opsec-build.md](../../opsec-build.md)) and re-pack. Retry.
2. **Test on a Defender-only VM with cloud-protection off.** Drop
   the file. Wait 30 s.
   - *pass* (not flagged): cause is cloud submission / behavioural,
     not on-disk signature. Go to step 4.
   - *fail* (still flagged): it's an on-disk static signature.
     Go to step 3.
3. **Mutate the stub.** Re-pack with
   `-randomize-stub-section-name` (already default since v0.135.0
   — `KeepDefaultStubSectionName: false`). Also try `-compress`
   to change the section-size signature.
   - *pass*: signature was on section names or size. Done.
   - *fail*: signature is on the stub body itself. Go to step 5.
4. **Disable behavioural triggers in the payload.** Comment out
   the AMSI/ETW patches, re-pack, re-drop.
   - *pass* (not flagged): cloud submitted and saw evasion calls.
     Hide them behind sleep or unhook first
     ([sleep-mask](../../techniques/evasion/sleep-mask.md)).
   - *fail*: it's not behavioural either.
5. **Mutate the SGN rounds.** Default is 3 rounds. Try
   `-rounds 5` or `-rounds 1` to see if the signature is keyed
   on the iteration count.
6. **Last resort: cert preservation.** Set
   `PreserveAuthenticodeDirectory: true` — sometimes Defender
   weights "no cert" against the binary. (Counter-intuitive but
   measurable.) See [pe/cert](../../techniques/pe/certificate-theft.md).

## Mitigation

Ordered cheapest first:

1. `garble build` + `strip Go pclntab` ([opsec-build](../../opsec-build.md)).
2. `RandomizeAll: true` in `PackBinaryOptions`.
3. Donor-cert from a legitimate signed binary
   ([masquerade](../../techniques/pe/masquerade.md)).
4. Switch to Mode 8 (EXE→DLL) + `rundll32` invocation — different
   load path, different signatures.
5. Compose `unhook.CommonClassic` BEFORE any AMSI/ETW patches —
   defeats Defender's `amsi.dll` hook scanner.

## Prevention

- Run `clamscan` and `Defender on a clean VM` BEFORE field-deploy.
  See [opsec-build.md](../../opsec-build.md) Quick Start.
- Use `preset.Aggressive` for evasion stacking
  ([preset](../../techniques/evasion/preset.md)).
- Rotate seeds (`Seed: <random>`) per engagement to keep
  hash-based signatures from matching.

## Related

- Cookbook: [UPX-style packer + cover](../upx-style-packer.md).
- Technique: [packer.md](../../techniques/pe/packer.md) § Defender bypass section.
- Runbook: [DLL hijack succeeded but silent](dll-hijack-silent.md).
