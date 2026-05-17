# `cert-snapshot`

Source: [`cmd/cert-snapshot/`](https://github.com/oioio-space/maldev/tree/master/cmd/cert-snapshot) ·
godoc: [pkg.go.dev/github.com/oioio-space/maldev/cmd/cert-snapshot](https://pkg.go.dev/github.com/oioio-space/maldev/cmd/cert-snapshot)

## What it does

Command cert-snapshot dumps the Authenticode WIN_CERTIFICATE
blob of every donor PE listed in
pe/masquerade/internal/donors.All to a target directory.
Operators run this once on a host that has the donors installed,
commit the resulting `<id>.bin` blobs to a build-side directory
(typically gitignored — these are large binaries), then graft
onto the implant at build time without needing the donor
available:
	go run ./cmd/cert-snapshot -out ./ignore/certs
	# later, on a different build host:
	c, _ := os.ReadFile("./ignore/certs/claude.bin")
	cert.Write("implant.exe", &cert.Certificate{Raw: c})
The grafted signature is NOT cryptographically valid (the PE
hash differs); this only fools static "does the file have a
signature blob?" checks and the file-properties UI. Real
validity needs the donor's private key, which is not on disk.

## Build

```bash
GOOS=windows GOARCH=amd64 go build -o cert-snapshot.exe ./cmd/cert-snapshot
```

For platform-native builds, drop the `GOOS` / `GOARCH` prefix.

## Help / flags

Run with `-h` to see the current flag set:

```bash
./cert-snapshot -h
```

## Related

- Reference for the underlying packages: see the [Techniques tree](../techniques/).
- Runnable examples: see [Runnable examples](../examples/runnable.md).
