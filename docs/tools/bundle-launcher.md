# `bundle-launcher`

> Runtime launcher for C6 multi-target bundles built by `packer bundle`.

**Source:** [`cmd/bundle-launcher/`](https://github.com/oioio-space/maldev/tree/master/cmd/bundle-launcher) · **godoc:** [pkg.go.dev/…/cmd/bundle-launcher](https://pkg.go.dev/github.com/oioio-space/maldev/cmd/bundle-launcher)
**Audience:** operator · **Platforms:** Windows + Linux

## What it does

Generic single-binary launcher that:

1. Reads its own image via `os.Executable()`.
2. Validates the trailing `MLDV-END` footer ([`packer.ExtractBundle`](https://pkg.go.dev/github.com/oioio-space/maldev/pe/packer)).
3. Matches a payload against host CPUID vendor + Windows build ([`packer.MatchBundleHost`](https://pkg.go.dev/github.com/oioio-space/maldev/pe/packer)).
4. Decrypts the matched payload ([`packer.UnpackBundle`](https://pkg.go.dev/github.com/oioio-space/maldev/pe/packer)).
5. Executes plaintext from a `memfd_create` FD (Linux, zero-disk) or temp file (Windows).

Exits cleanly when no entry matches and `FallbackBehaviour = BundleFallbackExit`.

## Build

```bash
go build -o bundle-launcher.exe ./cmd/bundle-launcher
```

## Example

```bash
# 1. Build the launcher once
go build -o bundle-launcher.exe ./cmd/bundle-launcher

# 2. Build a multi-target bundle
packer bundle -out bundle.bin \
  -pl payload-w11.exe:intel:22000-99999 \
  -pl payload-w10.exe:amd:10000-19999  \
  -pl fallback.exe:*:*-*

# 3. Wrap the bundle inside the launcher
packer bundle -wrap bundle-launcher.exe -bundle bundle.bin -out app.exe

# 4. Ship app.exe — runtime dispatches per host
./app.exe
```

## See also

- Technique: [`pe/packer` bundle mode](../techniques/pe/packer.md).
- Companion: [`packer`](packer.md).
