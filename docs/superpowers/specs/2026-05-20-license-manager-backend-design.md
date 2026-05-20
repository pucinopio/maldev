# License Manager — Backend Design Spec

- **Date:** 2026-05-20
- **Status:** Approved (brainstorming → implementation plan next)
- **Owner:** Mathieu Bachmann
- **Module target:** `github.com/oioio-space/maldev/cmd/license-manager` + `internal/manager/*`
- **Depends on:** `github.com/oioio-space/maldev/license` ≥ v0.160.0

---

## 1. Purpose

`license-manager` is a local-first TUI tool (bubbletea + lipgloss) that lets a single operator manage the lifecycle of maldev research licences without leaving the keyboard. This spec covers **only the backend layer** (`internal/manager/`) — the TUI is designed separately and consumes these services.

The backend exposes:

- Persistent storage (ENT + SQLite, pure-Go) with column-level encryption-at-rest for secrets
- Domain services for every operational action (issue, revoke, re-issue, rotate keys, manage identities, etc.)
- Lifecycle of three HTTP servers — revocation, heartbeat, and a new **fingerprint probe** server — each independently startable/stoppable on operator command
- An embedded probe binary per OS/arch served by the probe HTTP server, so a remote machine can produce its `hostid.Local()` + `Composite()` fingerprint with a single copy-paste curl command
- An audit trail of every operator action

The TUI sits on top of `*service.Services` and never touches the store or the encryption layer directly.

## 2. Goals & non-goals

### Goals (v1)

- Pure-Go end-to-end. No cgo. Cross-compile Linux/macOS/Windows without extra tooling.
- Encryption-at-rest of issuer private keys, recipient X25519 private keys, TOTP secrets, and HTTP admin tokens.
- HTTP servers are **OFF by default**. Started only on explicit operator command. Confirmation on exit if any are still running (configurable).
- Atomic operations + audit: every mutating service call writes the business row(s) and the corresponding `AuditEvent` in a single SQLite transaction.
- Re-issue chain via `replaces_license_id` so historical licences stay auditable.
- Probe binary served per-OS/arch from embedded `//go:embed` payloads. Operator copy-pastes a single curl one-liner to the remote machine.
- Real-time delivery of probe results to the subscribing wizard via channel, with DB persistence so results survive restarts and support batch use cases.

### Non-goals (v1)

- OS keystore integration (DPAPI / Keychain / libsecret) — deferred.
- Multi-operator / multi-tenant — single operator, single DB.
- Floating-seat enforcement — depends on stateful server design in `license/` that is itself backlog.
- Push webhooks for downstream backends.
- Sync between manager instances on different machines.
- Encrypted backup export — `cp manager.db` is sufficient at v1.
- License-chain visual graph — the `replaces_license_id` field is populated; the v1 UI shows the immediate parent/successor links, not the full DAG.

## 3. Architecture

```text
cmd/license-manager/
  main.go                  -- boot, passphrase resolution, TUI launch
  flags.go                 -- CLI flags

internal/manager/
  doc.go

  crypto/                  -- column-level encryption (passphrase → KEK → DEK)
    kek.go                 -- Argon2id KDF from passphrase + 16-byte salt
    cipher.go              -- ChaCha20-Poly1305 wrap/unwrap
    types.go               -- EncryptedBlob alias for ENT columns
    crypto_test.go

  store/                   -- ENT-generated client
    schema/                -- ENT entity definitions (one file per entity)
    store.go               -- New(path, kek) (*Store, error)
    migrate.go             -- auto-migrate on open
    store_test.go

  service/
    issuer.go              -- IssuerService
    license.go             -- LicenseService (Issue, List, Inspect, ReIssue, Revoke, Export, Import)
    revoke.go              -- RevokeService (CRL management, publish signed)
    identity.go            -- IdentityService
    recipient.go           -- RecipientService
    totp.go                -- TOTPService
    probe.go               -- ProbeService (token lifecycle, subscriber notification)
    audit.go               -- AuditService
    settings.go            -- SettingsService
    services.go            -- bundle struct wiring all services

  httpsrv/                 -- shared lifecycle for 3 HTTP servers
    server.go              -- Server interface + Status + Event types
    revocation.go          -- wraps license/server.NewRevocationHandler
    heartbeat.go           -- wraps license/server.NewHeartbeatHandler
    probe.go               -- the fingerprint probe HTTP server
    bundle.go              -- Bundle aggregates the 3 + MergedEvents()
    httpsrv_test.go

  probe/                   -- in-process side of the fingerprint probe
    agents/                -- prebuilt binaries (//go:embed agents/*)
      linux-amd64
      linux-arm64
      darwin-amd64
      darwin-arm64
      windows-amd64.exe
      gen/                 -- source of the agent + go:generate directives
        main.go
    embed.go               -- ServeAgent(w, os/arch)
    types.go               -- AgentResult shape

  tui/                     -- bubbletea models (designed separately, NOT in this spec)
```

External dependencies added:

- `entgo.io/ent` (ORM)
- `modernc.org/sqlite` (pure-Go SQLite driver, no cgo)
- `golang.org/x/crypto/{argon2,hkdf,chacha20poly1305}` (already in go.sum)
- `github.com/oioio-space/maldev/license/*` (in-repo)

## 4. Schema (ENT)

All times are stored in UTC. Encrypted columns use the `EncryptedBlob` Go type, which is a raw `[]byte` at the DB level — Wrap/Unwrap is done in the service layer with the KEK.

### `Issuer`

| Column           | Type           | Notes                                       |
|------------------|----------------|---------------------------------------------|
| `id`             | uuid (PK)      |                                             |
| `name`           | string         | "Lab EU primary"                            |
| `key_id`         | string UNIQUE  | the JSON `kid` field, e.g. `"k2026-05"`     |
| `public_key`     | bytes          | Ed25519 public, 32 bytes                    |
| `encrypted_priv` | bytes          | ChaCha20-Poly1305 wrap of the 64-byte priv  |
| `active`         | bool           | at most one row active                       |
| `created_at`     | time           |                                             |
| `retired_at`     | time?          |                                             |

Edges: `→ has many License`.

### `License`

| Column                | Type                              | Notes                                              |
|-----------------------|-----------------------------------|----------------------------------------------------|
| `id`                  | uuid (PK)                         | internal DB id                                     |
| `license_uuid`        | string UNIQUE                     | the `License.ID` emitted (UUIDv4)                  |
| `issuer_id`           | uuid (FK Issuer)                  |                                                    |
| `subject`             | string (indexed)                  |                                                    |
| `issuer_name`         | string                            | the `iss` claim                                    |
| `audience`            | json `[]string`                   |                                                    |
| `features`            | json `[]string`                   | new in v0.160.0                                    |
| `not_before`          | time                              |                                                    |
| `not_after`           | time                              |                                                    |
| `bindings_meta`       | json                              | typed snapshot for searching/filtering             |
| `payload_kind`        | enum `none|cleartext|sealed`      |                                                    |
| `identity_sha256`     | string? (indexed)                 |                                                    |
| `binary_sha256`       | string?                           |                                                    |
| `pem`                 | bytes                             | canonical artefact, exactly what was shipped       |
| `status`              | enum `active|revoked|expired|superseded` |                                              |
| `replaces_license_id` | uuid? (FK License)                | re-issue chain                                     |
| `created_at`          | time                              |                                                    |

Edges: `→ has many TOTPSecret`, `→ has one Revocation`.

Indexes: `subject`, `status`, `not_after`, `identity_sha256`, `(issuer_id, status)`.

### `Revocation`

| Column        | Type                        | Notes                              |
|---------------|-----------------------------|------------------------------------|
| `id`          | uuid (PK)                   |                                    |
| `license_id`  | uuid (FK License) UNIQUE    | at most one row per License        |
| `reason`      | string                      | free-form admin text               |
| `revoked_at`  | time                        |                                    |
| `revoked_by`  | string                      | operator name from Settings        |

### `Identity`

| Column       | Type             | Notes                                                 |
|--------------|------------------|-------------------------------------------------------|
| `id`         | uuid (PK)        |                                                       |
| `name`       | string UNIQUE    | `"rshell-v1.2"`                                        |
| `bytes`      | bytes            | the 32 random bytes — NOT encrypted (public once shipped) |
| `sha256`     | string (indexed) | hex                                                   |
| `created_at` | time             |                                                       |

### `RecipientKey` (X25519 for sealed payload)

| Column            | Type           | Notes                                |
|-------------------|----------------|--------------------------------------|
| `id`              | uuid (PK)      |                                      |
| `name`            | string UNIQUE  |                                      |
| `public_key`      | bytes          | 32 bytes                             |
| `encrypted_priv`  | bytes          | ChaCha20-Poly1305 wrap               |
| `created_at`      | time           |                                      |

### `TOTPSecret`

| Column              | Type                  | Notes                          |
|---------------------|-----------------------|--------------------------------|
| `id`                | uuid (PK)             |                                |
| `license_id`        | uuid (FK License)     |                                |
| `encrypted_secret`  | bytes                 | base32 secret, encrypted       |
| `account_label`     | string                | for the otpauth:// URI         |
| `created_at`        | time                  |                                |

### `ProbeToken`

| Column          | Type            | Notes                                            |
|-----------------|-----------------|--------------------------------------------------|
| `id`            | string (PK)     | 32 hex chars, URL-safe                           |
| `label`         | string          | `"Alice prod box"`                               |
| `created_at`    | time            |                                                  |
| `expires_at`    | time (indexed)  | default `created_at + 24h`                       |
| `used_at`       | time?           | set when result POSTed                           |
| `remote_addr`   | string?         |                                                  |
| `hostname`      | string?         |                                                  |
| `os`            | string?         |                                                  |
| `arch`          | string?         |                                                  |
| `cpu_brand`     | string?         |                                                  |
| `local_hex`     | string?         | `hostid.Local()` hex                             |
| `composite_hex` | string?         | `hostid.Composite()` hex                         |

### `ServerConfig` (singleton, PK = 1)

| Column                          | Type     | Notes                              |
|---------------------------------|----------|------------------------------------|
| `id`                            | int PK   | constant 1                         |
| `revocation_listen`             | string   | `":8443"`                          |
| `revocation_tls_cert`           | string?  |                                    |
| `revocation_tls_key`            | string?  |                                    |
| `revocation_admin_token_enc`    | bytes    | ChaCha20-Poly1305 wrap             |
| `revocation_path`               | string   | default `/revoked.pem`             |
| `heartbeat_listen`              | string   |                                    |
| `heartbeat_tls_cert`            | string?  |                                    |
| `heartbeat_tls_key`             | string?  |                                    |
| `heartbeat_path`                | string   | default `/heartbeat`               |
| `probe_listen`                  | string   |                                    |
| `probe_tls_cert`                | string?  |                                    |
| `probe_tls_key`                 | string?  |                                    |
| `probe_default_ttl_seconds`     | int      | default 86400                      |

### `Setting` (singleton, PK = 1)

| Column                       | Type           | Notes                            |
|------------------------------|----------------|----------------------------------|
| `id`                         | int PK         | constant 1                       |
| `default_issuer_name`        | string         |                                  |
| `default_audience`           | json `[]string`|                                  |
| `default_ttl_seconds`        | int            | default `30*86400`               |
| `default_argon_preset`       | enum `fast|default|paranoid` |                    |
| `operator_name`              | string         | audit `actor` field              |
| `auto_start_servers`         | bool           | default false                    |
| `confirm_quit_with_servers`  | bool           | default true                     |
| `kek_salt`                   | bytes (16)     | passphrase derivation salt       |
| `kek_canary`                 | bytes          | KEK.Wrap(random32) for verify    |

### `AuditEvent`

| Column        | Type             | Notes                              |
|---------------|------------------|------------------------------------|
| `id`          | uuid (PK)        |                                    |
| `kind`        | string           | `license.issue|license.revoke|issuer.create|...` |
| `target_kind` | string           | `License|Issuer|Identity|...`      |
| `target_id`   | string (indexed) | `-` for global events              |
| `actor`       | string           | operator name                      |
| `payload`     | json             | free-form structured details       |
| `created_at`  | time (indexed)   |                                    |

## 5. Crypto

### `KEK` derivation

- Passphrase resolved via cascade (see §6 bootstrap).
- 16-byte `kek_salt` stored in plaintext in `Setting.kek_salt`.
- `KEK = Argon2id(passphrase, salt, time=3, memory=64*1024, threads=4, keylen=32)`.
- A `KEK.canary = ChaCha20-Poly1305(random32)` is stored in `Setting.kek_canary`. On every boot we try to `Unwrap(canary)` — failure means wrong passphrase, retry up to 3 times, then exit.
- KEK is `Wipe()`-d on clean shutdown.

### Wrap format

`[12-byte random nonce][ciphertext][16-byte AEAD tag]`. The same format applies to every encrypted column.

### Columns encrypted at rest

- `Issuer.encrypted_priv`
- `RecipientKey.encrypted_priv`
- `TOTPSecret.encrypted_secret`
- `ServerConfig.revocation_admin_token_enc`

Everything else (license PEM bodies, subjects, audit events, identities, ProbeToken results) is plaintext in the DB. Rationale: the secrets that enable issuing new licences or computing valid TOTP codes are encrypted; the rest is operational data that can be reconstructed from the shipped licences themselves.

## 6. Bootstrap

```text
1. Parse flags: --db <path>, --passphrase-file <path>, --no-tui
2. Resolve passphrase via cascade:
   a. --passphrase-file <path>      → read + trim
   b. MALDEV_MGR_PASSPHRASE_FILE    → read + trim
   c. MALDEV_MGR_PASSPHRASE         → env var
   d. (v2) OS keystore lookup
   e. fallback: prompt the TUI passphrase modal
3. DB exists?
   YES → check passphrase:
     - read Setting.kek_salt
     - KEK = DeriveFromPassphrase(passphrase, salt)
     - tentative KEK.Unwrap(Setting.kek_canary)
     - failure → passphrase wrong, retry (3 max, then exit)
   NO  → first-launch wizard:
     - prompt passphrase + confirmation
     - GenerateSalt() → store as Setting.kek_salt
     - KEK = Derive(passphrase, salt)
     - kek_canary = KEK.Wrap(crypto/rand 32 bytes) → store
     - prompt for first Issuer (name + KeyID)
     - IssuerService.Generate(...) (active=true)
     - default Setting row populated
4. Construct Store + Services
5. If Setting.auto_start_servers → start configured HTTP servers (those with non-empty listen addr)
6. Launch TUI with *Services + httpsrv.Bundle
7. On exit:
   - if any server running AND Setting.confirm_quit_with_servers → confirm
   - Bundle.StopAll(10s)
   - Services.Close() → KEK.Wipe(), Store.Close()
```

The passphrase prompt is hidden when steps 2a-2d resolved it — no friction for scripted / keystore-driven flows.

## 7. Services

### `*service.Services` bundle

Single struct passed everywhere. Owns the `*store.Store`, `*crypto.KEK`, optional `*httpsrv.Bundle` (attached after instantiation since servers depend on Settings being readable first), and one of each service type.

```go
type Services struct {
    Store    *store.Store
    KEK      *crypto.KEK
    Servers  *httpsrv.Bundle

    Issuer    *IssuerService
    License   *LicenseService
    Revoke    *RevokeService
    Identity  *IdentityService
    Recipient *RecipientService
    TOTP      *TOTPService
    Probe     *ProbeService
    Audit     *AuditService
    Settings  *SettingsService
}

func New(store *store.Store, kek *crypto.KEK) *Services
func (s *Services) AttachServers(b *httpsrv.Bundle)
func (s *Services) Close() error
```

### `IssuerService`

```go
Generate(ctx, name, keyID string) (*ent.Issuer, error)
Import(ctx, name, keyID string, privatePEM []byte) (*ent.Issuer, error)
SetActive(ctx, id uuid.UUID) error
List(ctx) ([]*ent.Issuer, error)
Get(ctx, id uuid.UUID) (*ent.Issuer, error)
Active(ctx) (*ent.Issuer, error)
ExportPublic(ctx, id uuid.UUID) ([]byte, error)
ExportPrivate(ctx, id uuid.UUID, confirmPassphrase string) ([]byte, error)
Retire(ctx, id uuid.UUID) error
Delete(ctx, id uuid.UUID) error   // refuses if # licences signed > 0
```

### `LicenseService`

```go
type IssueRequest struct {
    IssuerID     uuid.UUID
    Subject      string
    AudienceList []string
    NotBefore    time.Time
    NotAfter     time.Time
    Bindings     []BindingSpec
    Features     []string
    IdentityID   *uuid.UUID
    BinarySHA256 string
    Payload      json.RawMessage
    SealedFor    *uuid.UUID
    SealedPlain  []byte
    Label        string
    ReplacesID   *uuid.UUID
}

type BindingSpec struct {
    Type   string
    Values []string
    Argon  *license.BindingParams
}

type IssuedLicense struct {
    Row   *ent.License
    PEM   []byte
    TOTPs []TOTPProvisioning
}

type TOTPProvisioning struct {
    Secret       string
    OtpauthURI   string
    QRImageASCII string
    QRImagePNG   []byte
}

Issue(ctx, IssueRequest) (*IssuedLicense, error)
ReIssue(ctx, originalID uuid.UUID, opts ReIssueOptions) (*IssuedLicense, error)
List(ctx, filter ListFilter) ([]*ent.License, error)
Get(ctx, id uuid.UUID) (*LicenseDetail, error)
GetByUUID(ctx, uuid string) (*ent.License, error)
Inspect(pem []byte) (*license.License, error)
Import(ctx, pem []byte, label string) (*ent.License, error)
ExportPEM(ctx, id uuid.UUID) ([]byte, error)
ExportBatch(ctx, ids []uuid.UUID) ([]byte, error)
HashFile(ctx, path string, progress func(read, total int64)) (string, error)
```

`ListFilter` covers status, expiry range, key_id, audience contains, features contains, subject search.

`LicenseDetail` aggregates the row + decoded bindings + decoded payload (and a `Sealed bool` flag if `SealedPayload` is present, with optional `RecipientKeyName` for display) + TOTPs + revocation if present + parent + successors.

### `RevokeService`

```go
Revoke(ctx, licenseID uuid.UUID, reason string) error
Unrevoke(ctx, licenseID uuid.UUID) error
ListRevoked(ctx) ([]RevocationView, error)
PublishSignedList(ctx) ([]byte, error)
```

The revocation HTTP server calls `PublishSignedList` on every GET. `PublishSignedList` reads the current `Revocation` rows, builds a `revoke.List`, signs with the active issuer, returns PEM.

### `IdentityService`

```go
Create(ctx, name string) (*ent.Identity, error)
Import(ctx, name string, bytes []byte) (*ent.Identity, error)
List(ctx) ([]*ent.Identity, error)
ExportBin(ctx, id uuid.UUID) ([]byte, error)
Regenerate(ctx, id uuid.UUID, confirmed bool) error
Delete(ctx, id uuid.UUID) error   // refuses if usage_count > 0
UsageCount(ctx, id uuid.UUID) (int, error)
```

### `RecipientService`

```go
Generate(ctx, name string) (*ent.RecipientKey, error)
Import(ctx, name string, pubKey, privKey []byte) (*ent.RecipientKey, error)
List(ctx) ([]*ent.RecipientKey, error)
ExportPublic(ctx, id uuid.UUID) ([]byte, error)
ExportPrivate(ctx, id uuid.UUID, confirmPassphrase string) ([]byte, error)
Delete(ctx, id uuid.UUID) error
```

### `TOTPService`

```go
Get(ctx, licenseID uuid.UUID) (*TOTPSecretView, error)
PrintQRASCII(ctx, licenseID uuid.UUID) (string, error)
ExportQRPNG(ctx, licenseID uuid.UUID, path string) error
```

The secret is generated by `LicenseService.Issue` when there is a TOTP binding — `TOTPService` only exposes read paths.

### `ProbeService`

```go
NewToken(ctx, label string, ttl time.Duration) (*ent.ProbeToken, error)
Subscribe(token string) <-chan *ent.ProbeToken
ConsumeToken(ctx, token string, result AgentResult, remoteAddr string) error
History(ctx, limit int) ([]*ent.ProbeToken, error)
Revoke(ctx, tokenID string) error
```

Subscribers are kept in an in-memory `sync.Map[token]chan` inside the service. `ConsumeToken` writes the result to the DB **and** sends on the subscriber channel if present. The HTTP probe server calls `ConsumeToken` from its POST handler.

### `AuditService`

```go
Append(ctx, kind string, target Target, payload any) error
List(ctx, filter AuditFilter, limit int) ([]*ent.AuditEvent, error)
ListForTarget(ctx, target Target, limit int) ([]*ent.AuditEvent, error)
Export(ctx, format string, w io.Writer) error   // "csv" | "json"
```

Every mutating service call appends an `AuditEvent` within the same transaction.

### `SettingsService`

```go
Get(ctx) (*ent.Setting, error)
Update(ctx, mut func(*ent.SettingUpdate)) (*ent.Setting, error)
GetServerConfig(ctx) (*ent.ServerConfig, error)
UpdateServerConfig(ctx, mut func(*ent.ServerConfigUpdate)) (*ent.ServerConfig, error)
ChangePassphrase(ctx, oldPassphrase, newPassphrase string) error
                                  // re-derive KEK, re-wrap every encrypted column in a single tx
```

## 8. HTTP servers

All three implement `httpsrv.Server`:

```go
type Server interface {
    Name() string
    Start(ctx context.Context) error
    Stop(timeout time.Duration) error
    Status() Status
    Events() <-chan Event
}

type Status struct {
    Running    bool
    ListenAddr string
    StartedAt  time.Time
    Requests   uint64
    LastReq    time.Time
    LastError  string
}

type Event struct {
    At     time.Time
    Server string
    Kind   string
    Method string
    Path   string
    Status int
    Remote string
    Note   string
}
```

`Events()` returns a 256-buffered channel with drop-oldest on overflow.

### `httpsrv.RevocationServer`

Wraps `license/server.NewRevocationHandler` with options pulled from `ServerConfig`. Admin token decrypted with KEK at start. Token rotation = stop + persist new encrypted token + start.

### `httpsrv.HeartbeatServer`

Wraps `license/server.NewHeartbeatHandler` with a `heartbeatLicenseStore` adapter that calls `LicenseService.GetByUUID` and maps `License.status → server.LicenseStatus`.

### `httpsrv.ProbeServer`

New. Routes:

```text
GET  /probe/<token>/agent[/<os-arch>]    serves the embedded probe binary
GET  /probe/<token>/snippet              copy-paste curl/PowerShell one-liner
POST /probe/<token>/result               receives the AgentResult JSON
```

POST handler calls `ProbeService.ConsumeToken(ctx, token, payload, remoteAddr)` which writes to DB and notifies the subscriber channel.

When no `os-arch` is supplied to `/agent`, the server best-effort detects from the User-Agent (`Linux`, `Darwin`, `Windows`) and the `Sec-CH-UA-Arch` header if present. Falls back to a 400 with the list of available targets.

### `httpsrv.Bundle`

```go
type Bundle struct {
    Revocation *RevocationServer
    Heartbeat  *HeartbeatServer
    Probe      *ProbeServer
}

func NewBundle(svc *service.Services) *Bundle
func (b *Bundle) MergedEvents() <-chan Event
func (b *Bundle) StopAll(timeout time.Duration) error
```

`MergedEvents()` fan-ins the three `Events()` channels into one for the TUI to consume via a single `tea.Cmd`.

## 9. Embedded probe binary

```
internal/manager/probe/
  agents/
    linux-amd64
    linux-arm64
    darwin-amd64
    darwin-arm64
    windows-amd64.exe
    gen/main.go        -- ~80 LoC, imports license/hostid, POSTs JSON to arg1
    gen.go             -- go:generate directives building the 5 binaries
  embed.go             -- //go:embed agents/* var agents embed.FS
                          + ServeAgent(w, osArch string) error
  types.go             -- AgentResult
```

Build is driven by `go generate ./internal/manager/probe/...`, which runs:

```bash
GOOS=linux   GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o agents/linux-amd64        ./gen
GOOS=linux   GOARCH=arm64 go build -trimpath -ldflags "-s -w" -o agents/linux-arm64        ./gen
GOOS=darwin  GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o agents/darwin-amd64       ./gen
GOOS=darwin  GOARCH=arm64 go build -trimpath -ldflags "-s -w" -o agents/darwin-arm64       ./gen
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o agents/windows-amd64.exe  ./gen
```

The agent itself:

```go
// agents/gen/main.go
package main

import (
    "bytes"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "net/http"
    "os"
    "runtime"
    "time"

    "github.com/oioio-space/maldev/license/hostid"
)

type AgentResult struct {
    Hostname     string `json:"hostname"`
    OS           string `json:"os"`
    Arch         string `json:"arch"`
    LocalHex     string `json:"local_hex"`
    CompositeHex string `json:"composite_hex"`
    SentAt       string `json:"sent_at"`
}

func main() {
    if len(os.Args) != 2 {
        fmt.Fprintln(os.Stderr, "usage: probe <result-url>")
        os.Exit(2)
    }
    local, _ := hostid.Local()
    composite, _ := hostid.Composite()
    host, _ := os.Hostname()
    body, _ := json.Marshal(AgentResult{
        Hostname:     host,
        OS:           runtime.GOOS,
        Arch:         runtime.GOARCH,
        LocalHex:     hex.EncodeToString(local),
        CompositeHex: hex.EncodeToString(composite),
        SentAt:       time.Now().UTC().Format(time.RFC3339),
    })
    client := &http.Client{Timeout: 10 * time.Second}
    resp, err := client.Post(os.Args[1], "application/json", bytes.NewReader(body))
    if err != nil {
        fmt.Fprintf(os.Stderr, "probe: %v\n", err)
        os.Exit(1)
    }
    defer resp.Body.Close()
    if resp.StatusCode/100 != 2 {
        fmt.Fprintf(os.Stderr, "probe: server returned %s\n", resp.Status)
        os.Exit(1)
    }
    fmt.Println("OK")
}
```

Operator-facing snippet, displayed in the wizard overlay and on the probe-server screen:

```bash
# Linux/macOS
URL="https://<manager>:<port>/probe/<token>"
curl -fsSL "$URL/agent/$(uname -s | tr A-Z a-z)-$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')" \
  -o /tmp/maldev-probe && chmod +x /tmp/maldev-probe \
  && /tmp/maldev-probe "$URL/result"

# Windows PowerShell
$URL = "https://<manager>:<port>/probe/<token>"
Invoke-WebRequest "$URL/agent/windows-amd64" -OutFile $env:TEMP\maldev-probe.exe
& "$env:TEMP\maldev-probe.exe" "$URL/result"
```

## 10. Atomicity

Every mutating service method:

1. Opens an `*ent.Tx` from the `Store`.
2. Performs all DB writes (business row + audit event).
3. Commits.
4. Returns the new row(s) plus any side-effect artefacts (PEM bytes, TOTP provisioning info).

Composite operations (e.g. rotation, change passphrase) accept an external `*ent.Tx` to compose.

## 11. Testing

| Layer        | Approach                                                                 |
|--------------|--------------------------------------------------------------------------|
| `crypto`     | KDF determinism, ChaCha20-Poly1305 roundtrip, canary detect              |
| `store`      | In-memory SQLite (`file::memory:?cache=shared`), schema migrations, encrypted-blob roundtrip |
| `service`    | One file per service; build `*Services` on a temp DB, exercise public surface, assert DB + audit |
| `httpsrv`    | `httptest.Server` per server; assert Events emitted, POST handlers correct |
| `probe`      | Build a no-op agent binary in `init()` of test, POST as a real client    |
| Integration  | `internal/manager/manager_test.go`: boot full Services + Bundle, generate keypair, issue licence with TOTP + machine binding, start revocation server, GET CRL, revoke, GET CRL again, verify license is now rejected by `license.Verify` |

## 12. Out-of-scope v1

- OS keystore for passphrase
- Multi-operator
- Floating seats
- Push webhooks
- License-chain graph viz
- Bulk operations CLI
- Encrypted backup export
- Sync between manager instances

## 13. Documentation

- `internal/manager/doc.go` — short package overview
- `docs/license-manager/concepts.md` — operator-facing primer (separate from `docs/license/` which is the library docs)
- `docs/license-manager/workflow.md` — cookbook of operational flows (issue, rotate, revoke, probe)
- `README.md` — add a row in the Packages table once the TUI lands
- `docs/mitre.md` — N/A (defensive primitive)

The TUI design + handoff doc come from a separate Claude Design session; they will populate `docs/license-manager/tui-design.md` once received.

## 14. Adjustments forwarded to Claude Design

Captured at end of brainstorming. These are integrated into the Claude Design prompt:

1. Passphrase resolution cascade — when resolved silently, no TUI prompt
2. Re-issue with parenthood — banner + successors panel in detail view
3. Probe binary served per OS/arch via URL — 3-tab snippet display
4. Probe subscriber via channel for real-time results
5. New settings: `default_argon_preset`, `auto_start_servers`, `confirm_quit_with_servers`
6. IdentityService.Delete refuses when used; show usage count
7. Per-target audit log tab on every detail view
8. Probe binary embedded via `//go:embed`, no runtime build
9. No mDNS / UPnP — operator types URLs
10. Admin token rotation triggers server restart with confirmation
