# Package `license/` — Design Spec

- **Date**: 2026-05-20
- **Status**: Approved (brainstorming → implementation plan next)
- **Owner**: Mathieu Bachmann
- **Target module**: `github.com/oioio-space/maldev/license`
- **Stack layer**: Layer 1 (uses `crypto/ed25519` stdlib + `cleanup/memory` + `internal/log` + `license/hostid` Layer 0)

---

## 1. Purpose

Provide a defensive framing mechanism for the maldev test/research binaries. A `license` is a signed, structured token that authorises a specific binary, for a specific subject, on optional sets of machines/passwords/custom constraints, within an optional time window, with optional revocation and heartbeat checks.

The package is consumed in code by binaries (`cmd/*`) and by license-issuing/serving code. CLI wrappers (`cmd/license`) are out of scope for v1 — only the in-code API is delivered.

## 2. Goals & non-goals

### Goals

- Simple, composable Go API with functional options. One-liner usage for the minimal case.
- Hybrid offline + online enforcement. Offline-only mode must work without any network call.
- Asymmetric crypto: Ed25519 (signature 64 B, key 32 B, deterministic, no algorithm-confusion).
- PEM-armored canonical JSON: human-inspectable, embeddable, copy-pasteable.
- Multi-binding constraints: machine (list), password (Argon2id), custom k/v, extensible interface.
- Key rotation via `KeyID`; trusted-key map at verify time.
- Audience (`aud`) + Issuer (`iss`) for multi-tenant/multi-binary isolation.
- Binary identity pinning that survives `cmd/packer` transformations (embedded identity bytes).
- Plug-pluggable revocation source (HTTP, File, Embed, Multi, custom interface).
- Signed revocation list with monotonic sequence + chain hash + signed expiry.
- Heartbeat with nonce echo + signed server time.
- Clock-tamper detection via trusted-floor + monotonic last-seen, both in an HMAC-protected local state file.
- Optional NTP cross-check (soft warning by default, strict mode opt-in).
- Sealed payload encryption (X25519 + ChaCha20-Poly1305) for sensitive license-bound config.
- Constant-time crypto comparisons; secret material wiped via `cleanup/memory`.
- Opaque public error (`ErrLicenseInvalid`); detailed cause routed to `slog.Logger`.
- Server-side `http.Handler` helpers (revocation publish + heartbeat) that the developer wraps in their own `http.Server`.

### Non-goals (documented, not implemented)

- Anti-debug / anti-tamper of the binary itself (out of scope; covered by `evasion/`).
- Seat counting (max N machines simultaneously) — needs stateful DB server.
- Perfect clock anti-rollback (would require TPM/enclave).
- Sub-licenses / delegated issuance (signature chains, inherited constraints).
- Transferable licenses revoking the previous holder atomically.
- EDR/sandbox detection inside the license check.
- Full-binary encryption gated on license (the existing `cmd/packer` already does that).
- CLI binary `cmd/license` (v2).
- HSM/PKCS#11 issuance (v2).
- COSE_Sign1 alternative format (v2).
- DB-backed server (v2).

## 3. Package architecture

```
license/
├── doc.go                  -- defensive framing, threat model, conventions
├── license.go              -- License type, Issue/Verify entry points
├── sign.go                 -- Issue / canonical signing
├── verify.go               -- Verify entry point + flow
├── verify_options.go       -- WithAudience, WithMachineID, WithPassword, ...
├── keys.go                 -- GenerateKey, MarshalPublicKey/PrivateKey, TrustedKeys
├── pem.go                  -- PEM block encode/decode
├── errors.go               -- ErrLicenseInvalid + internal cause enum
├── clock.go                -- Clock interface + realClock
├── state.go                -- HMAC-protected local state file (trusted_floor, ...)
├── *_test.go
│
├── canonical/              -- deterministic JSON (sorted keys, no-whitespace)
├── hostid/                 -- cross-platform machine fingerprint
│   ├── hostid.go           -- API
│   ├── hostid_windows.go   -- MachineGuid + DMI hints
│   ├── hostid_linux.go     -- /etc/machine-id + DMI
│   └── hostid_darwin.go    -- IOPlatformUUID best-effort
│
├── identity/               -- embedded build-time identity bytes
│   ├── identity.go         -- Set / Read runtime API
│   └── cmd/gen-identity/   -- go-generate-able 32-random-bytes producer
│
├── revoke/                 -- revocation list + cache + source interface
│   ├── list.go
│   ├── source.go           -- RevocationSource + builtins (HTTP, File, Embed, Multi)
│   └── cache.go
│
├── heartbeat/              -- client + server-handler primitives
│   ├── client.go
│   └── server.go
│
├── seal/                   -- X25519 + ChaCha20-Poly1305 sealed payload
│
├── ntp/                    -- optional NTP cross-check
│
└── server/                 -- http.Handler for revocation publish + heartbeat
    ├── revocation.go
    ├── heartbeat.go
    └── store.go            -- RevocationStore + LicenseStore interfaces, FileStore builtin
```

All sub-packages are exported (no `internal/`). Layer-1 status holds — only Layer-0 deps from the repo plus stdlib.

## 4. Data model

### 4.1 License (the signed body)

```go
type License struct {
    Version int       `json:"v"`             // always 1 in v1
    ID      string    `json:"id"`            // UUIDv4 from crypto/rand
    KeyID   string    `json:"kid"`           // signing key identifier
    Issuer   string   `json:"iss"`
    Subject  string   `json:"sub"`
    Audience []string `json:"aud,omitempty"`
    IssuedAt  time.Time `json:"iat"`
    NotBefore time.Time `json:"nbf,omitempty"`
    NotAfter  time.Time `json:"exp,omitempty"`
    Bindings []Binding `json:"bnd,omitempty"`
    BinarySHA256   string `json:"bin,omitempty"`     // sha256 of os.Executable contents
    IdentitySHA256 string `json:"id_sha,omitempty"`  // sha256 of embedded identity bytes
    Payload       json.RawMessage `json:"pld,omitempty"`  // dev-defined, in clear
    SealedPayload []byte          `json:"spld,omitempty"` // X25519 sealed
}

type Binding struct {
    Type  string   `json:"t"`              // "machine", "password", "custom:<name>"
    Value []string `json:"v,omitempty"`    // multi-valued = OR; empty for password
    Hash  []byte   `json:"h,omitempty"`    // argon2id digest, only for password
    Salt  []byte   `json:"s,omitempty"`    // 16 random bytes, only for password
}
```

### 4.2 Wire wrapper (PEM-armored)

```go
type signedLicense struct {
    License   License `json:"lic"`
    Signature []byte  `json:"sig"`         // ed25519 over domain-tag||canonical(License)
    KeyID     string  `json:"kid"`         // duplicated outside License for key lookup pre-parse
}
```

On disk:

```text
-----BEGIN MALDEV LICENSE-----
<base64 of canonical JSON of signedLicense>
-----END MALDEV LICENSE-----
```

### 4.3 Revocation list

```go
type RevocationList struct {
    Version    int       `json:"v"`
    KeyID      string    `json:"kid"`
    Sequence   uint64    `json:"seq"`        // monotonic
    PrevHash   []byte    `json:"prev"`       // sha256(previous signed list), nil for first
    IssuedAt   time.Time `json:"iat"`
    ExpiresAt  time.Time `json:"exp"`        // hard expiry, signed
    ServerTime time.Time `json:"st"`         // trusted time anchor
    Revoked    []string  `json:"rev"`        // license IDs
}
```

### 4.4 Heartbeat reply

```go
type HeartbeatReply struct {
    Version    int       `json:"v"`
    KeyID      string    `json:"kid"`
    LicenseID  string    `json:"lid"`
    Ok         bool      `json:"ok"`
    Reason     string    `json:"r,omitempty"`
    NonceEcho  []byte    `json:"n"`
    ServerTime time.Time `json:"st"`
    ValidUntil time.Time `json:"vu"`
}
```

### 4.5 Local state file (HMAC-protected)

```go
type State struct {
    TrustedFloor      time.Time `json:"tf"`     // max server_time ever observed
    LastSeenLocal     time.Time `json:"lsl"`    // max time.Now() ever observed
    LastSeenSequence  uint64    `json:"lss"`    // max revocation sequence ever observed
    LastFetchOk       time.Time `json:"lfo"`
    LastHeartbeatOk   time.Time `json:"lho"`
}
```

Stored as `{state, hmac}`. HMAC key = HKDF(license signature || hostid.Local()).

## 5. Public API

### 5.1 Issuing

```go
// One-liner
data, err := license.New(priv, "alice@example.com", 6*30*24*time.Hour)

// Full options
data, err := license.Issue(license.IssueOptions{
    PrivateKey:   priv,
    KeyID:        "k2026-05",
    Subject:      "alice@example.com",
    Issuer:       "maldev-lab-eu",
    Audience:     []string{"rshell", "memscan-server"},
    NotAfter:     time.Now().AddDate(0, 6, 0),
    Bindings: []license.Binding{
        license.BindMachineIDs("abc123", "def456"),
        license.BindPassword("s3cr3t"),
        license.BindCustom("project", "WRAITH-2026"),
    },
    BinarySHA256:   license.HashFile("rshell-packed.exe"),
    IdentitySHA256: license.HashIdentity(identityBytes),
    Payload:        mustJSON(myConfig),
})
```

### 5.2 Verifying

```go
// Minimal (offline pure)
lic, err := license.Verify(data, pub)

// Realistic production verify
lic, err := license.Verify(data,
    license.Trusted{Keys: map[string]ed25519.PublicKey{
        "k2026-05": pubCurrent,
        "k2025-11": pubOld,
    }},
    license.WithAudience("rshell"),
    license.WithIssuer("maldev-lab-eu"),
    license.WithMachineID(hostid.Local()),
    license.WithPassword(passphrase),
    license.WithBinaryPinning(),
    license.WithRevocation(
        revoke.HTTPSource("https://lic.maldev.test/revoked.json", nil),
        24*time.Hour,
        "~/.maldev/revoke.signed",
    ),
    license.WithGracePeriod(7*24*time.Hour),
    license.WithHeartbeat(heartbeat.HTTPClient(url), 1*time.Hour),
    license.WithMaxClockSkew(5*time.Minute),
    license.WithNTPCheck("pool.ntp.org", 10*time.Minute),
    license.WithClock(realClock),
    license.WithLogger(slog.Default()),
    license.WithStateFile("~/.maldev/license-state.signed"),
    license.WithContext(ctx),
)
```

### 5.3 Returned type

```go
type Verified struct {
    License            // verified content (read-only)
    Payload  []byte    // cleartext (after seal-open if applicable)
    KeyUsed  string    // KeyID that validated
    Warnings []string  // soft NTP drift, missing pinning data, etc.
}
```

### 5.4 Binding semantics

| `License.BinarySHA256` | `License.IdentitySHA256` | Behaviour when `WithBinaryPinning()` is set |
|---|---|---|
| empty | empty | Warning logged ("pinning requested but license carries nothing"); no rejection |
| set | empty | Hash `os.Executable()` contents; compare to disk hash |
| empty | set | Hash `identity.Read()`; compare to identity hash |
| set | set | Both must match (AND) |

### 5.5 Errors

```go
var ErrLicenseInvalid = errors.New("license: verification failed")
```

Internal cause enum is **not exported**, **not stringified into the returned error**, but logged via the injected `slog.Logger`. `errors.Is(err, ErrLicenseInvalid)` is the only public discrimination.

Causes (internal): `causeBadFormat`, `causeBadSignature`, `causeUnknownKey`, `causeNotYetValid`, `causeExpired`, `causeClockRollback`, `causeAudienceMismatch`, `causeIssuerMismatch`, `causeBindingMachineMismatch`, `causeBindingPasswordMismatch`, `causeBindingCustomMismatch`, `causeBinaryHashMismatch`, `causeIdentityMismatch`, `causeRevoked`, `causeRevocationStale`, `causeHeartbeatFailed`, `causeStateCorrupted`.

## 6. Verification flow (chronological)

Cheap → expensive ordering. Each step fails fast.

1. **Format**: PEM block type check, size ≤ `MaxLicenseSize` (16 KB), JSON parse, `Version == 1`.
2. **Key resolution**: `License.KeyID ∈ Trusted.Keys`.
3. **Signature**: `ed25519.Verify(pub, "maldev-license-v1\x00" || canonical(License), Signature)`.
4. **State file read** (HMAC-protected): corrupted → log + reset; rollback detection (`time.Now() < max(TrustedFloor, LastSeenLocal) − skew`) → `causeClockRollback`.
5. **Time**: `NotBefore > now + skew` → `causeNotYetValid`; `NotAfter < now − skew` → `causeExpired`.
6. **Audience/Issuer**: caller-provided audience must intersect `License.Audience` (empty audience = wildcard with warning); issuer must match.
7. **Bindings**: every binding in license must have matching evidence.
8. **Binary/Identity pinning**: AND of present fields (see §5.4).
9. **Revocation**:
   - Read cache (signature, sequence ≥ `LastSeenSequence`, `now < ExpiresAt`).
   - If stale → `Fetch()` from `RevocationSource` honouring `ctx`.
   - On fail + cache expired + `now > LastFetchOk + grace` → `causeRevocationStale`.
   - `License.ID ∈ Revoked` → `causeRevoked`.
   - On fetch success: update `TrustedFloor = max(TrustedFloor, ServerTime)`.
10. **Heartbeat** (if configured): POST with `nonce`; verify signed reply, nonce echo, `ok==true`; update `TrustedFloor` + `LastHeartbeatOk`.
11. **NTP** (if configured): query; drift > maxDrift → `Warnings` (soft) or `causeClockRollback` (strict).
12. **State write**: atomic (`os.WriteFile(tmp) + os.Rename`), updates `LastSeenLocal`, `TrustedFloor`, `LastSeenSequence`, `LastFetchOk`, `LastHeartbeatOk`.
13. **Wipe**: `cleanup/memory` over password material and any sensitive intermediate buffers.

## 7. Issuing flow

1. Validate `IssueOptions`. Warn if no constraints set at all.
2. Build `License`: random UUIDv4 from `crypto/rand`, `IssuedAt = time.Now().UTC()`, helpers apply argon2id to passwords.
3. Canonical JSON marshal of `License`.
4. `signed = ed25519.Sign(priv, "maldev-license-v1\x00" || canonical)`.
5. Assemble `signedLicense{License, signed, KeyID}`, canonical marshal again.
6. Base64 → PEM block `"MALDEV LICENSE"`.
7. Optional wipe of `priv` if `WithEphemeralKey` was used.

## 8. Server-side helpers (`license/server`)

Two `http.Handler` builders + two persistence interfaces:

```go
// RevocationStore: dev implements Load/Save. Builtin: FileStore.
// LicenseStore   : dev implements Status. Builtin: FileStore.

server.NewRevocationHandler(server.RevocationOptions{
    PrivateKey:  priv,
    KeyID:       "k2026-05",
    Store:       server.FileStore("./revoked.json"),
    ValidFor:    7 * 24 * time.Hour,
    AdminToken:  os.Getenv("MALDEV_ADMIN"),
    Logger:      slog.Default(),
})
// GET /revoked.json  → signed RevocationList
// POST /revoked.json → adds license_ids (Bearer auth)

server.NewHeartbeatHandler(server.HeartbeatOptions{
    PrivateKey: priv,
    KeyID:      "k2026-05",
    Store:      server.FileStore("./licenses.json"),
    ValidFor:   1 * time.Hour,
})
// POST /heartbeat → signed HeartbeatReply
```

Developer chooses port, TLS, routing, logging by wrapping in their own `http.Server`.

## 9. Sub-package summaries

### `license/canonical`

Deterministic JSON: recursively-sorted map keys, no whitespace, no HTML escaping, fixed time format `RFC3339Nano UTC`. `Marshal(v any) ([]byte, error)`. Used everywhere a signature happens.

### `license/hostid`

`Local() ([]byte, error)` — returns a 32-byte sha256 over OS-provided identifiers:
- Windows: registry `HKLM\SOFTWARE\Microsoft\Cryptography\MachineGuid` + DMI hints.
- Linux: `/etc/machine-id` + DMI hints.
- Darwin: `IOPlatformUUID` via `ioreg` (best-effort).

Multiple sources are mixed via sha256 to defeat trivial swapping. Function is read-only, no side effects.

### `license/identity`

Run-time API:
- `Set(b []byte)` — called once at init from the binary's `//go:embed identity.bin`.
- `Read() []byte` — returns the registered bytes.
- `HashIdentity(b []byte) string` — convenience hex sha256.
- `cmd/gen-identity` — `go:generate`-friendly tool. Writes 32 random bytes to `identity.bin` if absent, idempotent.

### `license/revoke`

- `type List` + `Sign`, `Verify`, `IsRevoked` methods.
- `type RevocationSource interface { Fetch(ctx) ([]byte, error) }`.
- Builtins: `HTTPSource`, `FileSource`, `EmbedSource`, `MultiSource`.
- `Cache`: read/write signed blob, validates sequence monotonicity vs local state, atomic write.

### `license/heartbeat`

- `type Client interface { Ping(ctx, licenseID, nonce []byte) (HeartbeatReply, []byte, error) }` — second return is the signature for caller verification.
- Builtin: `HTTPClient(url)`.

### `license/seal`

- `Seal(recipientPub []byte, plaintext []byte) ([]byte, error)` — X25519 + ChaCha20-Poly1305 with random nonce.
- `Open(recipientPriv []byte, sealed []byte) ([]byte, error)`.
- 32-byte ephemeral pubkey is prepended to the ciphertext.

### `license/ntp`

- `Query(server string, timeout time.Duration) (time.Time, time.Duration, error)` — second return is round-trip drift estimate.
- SNTPv4 minimal client, no auth.

### `license/server`

See §8.

## 10. Threat model (honest)

**Resists:**
- License forgery (Ed25519).
- License tampering after issuance (signature covers all fields).
- Trivial replay across audiences (`aud`).
- License reuse across binaries (audience + binary/identity pinning).
- Cache substitution with older revocation list (sequence monotonicity + chain hash).
- Stale cache + offline indefinitely (signed `expires_at` + grace period).
- Side-channel guessing of which constraint failed (opaque error + constant-time compare).
- Brute-force on password binding (argon2id with sane parameters: 64 MiB, t=3, p=4).
- Clock rollback to bypass `NotAfter` (trusted_floor + monotonic last_seen).
- Algorithm-confusion attacks on signature (single algo, domain-separated payload).

**Does NOT resist:**
- An attacker patching `Verify` in the binary to `return nil`. Mitigation: out-of-scope (use packer + code-signing).
- Permanent offline use beyond grace period after key rotation (intentional).
- Full clock-tamper on a machine that never reaches the issuer's servers (best-effort only without TPM).
- Attacker who modifies binary + identity together. Mitigation: out-of-scope.
- Seat sharing where two machines share the same hostid (impossible without per-machine secret).

## 11. Conventions compliance

- Naming: PascalCase exports, camelCase unexports, `ID`/`HTTP` per CLAUDE.md.
- `doc.go` per package with technique label, MITRE = N/A (defensive), detection level = N/A.
- Tech-md page `docs/techniques/license-framing.md`, audience-tagged sections.
- `%w` at end of `fmt.Errorf`, `%v` at boundaries.
- Accept interfaces, return concrete types.
- Comments explain WHY where non-obvious.
- All Go commits go through `/simplify`.
- README `Packages` table updated in the same commit as the package introduction.
- `docs/mitre.md` entry "N/A — defensive framing".
- `cleanup/memory` used for wiping secrets, not a custom implementation.
- `internal/log` used for the slog default, not a new logging primitive.

## 12. Testing plan

### Unit (host, no network, no VM)

- `license_test.go`: round-trip, omitempty.
- `verify_test.go`: every internal `cause*` isolated; 100% branch coverage on the cause table.
- `keys_test.go`: GenerateKey, Marshal/Parse PEM, malformed inputs.
- `canonical/canonical_test.go`: determinism, nested maps, vs stdlib `encoding/json`.
- `revoke/list_test.go`, `revoke/cache_test.go`: signing, chain, sequence monotonicity, corruption auto-reset.
- `hostid/hostid_test.go`: smoke per platform via build tags.
- `seal/seal_test.go`: round-trip, wrong key, tampered ciphertext.
- `identity/identity_test.go`: Set/Read, double-Set panic, size limit.

All time-related tests use `WithClock(testClock)`. No `time.Sleep`. Race tests with `-race` on cache/state files.

### Integration (host, httptest)

- End-to-end Issue → publish → Verify with revocation → revoke → Verify denied.
- Heartbeat success/failure scenarios.
- `MultiSource`: first fails → second succeeds.
- Concurrent Verify on shared cache (100 goroutines) — no data race, no corruption.

### VM (`scripts/vm-run-tests.sh`)

| Test | VM | Reason |
|---|---|---|
| `TestHostIDLocal_Windows` | windows | Real `MachineGuid` registry read |
| `TestHostIDLocal_Linux` | linux | Real `/etc/machine-id` read |
| `TestBinaryPinning_OnDisk` | windows + linux | OS-dependent `os.Executable()` and PE side-effects |
| `TestIdentityPinning_AfterPack` | windows | Build → `cmd/packer` → Verify identity match |
| `TestClockTamper` | windows + linux | Real `Set-Date` / `date -s` (admin/root) |
| `TestStateFileAtomicWrite` | windows | NTFS sharing modes |
| `TestRevocationCacheCrashSafety` | linux | `kill -9` mid-write, reboot, cache integrity |

### E2E (`cmd/license-test`)

200-line binary exercising the full pipeline: keygen → server → issue 5 profiles → verify each → revoke one → re-verify → tamper clock → restart server → recover.

### Adversarial

- Single-bit mutation on signed bytes → reject (statistical sweep over 1% of bytes).
- Replay of older revocation list (sequence regression) → reject.
- Substituted key (license.kid points to attacker key not in Trusted) → reject.
- Argon2 timing: 10 000 verifies, mean-difference between right and wrong password below configurable timing threshold.
- DoS: 100 MB license → rejected before JSON parse.
- Zero-byte / truncated / random-bytes cache file → auto-reset, never panic.

## 13. Out-of-scope v1 / future work

- **CLI `cmd/license`**: wraps the public functions, ~150 LoC. Comes after the package is exercised in code by ≥2 binaries.
- **Stateful DB-backed server**: SQLite optional, seat counter, audit log.
- **HSM / PKCS#11 signing**: YubiKey-backed issuance.
- **COSE_Sign1 alternative format**: for CBOR ecosystems.
- **Telemetry de-dup via `IdentitySHA256`**: reuse of `license/identity` outside license context.
- **Heartbeat with seat counter**: needs the DB-backed server.

## 14. Documentation deliverables (same commit)

- `license/doc.go` — defensive framing, threat-model summary, conventions.
- `docs/techniques/license-framing.md` — full tech-md page with Operator / Researcher / Detection-eng paths, following the 4-layer pedagogy pattern (TL;DR → vocabulary → flow diagram → narrated examples → decision table).
- `docs/license/workflow.md` — concrete genkey → issue → verify → revoke → rotate flows.
- `docs/license/threat-model.md` — §10 expanded.
- `README.md` `Packages` table update.
- `docs/mitre.md` entry "N/A — defensive framing".
