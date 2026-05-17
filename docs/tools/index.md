# CLI tools

> The `cmd/` tree ships **6 operator binaries** + a handful of research /
> dev / CI helpers. Most users only need the operator binaries — the rest
> exist to support packer research, in-VM testing, and CI workflows.

## Operator binaries

Build them with `go build -o <name> ./cmd/<name>`; pass `-h` for the live
flag set. Cross-compile with `GOOS=windows GOARCH=amd64` as usual.

| Tool | One-liner |
|---|---|
| [`packer`](packer.md) | Pack / unpack / bundle PE + ELF payloads with the SGN+LZ4 stub. |
| [`bundle-launcher`](bundle-launcher.md) | Runtime dispatcher for `packer bundle` multi-target blobs. |
| [`bof-runner`](bof-runner.md) | Standalone runner for Cobalt-Strike-compatible Beacon Object Files. |
| [`cert-snapshot`](cert-snapshot.md) | Harvest donor Authenticode certificates for masquerade builds. |
| [`rshell`](rshell.md) | Minimal reverse shell over `c2/shell` + `c2/transport`. |
| [`sleepmask-demo`](sleepmask-demo.md) | Demo harness comparing sleep masks under a concurrent scanner. *(research)* |

## Research & dev helpers

Tools that don't belong on a target. Consolidated on a single page to keep
the navigation honest:

- [Research & dev helpers](research-helpers.md) — `packer-vis`, `packerscope`,
  the three-binary `memscan` stack, `hashgen`, `vmtest`, `test-report`.

## Conventions

- Every CLI accepts `-h` / `-help` and prints a one-screen usage.
- File-path arguments are positional when there is one obvious in/out;
  otherwise named flags (`-in`, `-out`).
- Verbose mode is `-v` (never `-verbose`).
- Each `main.go` carries a header docstring with the intent + an example;
  the per-tool page recaptures it and pins the flag set.
