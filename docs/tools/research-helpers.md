# Research & dev helpers

> Tools shipped under `cmd/` that are **not** part of an operator
> loadout. They support packer research, in-VM inspection, build
> reproducibility, and CI. Listed here so you can find them, not
> because they ship to a target.

## Packer research

| Tool | Source | Purpose |
|---|---|---|
| **`packer-vis`** | [`cmd/packer-vis/`](https://github.com/oioio-space/maldev/tree/master/cmd/packer-vis) | Visualise a packed binary — entropy heatmap, section layout, bundle wire-format ASCII art. Use when iterating on stub layout or auditing IOC drift. |
| **`packerscope`** | [`cmd/packerscope/`](https://github.com/oioio-space/maldev/tree/master/cmd/packerscope) | Defender-side companion: detects + dumps + extracts maldev artefacts symmetrically with `packer`. Use for detection engineering. |

## Memory inspection (`memscan` stack)

In-VM memory scanner used by tests + research workflows. Three binaries
work together — none ship to a real target.

| Tool | Source | Purpose |
|---|---|---|
| **`memscan-server`** | [`cmd/memscan-server/`](https://github.com/oioio-space/maldev/tree/master/cmd/memscan-server) | HTTP/JSON API exposed inside the target VM for memory queries. |
| **`memscan-harness`** | [`cmd/memscan-harness/`](https://github.com/oioio-space/maldev/tree/master/cmd/memscan-harness) | Spawns sacrificial processes against which a scan is run. |
| **`memscan-mcp`** | [`cmd/memscan-mcp/`](https://github.com/oioio-space/maldev/tree/master/cmd/memscan-mcp) | Model Context Protocol adapter — relays AI tool calls to `memscan-server`. |

See [memscan stack — memory notes](../glossary.md#m).

## Build / CI helpers

| Tool | Source | Purpose |
|---|---|---|
| **`hashgen`** | [`cmd/hashgen/`](https://github.com/oioio-space/maldev/tree/master/cmd/hashgen) | Pre-compute ROR-13 / FNV-1a API-name hashes for shellcode embedding. Build-time helper. |
| **`vmtest`** | [`cmd/vmtest/`](https://github.com/oioio-space/maldev/tree/master/cmd/vmtest) | Run the Go test suite inside isolated VMs (VirtualBox + libvirt auto-detected). See [Testing](../testing.md). |
| **`test-report`** | [`cmd/test-report/`](https://github.com/oioio-space/maldev/tree/master/cmd/test-report) | Ingest `go test -json` streams, surface flaky tests + coverage gaps. |

## Truly internal (`internal/tools/`)

These don't even live in `cmd/` because they are CI/repo-only:
`build-fixture-winres`, `coverage-merge`, `docgen`, `lsass-dump-test`,
`vm-test-memscan`. They are listed here for completeness only — see
their respective Go files for usage.
