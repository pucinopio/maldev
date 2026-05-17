# `examples/` — runnable tutorials

> One self-contained Go binary per topic. Each example assembles a
> small chain of `maldev` packages and demonstrates a single
> technique end-to-end — built, runnable, and cross-linked to the
> reference docs in [`docs/techniques/`](../docs/techniques/).
>
> Operators wanting to **read the technique** start in the matching
> `docs/techniques/<domain>/` page; operators wanting to **run it**
> start here.

## Naming convention

`<domain>-<technique>` to align with `docs/techniques/<domain>/<technique>.md`.
No `-suite` suffix.

## Catalogue

### Reconnaissance

| Example | What it demonstrates | Reference |
|---|---|---|
| [`recon-host`](recon-host/) | Drive enumeration, special folders, network info, version probes | [`docs/techniques/recon/`](../docs/techniques/recon/) |
| [`recon-stealth-ppid`](recon-stealth-ppid/) | Anti-debug, anti-VM, sandbox detection, PPID spoofing | [`docs/techniques/recon/`](../docs/techniques/recon/) |

### Initial access / payload delivery

| Example | What it demonstrates | Reference |
|---|---|---|
| [`packer-tour`](packer-tour/) | Pack EXE/DLL with the SGN+LZ4 packer, every mode | [`docs/examples/upx-style-packer.md`](../docs/examples/upx-style-packer.md) |
| [`packer-shellcode`](packer-shellcode/) | Mode 6 — pack raw shellcode into self-executing PE | [`docs/techniques/pe/packer.md`](../docs/techniques/pe/packer.md) |

### Execution / injection

| Example | What it demonstrates | Reference |
|---|---|---|
| [`inject-evasive`](inject-evasive/) | Process hollowing + sleepmask + AMSI/ETW patches | [`docs/techniques/injection/`](../docs/techniques/injection/) |
| [`runtime-loaders`](runtime-loaders/) | CLR host + BOF loader composition | [`docs/techniques/runtime/`](../docs/techniques/runtime/) |
| [`syscall-matrix`](syscall-matrix/) | Direct + indirect + WinAPI syscall caller, all wired | [`docs/techniques/syscalls/`](../docs/techniques/syscalls/) |
| [`unhook-ntdll`](unhook-ntdll/) | Restore hooked ntdll functions to disk-image bytes | [`docs/techniques/evasion/`](../docs/techniques/evasion/) |

### Persistence

| Example | What it demonstrates | Reference |
|---|---|---|
| [`persistence-user`](persistence-user/) | User-context persistence (Run keys, scheduled tasks, COM hijack) | [`docs/techniques/persistence/`](../docs/techniques/persistence/) |
| [`persistence-system`](persistence-system/) | SYSTEM-context persistence (services, drivers) | [`docs/techniques/persistence/`](../docs/techniques/persistence/) |

### Privilege escalation

| Example | What it demonstrates | Reference |
|---|---|---|
| [`privesc-dll-hijack`](privesc-dll-hijack/) | **Full chain** — lowuser → SYSTEM via DLL hijack, with packer + AMSI bypass | [`docs/techniques/recon/dll-hijack.md`](../docs/techniques/recon/dll-hijack.md) |
| [`privesc-uac`](privesc-uac/) | UAC bypass primitives | [`docs/techniques/privesc/`](../docs/techniques/privesc/) |

### Credentials / collection

| Example | What it demonstrates | Reference |
|---|---|---|
| [`credentials-dump`](credentials-dump/) | LSASS dump + sekurlsa parse + SAM/SYSTEM hive | [`docs/techniques/credentials/`](../docs/techniques/credentials/) |
| [`tokens-impersonate`](tokens-impersonate/) | Token theft, impersonation, primary-token swap | [`docs/techniques/tokens/`](../docs/techniques/tokens/) |
| [`collection-screen-keylog`](collection-screen-keylog/) | Screenshot capture + keylogger | [`docs/techniques/collection/`](../docs/techniques/collection/) |

### C2

| Example | What it demonstrates | Reference |
|---|---|---|
| [`c2-reverse-shell`](c2-reverse-shell/) | Reverse shell + named-pipe transport + Meterpreter compat | [`docs/techniques/c2/`](../docs/techniques/c2/) |

### Kernel

| Example | What it demonstrates | Reference |
|---|---|---|
| [`kernel-byovd`](kernel-byovd/) | Bring-your-own-vulnerable-driver primitives | [`docs/techniques/kernel/`](../docs/techniques/kernel/) |

### Misc / tooling

| Example | What it demonstrates | Reference |
|---|---|---|
| [`cleanup-artifacts`](cleanup-artifacts/) | Self-delete, log wipe, memory wipe | [`docs/techniques/cleanup/`](../docs/techniques/cleanup/) |
| [`pe-modify`](pe-modify/) | PE-header mutation, masquerade donor swap | [`docs/techniques/pe/`](../docs/techniques/pe/) |
| [`preset-stacks`](preset-stacks/) | Pre-baked evasion stacks (Stealth, Aggressive, …) | [`docs/techniques/evasion/preset.md`](../docs/techniques/evasion/preset.md) |
| [`process-tamper`](process-tamper/) | PEB tampering, command-line spoof, parent-PID spoof | [`docs/techniques/process/`](../docs/techniques/process/) |

## Running an example

Each binary cross-builds for Windows from Linux:

```bash
GOOS=windows GOARCH=amd64 go build -o /tmp/example.exe ./examples/<name>
```

Some examples (`privesc-dll-hijack`, `kernel-byovd`) ship their own
`README.md` with a full setup walkthrough.

## Adding a new example

1. Create `examples/<domain>-<technique>/main.go` with a short header
   docstring (one-line summary, like the existing examples).
2. Run `go run ./internal/tools/docgen` — auto-updates `docs/index.md`.
3. Add a row to the catalogue table above.
4. Cross-link the matching `docs/techniques/<domain>/<technique>.md`
   page back to your example.
