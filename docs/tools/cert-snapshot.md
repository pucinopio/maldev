# `cert-snapshot`

> Harvest donor Authenticode certificates for masquerade builds.

**Source:** [`cmd/cert-snapshot/`](https://github.com/oioio-space/maldev/tree/master/cmd/cert-snapshot) · **godoc:** [pkg.go.dev/…/cmd/cert-snapshot](https://pkg.go.dev/github.com/oioio-space/maldev/cmd/cert-snapshot)
**Audience:** operator (build host) · **Platforms:** Windows (donors must be installed)

## Synopsis

```text
cert-snapshot -out <dir>
```

## What it does

Dumps the Authenticode `WIN_CERTIFICATE` blob of every donor PE in
`pe/masquerade/internal/donors.All` to `<dir>/<id>.bin`. Run once on a host
that has the donors installed; ship the resulting blobs alongside your build
toolchain so subsequent builds can graft signatures without the donors
present.

```go
// later, on any build host:
raw, _ := os.ReadFile("certs/claude.bin")
cert.Write("implant.exe", &cert.Certificate{Raw: raw})
```

> [!WARNING]
> The grafted signature is **not** cryptographically valid — the PE hash
> differs from the donor. This fools "has a signature blob?" static checks
> and the file-properties UI, nothing more.

## Build

```bash
go build -o cert-snapshot ./cmd/cert-snapshot
```

## Example

```bash
mkdir -p ignore/certs
cert-snapshot -out ./ignore/certs
ls ignore/certs/
# acrobat.bin  chrome.bin  claude.bin  notepadpp.bin  …
```

## See also

- Technique: [`pe/cert`](../techniques/pe/certificate-theft.md).
- Glossary: [Donor cert](../glossary.md), [WIN_CERTIFICATE](../glossary.md).
