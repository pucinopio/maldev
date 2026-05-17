# Glossary

> Terms maldev uses without redefinition. If a page references
> jargon and you're not sure, the term should be here. Open an
> issue if a term is missing.
>
> Entries are alphabetical. Each entry is one sentence (definition)
> plus one optional sentence (where it shows up in maldev).

## A

**ADR (Architecture Decision Record).** A short record of why a
non-trivial decision was made. See [Concepts ▸ Decisions](concepts/decisions/README.md).

**AMSI (Antimalware Scan Interface).** Windows mechanism that
ships script bodies (PowerShell, .NET, …) to a registered
antimalware provider for inspection. Bypass via
[`evasion/amsi`](techniques/evasion/amsi-bypass.md).

**ACG (Arbitrary Code Guard).** Windows mitigation that blocks a
process from allocating dynamic executable memory. Activated via
`SetProcessMitigationPolicy`; relevant in `preset.Aggressive`.

**ApiSet.** Indirection layer (`api-ms-win-*.dll`) that resolves
to real DLLs at load time. Important when walking exports;
ApiSet contracts get filtered in DLL-hijack discovery.

## B

**BOF (Beacon Object File).** Cobalt-Strike-style relocatable
COFF object loaded in-process. Run via
[`cmd/bof-runner`](tools/bof-runner.md) or
[`runtime/bof-loader`](techniques/runtime/bof-loader.md).

**BYOVD (Bring Your Own Vulnerable Driver).** Use a legitimately-
signed but exploitable driver to gain kernel R/W. See
[`kernel/byovd`](techniques/kernel/byovd-rtcore64.md).

**BlockDLLs (`Mitigation::BinarySignaturePolicy::MicrosoftSignedOnly`).**
Process mitigation that allows only Microsoft-signed DLLs to
load. Part of `preset.Aggressive`.

## C

**Caller (`*wsyscall.Caller`).** The runtime knob that selects
how every NT* call is issued (WinAPI / Native / Direct / Indirect).
See [ADR-0001](concepts/decisions/0001-wsyscall-caller-pattern.md).

**CET (Control-flow Enforcement Technology).** Hardware-assisted
mitigation (shadow stack + indirect-branch tracking). maldev's
`evasion/cet` covers opt-out at the implant-process level.

**CFG (Control Flow Guard).** Software mitigation that validates
indirect-call targets against a bitmap. AMSI bypass works around
it because prologue patching doesn't trigger CFG checks.

**COFF (Common Object File Format).** Object-file format used by
BOFs. Different from PE; PE wraps COFF + extras.

## D

**D3FEND.** MITRE's defender taxonomy (counter-techniques to
ATT&CK). Tagged on every technique page as `D3-XXX`.

**Diátaxis.** Doc-IA framework with 4 quadrants (Tutorial /
How-to / Reference / Explanation). maldev's nav is Diátaxis-
inspired (see [ADR-0004](concepts/decisions/0004-diataxis-pragmatic.md)).

**DLL hijack.** Plant a DLL on the search path of a privileged
process. Discovery via [`recon/dllhijack`](techniques/recon/dll-hijack.md);
full chain in [`examples/privesc-dll-hijack`](https://github.com/oioio-space/maldev/tree/master/examples/privesc-dll-hijack).

**Donor cert.** A `WIN_CERTIFICATE` blob harvested from a
legitimately-signed binary, stamped into a packed payload to
mimic provenance.

## E

**EDR (Endpoint Detection and Response).** Defender + product (CrowdStrike,
SentinelOne, Defender for Endpoint, …). Hooks `ntdll`, watches ETW,
inspects memory.

**EAT (Export Address Table).** The table in a PE's
`IMAGE_DIRECTORY_ENTRY_EXPORT` listing exported functions. Walked
by hash-resolution shellcode.

**ETW (Event Tracing for Windows).** Kernel + user telemetry
backbone. AMSI counterpart for behavioural data. Patched via
[`evasion/etw`](techniques/evasion/etw-patching.md).

**ETW-TI (Threat Intelligence).** Privileged ETW provider that
exposes the loudest signals (RWX allocations, suspicious
loaders).

## F

**FILE_SHARE_READ.** `CreateFile` share flag. Allows the file to
be opened for read by others; relevant for marker-file flushing
races (see [Runbook: DLL hijack silent](examples/runbooks/dll-hijack-silent.md)).

## G

**garble.** Go toolchain wrapper that obfuscates names + literals
+ build IDs. Used by `make release`; see
[OPSEC build pipeline](opsec-build.md).

**godoc / pkg.go.dev.** The authoritative API reference for any Go
package. maldev's policy: every technique page links here, never
duplicates content. See
[ADR-0002](concepts/decisions/0002-godoc-only-api-ref.md).

## H

**Hook.** Code overlay on a Windows API entry (typically by an
EDR) that intercepts calls. Unhooking removes the overlay; see
[`evasion/unhook`](techniques/evasion/ntdll-unhooking.md).

**HALO's gate / Tartarus' gate / Hell's gate.** Three flavours of
direct-syscall SSN resolution. maldev's `wsyscall.MethodDirect`
implements the modern variant.

## I

**IAT (Import Address Table).** Where the loader writes the
absolute VAs of imported functions. Hooked by some EDRs as a
cheap interception point.

**IL (Integrity Level).** Windows process integrity (Low /
Medium / High / SYSTEM). UAC bypass moves from Medium to High.

**IOC (Indicator of Compromise).** Anything (hash, IP, file path,
registry key) that telegraphs your presence.

## L

**LSASS.** `lsass.exe`, the Windows process that holds
authentication material in memory. Target of credential dumping
via [`credentials/lsassdump`](techniques/credentials/lsassdump.md).

## M

**MITRE ATT&CK.** Adversary technique taxonomy
(https://attack.mitre.org). Every maldev technique declares its
T-IDs in frontmatter.

**MDX.** Markdown + JSX. Docusaurus syntax; mdBook doesn't
support it. Relevant only if we ever migrate
([ADR-0003](concepts/decisions/0003-mdbook-over-docusaurus.md)).

## N

**NT* API.** Native Windows API in `ntdll.dll`
(`NtAllocateVirtualMemory`, `NtProtectVirtualMemory`, …). Lower-
level than Win32 `kernel32.dll`; the hook layer EDRs care most
about.

**ntdll.** `ntdll.dll`, the lowest-level user-mode DLL. Every
Win32 API ends up here. EDR hooks usually live in its `.text`
section.

## O

**OEP (Original Entry Point).** Where a binary's first
instruction was *before* it got packed. The packer's stub jumps
to OEP after decryption.

**OPSEC.** Operational Security — minimising your attack
surface. Covers tooling artefacts, network behaviour, build
metadata.

## P

**PEB (Process Environment Block).** Per-process structure at a
fixed offset from the GS segment. Holds
`ProcessParameters.CommandLine`, loaded modules, image base.
maldev's PEB-CommandLine patch (RunWithArgs export) lives in
the packer stub.

**Plan 9 asm.** Go's internal assembler syntax. maldev's
`pe/packer/stubgen/amd64` emits raw bytes via Plan9 helpers.

**Plt / IAT thunk.** Per-import indirection slot in the IAT.

## R

**RVA (Relative Virtual Address).** Offset from a PE's image base
(after loading). The linker bakes RVAs; the loader doesn't rewrite
them unless the image is rebased.

**rpc.** Microsoft RPC (`rpcrt4.dll`). Several persistence and
LSASS-dump primitives ride on RPC interfaces.

**Reverse-engineering of `.text`.** Static analysis that reads
the unpacked code section. Defeated by the packer's per-pack
SGN encoding + section-name randomisation
([ADR randomisation default-on](concepts/decisions/) — see
v0.135.0 changelog).

## S

**SGN (Shellcode Generation).** Polymorphic encoder format
(SUB-NEG-NEGATE shape) used by maldev's packer for byte-level
diversity per pack.

**SSN (System Service Number).** Per-Windows-version index used
by the kernel syscall stub. Direct syscalls require resolving SSN
at runtime (`wsyscall` does this).

**Stub.** The decoder bytes injected at the new PE entry by the
packer. Stage1 = current implementation; emitted by
[`pe/packer/stubgen/stage1`](techniques/pe/packer.md).

**SYSTEM.** `NT AUTHORITY\SYSTEM`, the highest non-kernel
account. End goal of most privesc chains.

## T

**TEB (Thread Environment Block).** Per-thread structure
analogous to PEB. Holds TLS slots; relevant for Go runtime
init in injected threads.

## U

**UAC (User Account Control).** Windows prompt that gates
elevation from Medium to High IL. Bypass primitives in
[`privesc/uac`](techniques/privesc/uac.md).

## W

**WIN_CERTIFICATE.** Authenticode signature blob layout (PE's
`DataDirectory[SECURITY]`). Harvested by
[`cmd/cert-snapshot`](tools/cert-snapshot.md), pasted onto packed
binaries for masquerade.

**wsyscall.** The maldev-internal package providing the `Caller`
abstraction over WinAPI / Native / Direct / Indirect syscall
methods. See [ADR-0001](concepts/decisions/0001-wsyscall-caller-pattern.md).

## X

**x64dbg.** Open-source ring-3 debugger. Used to develop the
memscan stack (see [research helpers](tools/research-helpers.md#memory-inspection-memscan-stack)
for the current pure-Go incarnation that replaced the legacy
x64dbg-MCP plumbing).
