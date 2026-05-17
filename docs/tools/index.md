# CLI tools (`cmd/`)

> Operator-facing binaries shipped from the `cmd/` tree. Each tool
> is a self-contained CLI you can `go build` and ship as part of
> your loadout.

Internal dev/CI helpers live under `internal/tools/` (out of this
list deliberately — they aren't intended for operator use).

## Catalogue

### Packer & PE manipulation

| Tool | Summary | Source |
|---|---|---|
| [`packer`](packer.md) | CLI wrapper around `pe/packer.PackBinary` — pack EXE/DLL with the SGN+LZ4 stub, with operator-controllable flags (`-mode`, `-compress`, `-antidebug`, `-randomize`, …) | [`cmd/packer/`](https://github.com/oioio-space/maldev/tree/master/cmd/packer) |
| [`packer-vis`](packer-vis.md) | Visual diff/layout introspection on packer outputs (sections, RVAs, entrypoint, decoder span) | [`cmd/packer-vis/`](https://github.com/oioio-space/maldev/tree/master/cmd/packer-vis) |
| [`packerscope`](packerscope.md) | Defender-side companion — emulates loader steps, decodes the stub, surfaces IOCs | [`cmd/packerscope/`](https://github.com/oioio-space/maldev/tree/master/cmd/packerscope) |
| [`cert-snapshot`](cert-snapshot.md) | Dumps a PE's Authenticode `WIN_CERTIFICATE` blob — used to harvest donor certs for masquerade | [`cmd/cert-snapshot/`](https://github.com/oioio-space/maldev/tree/master/cmd/cert-snapshot) |

### Loader & runtime

| Tool | Summary | Source |
|---|---|---|
| [`bof-runner`](bof-runner.md) | Execute a Beacon Object File (`.o` COFF) standalone and print its output | [`cmd/bof-runner/`](https://github.com/oioio-space/maldev/tree/master/cmd/bof-runner) |
| [`bundle-launcher`](bundle-launcher.md) | Operator runtime for the C6 multi-target bundle (AES-CTR encrypted payload table) | [`cmd/bundle-launcher/`](https://github.com/oioio-space/maldev/tree/master/cmd/bundle-launcher) |
| [`rshell`](rshell.md) | Minimal reverse shell using `c2/shell` + `c2/transport` | [`cmd/rshell/`](https://github.com/oioio-space/maldev/tree/master/cmd/rshell) |
| [`sleepmask-demo`](sleepmask-demo.md) | Run encrypted-sleep scenarios against a configurable mask | [`cmd/sleepmask-demo/`](https://github.com/oioio-space/maldev/tree/master/cmd/sleepmask-demo) |

### Memory inspection (memscan stack)

| Tool | Summary | Source |
|---|---|---|
| [`memscan-server`](memscan-server.md) | HTTP/JSON memory-inspection API exposed inside the target | [`cmd/memscan-server/`](https://github.com/oioio-space/maldev/tree/master/cmd/memscan-server) |
| [`memscan-harness`](memscan-harness.md) | Target-side helper that spawns sacrificial processes for scan tests | [`cmd/memscan-harness/`](https://github.com/oioio-space/maldev/tree/master/cmd/memscan-harness) |
| [`memscan-mcp`](memscan-mcp.md) | Model Context Protocol adapter — relays Claude tool calls to `memscan-server` | [`cmd/memscan-mcp/`](https://github.com/oioio-space/maldev/tree/master/cmd/memscan-mcp) |

### Dev / CI helpers

| Tool | Summary | Source |
|---|---|---|
| [`hashgen`](hashgen.md) | Pre-compute API-name hashes (ROR-13 etc.) for shellcode embedding | [`cmd/hashgen/`](https://github.com/oioio-space/maldev/tree/master/cmd/hashgen) |
| [`vmtest`](vmtest.md) | Run the Go test suite inside isolated VMs (VBox / libvirt auto-detect) | [`cmd/vmtest/`](https://github.com/oioio-space/maldev/tree/master/cmd/vmtest) |
| [`test-report`](test-report.md) | Ingest `go test -json` streams, surface flaky tests / coverage gaps | [`cmd/test-report/`](https://github.com/oioio-space/maldev/tree/master/cmd/test-report) |

## Conventions

- Every CLI accepts `-h` / `-help` and prints a one-screen usage.
- Verbose modes use `-v` flag (not `-verbose`).
- File-path arguments are positional when there's only one obvious
  input/output; otherwise named flags (`-in`, `-out`).
- Each tool ships a `main.go` header docstring with intent + examples
  — the per-tool pages in this section recapture that text.
