# License Manager — Backend Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver the backend layer described in `docs/superpowers/specs/2026-05-20-license-manager-backend-design.md`: `internal/manager/` packages (crypto, store with ENT, services, httpsrv, probe) + `cmd/license-manager/` boot loader, all pure-Go, with column-level encryption-at-rest of secrets and three independently-controllable HTTP servers (revocation, heartbeat, fingerprint-probe).

**Architecture:** Layered — `store` (ENT/SQLite, pure Go) → `service` (business logic + audit + tx) → `httpsrv` (HTTP adapters with Server interface, Events channel) ; plus `crypto` for passphrase-derived KEK and `probe` for embedded per-OS agent binaries. The TUI layer is out-of-scope of this plan and consumes `*service.Services` from `cmd/license-manager/main.go`.

**Tech Stack:** Go 1.22+, `entgo.io/ent`, `modernc.org/sqlite` (no cgo), `golang.org/x/crypto/{argon2,hkdf,chacha20poly1305}`, `github.com/oioio-space/maldev/license/*` (in-repo).

**Commit conventions:** conventional commits, `/simplify` per CLAUDE.md before each Go commit (the simplify skill is invoked at the END of each task once tests are green), tech-md updates in the same commit as new API surface.

---

## File structure

```
cmd/license-manager/
  main.go                  -- boot, passphrase resolution, services init, no TUI yet (smoke)
  flags.go                 -- CLI flag definitions

internal/manager/
  doc.go
  crypto/
    kek.go                 -- DeriveFromPassphrase + Wipe
    cipher.go              -- Wrap/Unwrap (ChaCha20-Poly1305)
    types.go               -- EncryptedBlob, Canary helpers
    kek_test.go
    cipher_test.go
  store/
    schema/                -- ENT entities, one file per entity
      issuer.go
      license.go
      revocation.go
      identity.go
      recipient_key.go
      totp_secret.go
      probe_token.go
      server_config.go
      setting.go
      audit_event.go
    store.go               -- New(path) (*Store, error), wraps *ent.Client
    migrate.go             -- AutoMigrate on open
    store_test.go
  service/
    audit.go               -- AuditService
    settings.go            -- SettingsService
    issuer.go              -- IssuerService
    identity.go            -- IdentityService
    recipient.go           -- RecipientService
    totp.go                -- TOTPService
    probe.go               -- ProbeService (token mgmt + Subscribe channels)
    license.go             -- LicenseService
    revoke.go              -- RevokeService
    services.go            -- *Services bundle
    *_test.go              -- one per service file
  httpsrv/
    server.go              -- Server interface, Status, Event
    revocation.go
    heartbeat.go
    probe.go
    bundle.go
    httpsrv_test.go
  probe/
    agents/
      gen/
        main.go            -- the agent itself
        go.mod             -- module isolation for the agent build
      gen.go               -- //go:generate directives
    embed.go               -- //go:embed agents/* + ServeAgent
    types.go               -- AgentResult JSON shape

docs/license-manager/
  concepts.md
  workflow.md
.dev/license-manager-2026/
  progress.md
  backlog.md
```

---

## Task 1 — Module scaffold + dependencies

**Files:**
- Modify: `go.mod`, `go.sum`
- Create: `internal/manager/doc.go`, `cmd/license-manager/main.go` (stub), `cmd/license-manager/flags.go` (stub)

- [ ] **Step 1: Add dependencies**

```bash
go get entgo.io/ent@v0.13.1
go get modernc.org/sqlite@v1.34.4
go get github.com/google/uuid@v1.6.0
```

- [ ] **Step 2: Create `internal/manager/doc.go`**

```go
// Package manager houses the local backend for the license-manager TUI.
//
// Layers:
//
//   crypto/   - passphrase-derived KEK + ChaCha20-Poly1305 wrap/unwrap
//   store/    - ENT-backed SQLite store
//   service/  - domain services orchestrating store + audit + tx
//   httpsrv/  - lifecycle-managed HTTP servers (revocation / heartbeat / probe)
//   probe/    - embedded per-OS agent binaries for remote fingerprinting
//
// The TUI layer (cmd/license-manager) consumes *service.Services and never
// touches store or crypto directly.
package manager
```

- [ ] **Step 3: Create stub `cmd/license-manager/main.go`**

```go
package main

import (
	"fmt"
	"os"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "license-manager:", err)
		os.Exit(1)
	}
}

func run() error {
	// Wired in Task 22.
	fmt.Println("license-manager: stub — wiring lands in Task 22")
	return nil
}
```

- [ ] **Step 4: Create stub `cmd/license-manager/flags.go`**

```go
package main

import "flag"

type cliFlags struct {
	DBPath         string
	PassphraseFile string
	NoTUI          bool
}

func parseFlags() cliFlags {
	var f cliFlags
	flag.StringVar(&f.DBPath, "db", "manager.db", "path to the SQLite store")
	flag.StringVar(&f.PassphraseFile, "passphrase-file", "", "file containing the passphrase")
	flag.BoolVar(&f.NoTUI, "no-tui", false, "boot without launching the TUI (smoke test)")
	flag.Parse()
	return f
}
```

- [ ] **Step 5: Build + commit**

```bash
go build ./...
git add go.mod go.sum internal/manager/doc.go cmd/license-manager/
git commit -m "feat(manager): scaffold package + cmd entry point + deps"
```

---

## Task 2 — `crypto/` — KEK derivation

**Files:**
- Create: `internal/manager/crypto/kek.go`, `internal/manager/crypto/kek_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/manager/crypto/kek_test.go
package crypto

import (
	"bytes"
	"testing"
)

func TestDeriveDeterministic(t *testing.T) {
	salt := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	a := DeriveFromPassphrase("hunter2", salt)
	b := DeriveFromPassphrase("hunter2", salt)
	if !bytes.Equal(a.key[:], b.key[:]) {
		t.Fatal("KEK derivation not deterministic")
	}
}

func TestDeriveDifferentSalt(t *testing.T) {
	a := DeriveFromPassphrase("hunter2", [16]byte{1})
	b := DeriveFromPassphrase("hunter2", [16]byte{2})
	if bytes.Equal(a.key[:], b.key[:]) {
		t.Fatal("different salt should give different key")
	}
}

func TestWipeZeroes(t *testing.T) {
	k := DeriveFromPassphrase("p", [16]byte{0})
	k.Wipe()
	for i, b := range k.key {
		if b != 0 {
			t.Fatalf("key[%d]=%d after Wipe", i, b)
		}
	}
}

func TestGenerateSalt(t *testing.T) {
	a, err := GenerateSalt()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := GenerateSalt()
	if a == b {
		t.Fatal("salt collision (probability ~ 2^-128)")
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/manager/crypto/...`
Expected: FAIL — undefined symbols.

- [ ] **Step 3: Implement `kek.go`**

```go
// Package crypto provides passphrase-derived key wrapping for the license
// manager's at-rest secrets. KEK = Argon2id(passphrase, salt). Wrap/Unwrap
// uses ChaCha20-Poly1305 with a random 12-byte nonce per blob.
package crypto

import (
	"crypto/rand"

	"golang.org/x/crypto/argon2"
)

const (
	argonTime    uint32 = 3
	argonMemory  uint32 = 64 * 1024
	argonThreads uint8  = 4
	keyLen       uint32 = 32
)

// KEK is the symmetric key derived from the operator's passphrase. Never
// persist this value — only the wrapped blobs and the salt land on disk.
type KEK struct {
	key [32]byte
}

// DeriveFromPassphrase computes Argon2id(passphrase, salt). Same passphrase
// + same salt always yields the same KEK, so an existing DB can be reopened.
func DeriveFromPassphrase(passphrase string, salt [16]byte) *KEK {
	out := argon2.IDKey([]byte(passphrase), salt[:], argonTime, argonMemory, argonThreads, keyLen)
	var k KEK
	copy(k.key[:], out)
	for i := range out {
		out[i] = 0
	}
	return &k
}

// Wipe zeroes the key bytes. Call on clean shutdown so a memory snapshot
// after exit doesn't reveal the KEK.
func (k *KEK) Wipe() {
	for i := range k.key {
		k.key[i] = 0
	}
}

// GenerateSalt returns a fresh 16-byte random salt for a new DB.
func GenerateSalt() ([16]byte, error) {
	var s [16]byte
	if _, err := rand.Read(s[:]); err != nil {
		return s, err
	}
	return s, nil
}
```

- [ ] **Step 4: Tests pass**

Run: `go test ./internal/manager/crypto/... -v`
Expected: 4 PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/manager/crypto/kek.go internal/manager/crypto/kek_test.go
git commit -m "feat(manager/crypto): Argon2id-derived KEK + Wipe + GenerateSalt"
```

---

## Task 3 — `crypto/` — ChaCha20-Poly1305 Wrap/Unwrap + Canary

**Files:**
- Create: `internal/manager/crypto/cipher.go`, `internal/manager/crypto/types.go`, `internal/manager/crypto/cipher_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/manager/crypto/cipher_test.go
package crypto

import (
	"bytes"
	"errors"
	"testing"
)

func TestWrapUnwrapRoundTrip(t *testing.T) {
	k := DeriveFromPassphrase("p", [16]byte{1})
	plain := []byte("classified config")
	w, err := k.Wrap(plain)
	if err != nil {
		t.Fatal(err)
	}
	got, err := k.Unwrap(w)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatalf("roundtrip mismatch")
	}
}

func TestUnwrapRejectsTampered(t *testing.T) {
	k := DeriveFromPassphrase("p", [16]byte{1})
	w, _ := k.Wrap([]byte("x"))
	w[len(w)-1] ^= 0x01
	if _, err := k.Unwrap(w); err == nil {
		t.Fatal("tampered ciphertext accepted")
	}
}

func TestUnwrapRejectsWrongKey(t *testing.T) {
	a := DeriveFromPassphrase("p1", [16]byte{1})
	b := DeriveFromPassphrase("p2", [16]byte{1})
	w, _ := a.Wrap([]byte("x"))
	if _, err := b.Unwrap(w); err == nil {
		t.Fatal("wrong KEK accepted")
	}
}

func TestCanaryDetectsWrongPassphrase(t *testing.T) {
	good := DeriveFromPassphrase("p1", [16]byte{1})
	canary, err := NewCanary(good)
	if err != nil {
		t.Fatal(err)
	}
	if !good.VerifyCanary(canary) {
		t.Fatal("good KEK rejected its own canary")
	}
	bad := DeriveFromPassphrase("p2", [16]byte{1})
	if bad.VerifyCanary(canary) {
		t.Fatal("wrong KEK passed canary check")
	}
}

func TestErrIsExported(t *testing.T) {
	k := DeriveFromPassphrase("p", [16]byte{1})
	_, err := k.Unwrap([]byte{1, 2, 3})
	if !errors.Is(err, ErrWrappedFormat) {
		t.Fatalf("err=%v want ErrWrappedFormat", err)
	}
}
```

- [ ] **Step 2: Tests fail**

Run: `go test ./internal/manager/crypto/... -v`
Expected: FAIL — undefined Wrap, Unwrap, NewCanary, VerifyCanary, ErrWrappedFormat.

- [ ] **Step 3: Implement `cipher.go`**

```go
package crypto

import (
	"crypto/rand"
	"errors"
	"fmt"

	"golang.org/x/crypto/chacha20poly1305"
)

// ErrWrappedFormat is returned when Unwrap receives bytes that aren't a
// well-formed wrap: [12-byte nonce][ciphertext][16-byte tag].
var ErrWrappedFormat = errors.New("crypto: wrapped blob has bad length")

const nonceLen = 12

// Wrap encrypts plain under the KEK with a fresh random nonce. Format on
// disk is nonce || ciphertext || tag.
func (k *KEK) Wrap(plain []byte) ([]byte, error) {
	aead, err := chacha20poly1305.New(k.key[:])
	if err != nil {
		return nil, fmt.Errorf("crypto: build aead: %w", err)
	}
	nonce := make([]byte, nonceLen)
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	out := make([]byte, 0, nonceLen+len(plain)+aead.Overhead())
	out = append(out, nonce...)
	return aead.Seal(out, nonce, plain, nil), nil
}

// Unwrap reverses Wrap. Returns ErrWrappedFormat for blobs that are too
// short to even contain a nonce + tag.
func (k *KEK) Unwrap(wrapped []byte) ([]byte, error) {
	aead, err := chacha20poly1305.New(k.key[:])
	if err != nil {
		return nil, err
	}
	if len(wrapped) < nonceLen+aead.Overhead() {
		return nil, ErrWrappedFormat
	}
	nonce := wrapped[:nonceLen]
	ct := wrapped[nonceLen:]
	return aead.Open(nil, nonce, ct, nil)
}

// NewCanary wraps a 32-byte random payload under the KEK. Store it once at
// DB creation; verify with KEK.VerifyCanary on every boot.
func NewCanary(k *KEK) ([]byte, error) {
	plain := make([]byte, 32)
	if _, err := rand.Read(plain); err != nil {
		return nil, err
	}
	return k.Wrap(plain)
}

// VerifyCanary returns true iff the KEK can decrypt the canary. A false
// result is the definitive "wrong passphrase" signal at boot.
func (k *KEK) VerifyCanary(canary []byte) bool {
	_, err := k.Unwrap(canary)
	return err == nil
}
```

- [ ] **Step 4: Implement `types.go`**

```go
package crypto

// EncryptedBlob is the type alias used by ENT schemas for columns that hold
// wrapped secrets. It is functionally a []byte but the type makes the intent
// explicit at the schema layer.
type EncryptedBlob []byte
```

- [ ] **Step 5: Tests pass**

Run: `go test ./internal/manager/crypto/... -v`
Expected: all PASS (4 from Task 2 + 5 new).

- [ ] **Step 6: Commit**

```bash
git add internal/manager/crypto/cipher.go internal/manager/crypto/types.go internal/manager/crypto/cipher_test.go
git commit -m "feat(manager/crypto): Wrap/Unwrap + Canary + EncryptedBlob"
```

---

## Task 4 — ENT schema definitions (10 entities)

**Files:**
- Create: `internal/manager/store/schema/{issuer,license,revocation,identity,recipient_key,totp_secret,probe_token,server_config,setting,audit_event}.go`
- Create: `internal/manager/store/schema/generate.go` (go:generate directive)

This task lays out the entity skeletons; ENT code generation happens in Task 5.

- [ ] **Step 1: Create `internal/manager/store/schema/generate.go`**

```go
package schema

//go:generate go run -mod=mod entgo.io/ent/cmd/ent generate ./
```

- [ ] **Step 2: Create `issuer.go`**

```go
package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"

	"github.com/google/uuid"
)

type Issuer struct{ ent.Schema }

func (Issuer) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New),
		field.String("name").NotEmpty(),
		field.String("key_id").Unique().NotEmpty(),
		field.Bytes("public_key").MaxLen(32).MinLen(32),
		field.Bytes("encrypted_priv"),
		field.Bool("active").Default(false),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("retired_at").Optional().Nillable(),
	}
}

func (Issuer) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("licenses", License.Type),
	}
}

func (Issuer) Indexes() []ent.Index { return nil }
func (Issuer) Annotations() []schema.Annotation { return nil }
```

Wait — ENT has a circular reference if we reference `License.Type` from `Issuer` and vice versa. The pattern in ENT is fine because schema files just declare; the generated code resolves cross-references. Continue.

- [ ] **Step 3: Create `license.go`**

```go
package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/google/uuid"
)

type License struct{ ent.Schema }

func (License) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New),
		field.String("license_uuid").Unique().NotEmpty(),
		field.String("subject").NotEmpty(),
		field.String("issuer_name").Optional(),
		field.JSON("audience", []string{}).Optional(),
		field.JSON("features", []string{}).Optional(),
		field.Time("not_before"),
		field.Time("not_after"),
		field.JSON("bindings_meta", map[string]any{}).Optional(),
		field.Enum("payload_kind").Values("none", "cleartext", "sealed").Default("none"),
		field.String("identity_sha256").Optional(),
		field.String("binary_sha256").Optional(),
		field.Bytes("pem"),
		field.Enum("status").Values("active", "revoked", "expired", "superseded").Default("active"),
		field.UUID("replaces_license_id", uuid.UUID{}).Optional().Nillable(),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

func (License) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("issuer", Issuer.Type).Ref("licenses").Unique().Required(),
		edge.To("totps", TOTPSecret.Type),
		edge.To("revocation", Revocation.Type).Unique(),
	}
}

func (License) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("subject"),
		index.Fields("status"),
		index.Fields("not_after"),
		index.Fields("identity_sha256"),
	}
}
```

- [ ] **Step 4: Create `revocation.go`**

```go
package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"

	"github.com/google/uuid"
)

type Revocation struct{ ent.Schema }

func (Revocation) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New),
		field.String("reason"),
		field.Time("revoked_at").Default(time.Now).Immutable(),
		field.String("revoked_by"),
	}
}

func (Revocation) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("license", License.Type).Ref("revocation").Unique().Required(),
	}
}
```

- [ ] **Step 5: Create `identity.go`**

```go
package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/google/uuid"
)

type Identity struct{ ent.Schema }

func (Identity) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New),
		field.String("name").Unique().NotEmpty(),
		field.Bytes("bytes").MaxLen(32).MinLen(32),
		field.String("sha256").NotEmpty(),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

func (Identity) Indexes() []ent.Index {
	return []ent.Index{index.Fields("sha256")}
}
```

- [ ] **Step 6: Create `recipient_key.go`**

```go
package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"

	"github.com/google/uuid"
)

type RecipientKey struct{ ent.Schema }

func (RecipientKey) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New),
		field.String("name").Unique().NotEmpty(),
		field.Bytes("public_key").MaxLen(32).MinLen(32),
		field.Bytes("encrypted_priv"),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}
```

- [ ] **Step 7: Create `totp_secret.go`**

```go
package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"

	"github.com/google/uuid"
)

type TOTPSecret struct{ ent.Schema }

func (TOTPSecret) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New),
		field.Bytes("encrypted_secret"),
		field.String("account_label"),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

func (TOTPSecret) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("license", License.Type).Ref("totps").Unique().Required(),
	}
}
```

- [ ] **Step 8: Create `probe_token.go`**

```go
package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type ProbeToken struct{ ent.Schema }

func (ProbeToken) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").Unique().NotEmpty(), // 32-hex token
		field.String("label").Optional(),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("expires_at"),
		field.Time("used_at").Optional().Nillable(),
		field.String("remote_addr").Optional(),
		field.String("hostname").Optional(),
		field.String("os").Optional(),
		field.String("arch").Optional(),
		field.String("cpu_brand").Optional(),
		field.String("local_hex").Optional(),
		field.String("composite_hex").Optional(),
	}
}

func (ProbeToken) Indexes() []ent.Index {
	return []ent.Index{index.Fields("expires_at")}
}
```

- [ ] **Step 9: Create `server_config.go`**

```go
package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

type ServerConfig struct{ ent.Schema }

func (ServerConfig) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").Unique().Immutable(),
		field.String("revocation_listen").Default(":8443"),
		field.String("revocation_tls_cert").Optional(),
		field.String("revocation_tls_key").Optional(),
		field.Bytes("revocation_admin_token_enc").Optional(),
		field.String("revocation_path").Default("/revoked.pem"),
		field.String("heartbeat_listen").Default(":8444"),
		field.String("heartbeat_tls_cert").Optional(),
		field.String("heartbeat_tls_key").Optional(),
		field.String("heartbeat_path").Default("/heartbeat"),
		field.String("probe_listen").Default(":8445"),
		field.String("probe_tls_cert").Optional(),
		field.String("probe_tls_key").Optional(),
		field.Int("probe_default_ttl_seconds").Default(86400),
	}
}
```

- [ ] **Step 10: Create `setting.go`**

```go
package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

type Setting struct{ ent.Schema }

func (Setting) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").Unique().Immutable(),
		field.String("default_issuer_name").Optional(),
		field.JSON("default_audience", []string{}).Optional(),
		field.Int("default_ttl_seconds").Default(30 * 86400),
		field.Enum("default_argon_preset").Values("fast", "default", "paranoid").Default("default"),
		field.String("operator_name").Optional(),
		field.Bool("auto_start_servers").Default(false),
		field.Bool("confirm_quit_with_servers").Default(true),
		field.Bytes("kek_salt"),
		field.Bytes("kek_canary"),
	}
}
```

- [ ] **Step 11: Create `audit_event.go`**

```go
package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/google/uuid"
)

type AuditEvent struct{ ent.Schema }

func (AuditEvent) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New),
		field.String("kind"),
		field.String("target_kind"),
		field.String("target_id"),
		field.String("actor"),
		field.JSON("payload", map[string]any{}).Optional(),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

func (AuditEvent) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("created_at"),
		index.Fields("target_id"),
		index.Fields("kind"),
	}
}
```

- [ ] **Step 12: Commit (schemas only — generation in Task 5)**

```bash
git add internal/manager/store/schema/
git commit -m "feat(manager/store): ENT schemas for 10 entities"
```

---

## Task 5 — ENT code generation + Store wrapper

**Files:**
- Generate: `internal/manager/store/ent/*` (auto-generated, do not edit)
- Create: `internal/manager/store/store.go`, `internal/manager/store/migrate.go`, `internal/manager/store/store_test.go`

- [ ] **Step 1: Generate ENT code**

```bash
cd internal/manager/store/schema
go generate ./...
cd ../../../..
```

This produces `internal/manager/store/ent/` with one file per entity + a client. Verify the directory was created.

- [ ] **Step 2: Implement `store.go`**

```go
// Package store wraps the ENT-generated client with a constructor that opens
// a SQLite file via the pure-Go modernc.org/sqlite driver.
package store

import (
	"context"
	"fmt"

	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/sql"

	_ "modernc.org/sqlite"

	"github.com/oioio-space/maldev/internal/manager/store/ent"
)

// Store wraps the ENT client.
type Store struct {
	*ent.Client
}

// New opens (or creates) a SQLite store at path and runs schema migrations.
// Pass ":memory:" or "file::memory:?cache=shared" for an in-memory store
// (used by tests).
func New(ctx context.Context, path string) (*Store, error) {
	dsn := "file:" + path + "?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
	drv, err := sql.Open(dialect.SQLite, dsn)
	if err != nil {
		return nil, fmt.Errorf("store: open %s: %w", path, err)
	}
	client := ent.NewClient(ent.Driver(drv))
	if err := client.Schema.Create(ctx); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("store: migrate: %w", err)
	}
	return &Store{Client: client}, nil
}

// Close closes the underlying ENT client (and SQLite connection).
func (s *Store) Close() error { return s.Client.Close() }
```

- [ ] **Step 3: Implement `migrate.go`**

```go
package store

import (
	"context"
	"errors"

	"github.com/oioio-space/maldev/internal/manager/store/ent"
)

// EnsureSingletons makes sure the Setting (id=1) and ServerConfig (id=1)
// rows exist. Idempotent — safe to call on every boot. Sets kek_salt and
// kek_canary only if they are not yet populated; callers are responsible
// for providing them on first launch.
func (s *Store) EnsureSingletons(ctx context.Context, kekSalt, kekCanary []byte) error {
	// Setting
	_, err := s.Client.Setting.Get(ctx, 1)
	if ent.IsNotFound(err) {
		_, err = s.Client.Setting.Create().
			SetID(1).
			SetKekSalt(kekSalt).
			SetKekCanary(kekCanary).
			Save(ctx)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	// ServerConfig
	_, err = s.Client.ServerConfig.Get(ctx, 1)
	if ent.IsNotFound(err) {
		_, err = s.Client.ServerConfig.Create().SetID(1).Save(ctx)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	_ = errors.New // import retention
	return nil
}
```

- [ ] **Step 4: Write failing test**

```go
// internal/manager/store/store_test.go
package store

import (
	"context"
	"testing"

	"github.com/oioio-space/maldev/internal/manager/store/ent"
)

func TestNewInMemory(t *testing.T) {
	ctx := context.Background()
	s, err := New(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	if err := s.EnsureSingletons(ctx, []byte("salt-12345-6789-0"), []byte("canary")); err != nil {
		t.Fatal(err)
	}
	setting, err := s.Client.Setting.Get(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if string(setting.KekCanary) != "canary" {
		t.Fatalf("canary not persisted: %q", setting.KekCanary)
	}
}

func TestIssuerInsert(t *testing.T) {
	ctx := context.Background()
	s, _ := New(ctx, ":memory:")
	defer s.Close()

	iss, err := s.Client.Issuer.Create().
		SetName("Lab EU").
		SetKeyID("k-test").
		SetPublicKey(make([]byte, 32)).
		SetEncryptedPriv([]byte("enc")).
		SetActive(true).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if iss.ID.String() == "" {
		t.Fatal("ID not populated")
	}
	_ = ent.IsNotFound // import retention
}
```

- [ ] **Step 5: Tests pass**

Run: `go test ./internal/manager/store/...`
Expected: 2 PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/manager/store/
git commit -m "feat(manager/store): ENT-generated client + Store wrapper + singletons"
```

---

## Task 6 — Probe agent source + go:generate

**Files:**
- Create: `internal/manager/probe/agents/gen/main.go`, `internal/manager/probe/agents/gen/go.mod`
- Create: `internal/manager/probe/agents/gen.go`
- Create: `internal/manager/probe/types.go`

The agent is a tiny standalone Go program. To avoid pulling the whole maldev module into it, the `gen/` folder is its own module that depends only on `license/hostid`. Cross-compilation builds 5 binaries placed in `internal/manager/probe/agents/`.

- [ ] **Step 1: Create `types.go`**

```go
package probe

// AgentResult is the JSON the embedded probe binary POSTs back to the
// fingerprint probe server.
type AgentResult struct {
	Hostname     string `json:"hostname"`
	OS           string `json:"os"`
	Arch         string `json:"arch"`
	LocalHex     string `json:"local_hex"`
	CompositeHex string `json:"composite_hex"`
	CPUBrand     string `json:"cpu_brand,omitempty"`
	SentAt       string `json:"sent_at"`
}
```

- [ ] **Step 2: Create `agents/gen/main.go`**

```go
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

type result struct {
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
	body, _ := json.Marshal(result{
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

- [ ] **Step 3: Create `agents/gen/go.mod`**

Use a replace directive so the agent picks up the local hostid source:

```text
module github.com/oioio-space/maldev/internal/manager/probe/agents/gen

go 1.22

require github.com/oioio-space/maldev v0.0.0

replace github.com/oioio-space/maldev => ../../../../..
```

- [ ] **Step 4: Create `agents/gen.go` with build directives**

```go
//go:build never
// +build never

// This file exists solely to host the go:generate directives that build the
// embedded probe binaries. Building the file itself is excluded by the
// "never" build tag — only `go generate` invokes the directives.
//
//go:generate env GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w" -C gen -o ../linux-amd64 .
//go:generate env GOOS=linux GOARCH=arm64 go build -trimpath -ldflags "-s -w" -C gen -o ../linux-arm64 .
//go:generate env GOOS=darwin GOARCH=amd64 go build -trimpath -ldflags "-s -w" -C gen -o ../darwin-amd64 .
//go:generate env GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags "-s -w" -C gen -o ../darwin-arm64 .
//go:generate env GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-s -w" -C gen -o ../windows-amd64.exe .

package agents
```

- [ ] **Step 5: Run generation + verify**

```bash
go generate ./internal/manager/probe/agents/...
ls internal/manager/probe/agents/
```

Expected files: `linux-amd64`, `linux-arm64`, `darwin-amd64`, `darwin-arm64`, `windows-amd64.exe`.

- [ ] **Step 6: Add `.gitignore` entries for the binaries (we commit them per spec)**

Actually the spec says the binaries are embedded via //go:embed and committed so anyone cloning can build the manager without cross-compile infra. Verify they are tracked:

```bash
git status --short internal/manager/probe/agents/
```

- [ ] **Step 7: Commit**

```bash
git add internal/manager/probe/
git commit -m "feat(manager/probe): agent source + cross-compiled binaries (5 OS/arch)"
```

---

## Task 7 — Probe embed.go + ServeAgent

**Files:**
- Create: `internal/manager/probe/embed.go`, `internal/manager/probe/embed_test.go`

- [ ] **Step 1: Write failing test**

```go
// internal/manager/probe/embed_test.go
package probe

import (
	"bytes"
	"strings"
	"testing"
)

func TestServeAgentReturnsBinary(t *testing.T) {
	cases := []string{"linux-amd64", "linux-arm64", "darwin-amd64", "darwin-arm64", "windows-amd64"}
	for _, c := range cases {
		got, err := ServeAgent(c)
		if err != nil {
			t.Fatalf("ServeAgent(%q): %v", c, err)
		}
		if len(got) < 1024 {
			t.Fatalf("%s too small: %d bytes", c, len(got))
		}
	}
}

func TestServeAgentRejectsUnknown(t *testing.T) {
	_, err := ServeAgent("plan9-mips64")
	if err == nil {
		t.Fatal("unknown target accepted")
	}
	if !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("err=%v", err)
	}
}

func TestAvailableTargets(t *testing.T) {
	tgs := AvailableTargets()
	if len(tgs) != 5 {
		t.Fatalf("got %d targets, want 5", len(tgs))
	}
	if !bytes.Contains([]byte(strings.Join(tgs, ",")), []byte("linux-amd64")) {
		t.Fatalf("missing linux-amd64: %v", tgs)
	}
}
```

- [ ] **Step 2: Implement `embed.go`**

```go
package probe

import (
	"embed"
	"fmt"
	"sort"
)

//go:embed agents/linux-amd64 agents/linux-arm64 agents/darwin-amd64 agents/darwin-arm64 agents/windows-amd64.exe
var agentFS embed.FS

var targetFile = map[string]string{
	"linux-amd64":   "agents/linux-amd64",
	"linux-arm64":   "agents/linux-arm64",
	"darwin-amd64":  "agents/darwin-amd64",
	"darwin-arm64":  "agents/darwin-arm64",
	"windows-amd64": "agents/windows-amd64.exe",
}

// ServeAgent returns the raw bytes of the probe binary for target (formatted
// as "os-arch", e.g. "linux-amd64"). The Windows binary is served when
// "windows-amd64" is requested even though the embedded filename has the
// .exe suffix.
func ServeAgent(target string) ([]byte, error) {
	path, ok := targetFile[target]
	if !ok {
		return nil, fmt.Errorf("probe: unknown target %q (available: %v)", target, AvailableTargets())
	}
	return agentFS.ReadFile(path)
}

// AvailableTargets returns the list of supported OS-arch identifiers.
func AvailableTargets() []string {
	out := make([]string, 0, len(targetFile))
	for k := range targetFile {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
```

- [ ] **Step 3: Tests pass**

Run: `go test ./internal/manager/probe/...`
Expected: 3 PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/manager/probe/embed.go internal/manager/probe/embed_test.go
git commit -m "feat(manager/probe): //go:embed agent binaries + ServeAgent helper"
```

---

## Task 8 — service/audit.go + service/settings.go

These two are foundational, used by every other service.

**Files:**
- Create: `internal/manager/service/audit.go`, `internal/manager/service/settings.go`
- Create: `internal/manager/service/audit_test.go`, `internal/manager/service/settings_test.go`

- [ ] **Step 1: Implement `audit.go`**

```go
package service

import (
	"context"

	"github.com/oioio-space/maldev/internal/manager/store"
	"github.com/oioio-space/maldev/internal/manager/store/ent"
)

// Target identifies the entity an audit event refers to. ID is the
// stringified UUID (or "-" for global events).
type Target struct {
	Kind string
	ID   string
}

// AuditService writes structured operator events to the store.
type AuditService struct {
	store *store.Store
}

func NewAuditService(s *store.Store) *AuditService {
	return &AuditService{store: s}
}

// Append writes a new audit row. Pass nil payload for events with no
// extra detail. Intended to be called inside an existing transaction by
// other services; callers without a tx use this method directly.
func (a *AuditService) Append(ctx context.Context, kind, actor string, target Target, payload map[string]any) error {
	if payload == nil {
		payload = map[string]any{}
	}
	_, err := a.store.Client.AuditEvent.Create().
		SetKind(kind).
		SetTargetKind(target.Kind).
		SetTargetID(target.ID).
		SetActor(actor).
		SetPayload(payload).
		Save(ctx)
	return err
}

// AppendTx is the tx-aware variant used by services that compose multiple
// writes atomically.
func (a *AuditService) AppendTx(ctx context.Context, tx *ent.Tx, kind, actor string, target Target, payload map[string]any) error {
	if payload == nil {
		payload = map[string]any{}
	}
	_, err := tx.AuditEvent.Create().
		SetKind(kind).
		SetTargetKind(target.Kind).
		SetTargetID(target.ID).
		SetActor(actor).
		SetPayload(payload).
		Save(ctx)
	return err
}

// List returns up to limit recent events, newest first.
func (a *AuditService) List(ctx context.Context, limit int) ([]*ent.AuditEvent, error) {
	if limit <= 0 {
		limit = 100
	}
	return a.store.Client.AuditEvent.Query().
		Order(ent.Desc("created_at")).
		Limit(limit).
		All(ctx)
}

// ListForTarget returns events filtered to one target.
func (a *AuditService) ListForTarget(ctx context.Context, t Target, limit int) ([]*ent.AuditEvent, error) {
	if limit <= 0 {
		limit = 100
	}
	return a.store.Client.AuditEvent.Query().
		Where(/* target_kind == t.Kind AND target_id == t.ID — use the generated predicates */).
		Order(ent.Desc("created_at")).
		Limit(limit).
		All(ctx)
}
```

> **Note for the implementer:** the ENT-generated package gives you predicate functions like `auditevent.TargetKindEQ(...)`. Import and use them. The plan elides them only for terseness.

- [ ] **Step 2: Implement `settings.go`**

```go
package service

import (
	"context"

	"github.com/oioio-space/maldev/internal/manager/crypto"
	"github.com/oioio-space/maldev/internal/manager/store"
	"github.com/oioio-space/maldev/internal/manager/store/ent"
)

type SettingsService struct {
	store *store.Store
	kek   *crypto.KEK
}

func NewSettingsService(s *store.Store, k *crypto.KEK) *SettingsService {
	return &SettingsService{store: s, kek: k}
}

// Get returns the singleton Setting row (id=1). EnsureSingletons must have
// run at boot.
func (s *SettingsService) Get(ctx context.Context) (*ent.Setting, error) {
	return s.store.Client.Setting.Get(ctx, 1)
}

// Update runs mut against the SettingUpdate query and saves.
func (s *SettingsService) Update(ctx context.Context, mut func(*ent.SettingUpdateOne)) (*ent.Setting, error) {
	q := s.store.Client.Setting.UpdateOneID(1)
	mut(q)
	return q.Save(ctx)
}

func (s *SettingsService) GetServerConfig(ctx context.Context) (*ent.ServerConfig, error) {
	return s.store.Client.ServerConfig.Get(ctx, 1)
}

func (s *SettingsService) UpdateServerConfig(ctx context.Context, mut func(*ent.ServerConfigUpdateOne)) (*ent.ServerConfig, error) {
	q := s.store.Client.ServerConfig.UpdateOneID(1)
	mut(q)
	return q.Save(ctx)
}
```

- [ ] **Step 3: Tests**

```go
// internal/manager/service/audit_test.go
package service

import (
	"context"
	"testing"

	"github.com/oioio-space/maldev/internal/manager/store"
)

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.New(context.Background(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.EnsureSingletons(context.Background(), []byte("salt-16-byte-xxx"), []byte("c")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestAuditAppendList(t *testing.T) {
	s := newTestStore(t)
	a := NewAuditService(s)
	ctx := context.Background()
	if err := a.Append(ctx, "test.event", "alice", Target{Kind: "X", ID: "id-1"}, map[string]any{"k": "v"}); err != nil {
		t.Fatal(err)
	}
	rows, err := a.List(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Kind != "test.event" {
		t.Fatalf("got %+v", rows)
	}
}
```

```go
// internal/manager/service/settings_test.go
package service

import (
	"context"
	"testing"

	"github.com/oioio-space/maldev/internal/manager/store/ent"
)

func TestSettingsGetUpdate(t *testing.T) {
	s := newTestStore(t)
	settings := NewSettingsService(s, nil)
	ctx := context.Background()

	row, err := settings.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if row.DefaultTTLSeconds == 0 {
		t.Fatal("default TTL should be set by schema default")
	}
	_, err = settings.Update(ctx, func(u *ent.SettingUpdateOne) {
		u.SetOperatorName("alice")
	})
	if err != nil {
		t.Fatal(err)
	}
	row2, _ := settings.Get(ctx)
	if row2.OperatorName != "alice" {
		t.Fatalf("operator_name=%q", row2.OperatorName)
	}
}
```

- [ ] **Step 4: Build + tests pass**

Run: `go test ./internal/manager/...`
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/manager/service/audit.go internal/manager/service/settings.go internal/manager/service/audit_test.go internal/manager/service/settings_test.go
git commit -m "feat(manager/service): AuditService + SettingsService"
```

---

## Task 9 — service/issuer.go

**Files:**
- Create: `internal/manager/service/issuer.go`, `internal/manager/service/issuer_test.go`

- [ ] **Step 1: Implement `issuer.go`**

```go
package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/oioio-space/maldev/internal/manager/crypto"
	"github.com/oioio-space/maldev/internal/manager/store"
	"github.com/oioio-space/maldev/internal/manager/store/ent"
	"github.com/oioio-space/maldev/license"
)

type IssuerService struct {
	store *store.Store
	kek   *crypto.KEK
	audit *AuditService
}

func NewIssuerService(s *store.Store, k *crypto.KEK, a *AuditService) *IssuerService {
	return &IssuerService{store: s, kek: k, audit: a}
}

// Generate creates a fresh Ed25519 keypair and persists it (encrypted priv).
// The new issuer is NOT marked active automatically — caller decides.
func (svc *IssuerService) Generate(ctx context.Context, name, keyID, actor string) (*ent.Issuer, error) {
	pub, priv, err := license.GenerateKey()
	if err != nil {
		return nil, err
	}
	wrapped, err := svc.kek.Wrap(priv)
	if err != nil {
		return nil, err
	}
	var iss *ent.Issuer
	err = svc.store.Client.Tx(ctx, nil)(func(ctx context.Context, tx *ent.Tx) error {
		row, err := tx.Issuer.Create().
			SetName(name).
			SetKeyID(keyID).
			SetPublicKey(pub).
			SetEncryptedPriv(wrapped).
			SetActive(false).
			Save(ctx)
		if err != nil {
			return err
		}
		iss = row
		return svc.audit.AppendTx(ctx, tx, "issuer.generate", actor,
			Target{Kind: "Issuer", ID: row.ID.String()},
			map[string]any{"name": name, "key_id": keyID})
	})
	return iss, err
}

// SetActive marks id active and unsets all other issuers.
func (svc *IssuerService) SetActive(ctx context.Context, id uuid.UUID, actor string) error {
	return svc.store.Client.Tx(ctx, nil)(func(ctx context.Context, tx *ent.Tx) error {
		if _, err := tx.Issuer.Update().SetActive(false).Save(ctx); err != nil {
			return err
		}
		if _, err := tx.Issuer.UpdateOneID(id).SetActive(true).Save(ctx); err != nil {
			return err
		}
		return svc.audit.AppendTx(ctx, tx, "issuer.set_active", actor,
			Target{Kind: "Issuer", ID: id.String()}, nil)
	})
}

func (svc *IssuerService) Active(ctx context.Context) (*ent.Issuer, error) {
	return svc.store.Client.Issuer.Query().
		Where(/* issuer.ActiveEQ(true) */).
		First(ctx)
}

func (svc *IssuerService) List(ctx context.Context) ([]*ent.Issuer, error) {
	return svc.store.Client.Issuer.Query().All(ctx)
}

func (svc *IssuerService) Get(ctx context.Context, id uuid.UUID) (*ent.Issuer, error) {
	return svc.store.Client.Issuer.Get(ctx, id)
}

// PrivateKey returns the decrypted Ed25519 private key for in-memory use.
// Caller must wipe it after use.
func (svc *IssuerService) PrivateKey(ctx context.Context, id uuid.UUID) ([]byte, error) {
	row, err := svc.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return svc.kek.Unwrap(row.EncryptedPriv)
}

// ExportPublic returns the public key as a PEM "MALDEV PUBLIC KEY" with
// KID header.
func (svc *IssuerService) ExportPublic(ctx context.Context, id uuid.UUID) ([]byte, error) {
	row, err := svc.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return license.MarshalPublicKey(row.PublicKey, row.KeyID)
}

// Note: svc.store.Client.Tx(...) helper. Use ent's WithTx pattern instead
// if the generated code prefers it. Implementer: follow the canonical
// pattern from ent docs (TxFunc + Rollback on panic) — see ent README.
var _ = errors.New
var _ = fmt.Errorf
```

> **Implementer note:** `svc.store.Client.Tx(ctx, nil)(func(...))` is a shorthand. The canonical ent pattern is:
> ```go
> tx, err := svc.store.Client.Tx(ctx)
> if err != nil { return err }
> // ... do work
> if err := tx.Commit(); err != nil { _ = tx.Rollback(); return err }
> ```
> Wrap in a helper `withTx(ctx, store, func(ctx, tx) error) error` in `services.go` later (Task 17) to avoid repetition.

- [ ] **Step 2: Test**

```go
// internal/manager/service/issuer_test.go
package service

import (
	"context"
	"testing"

	"github.com/oioio-space/maldev/internal/manager/crypto"
)

func newKEK(t *testing.T) *crypto.KEK {
	t.Helper()
	return crypto.DeriveFromPassphrase("test", [16]byte{1})
}

func TestIssuerGenerateAndActivate(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	kek := newKEK(t)
	audit := NewAuditService(s)
	svc := NewIssuerService(s, kek, audit)

	iss, err := svc.Generate(ctx, "Lab EU", "k-1", "operator")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.SetActive(ctx, iss.ID, "operator"); err != nil {
		t.Fatal(err)
	}
	active, err := svc.Active(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if active.ID != iss.ID {
		t.Fatalf("active=%s want %s", active.ID, iss.ID)
	}

	priv, err := svc.PrivateKey(ctx, iss.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(priv) != 64 {
		t.Fatalf("priv len=%d", len(priv))
	}

	pemBytes, err := svc.ExportPublic(ctx, iss.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(pemBytes) < 50 {
		t.Fatalf("PEM too small: %s", pemBytes)
	}
}
```

- [ ] **Step 3: Tests pass + commit**

```bash
go test ./internal/manager/service/...
git add internal/manager/service/issuer.go internal/manager/service/issuer_test.go
git commit -m "feat(manager/service): IssuerService — generate/activate/export"
```

---

## Tasks 10-16 — Remaining services

Each follows the same pattern as Task 9: implement the service struct + methods from the spec §7, write tests for the happy path + 1-2 edge cases, commit.

### Task 10 — IdentityService

**Files:** `internal/manager/service/identity.go` + test

Methods: `Create(ctx, name) → (*ent.Identity, error)` (generates 32 bytes via crypto/rand, computes sha256), `Import(name, bytes)`, `List`, `ExportBin(id) []byte`, `Regenerate(id, confirmed)` (rotates bytes + sha256), `Delete(id)` (refuse if UsageCount > 0), `UsageCount(id) int` (count of licenses with identity_sha256 = this row's sha256).

Test: create + list + UsageCount=0 + Delete succeeds. Then re-create + insert a fake license referencing the sha → UsageCount=1 → Delete fails.

Commit: `feat(manager/service): IdentityService`

### Task 11 — RecipientService

**Files:** `internal/manager/service/recipient.go` + test

Methods: `Generate(name)` (calls `seal.GenerateRecipient`, wraps priv with KEK), `Import(name, pub, priv)`, `List`, `ExportPublic(id)`, `Delete(id)`.

Test: Generate + roundtrip Unwrap of priv.

Commit: `feat(manager/service): RecipientService`

### Task 12 — TOTPService

**Files:** `internal/manager/service/totp.go` + test

Methods: `Get(licenseID) → secret + QR ASCII + PNG` (decrypts secret, calls `totp.URI/QRImageASCII/QRImagePNG`). Only read paths — the secret is created by LicenseService when there's a TOTP binding.

Helpers: `createForLicense(ctx, tx, licenseID, label) (*ent.TOTPSecret, secret string, error)` — called by LicenseService inside its tx.

Test: insert a fake TOTPSecret with a known base32, Get returns matching QR strings.

Commit: `feat(manager/service): TOTPService`

### Task 13 — ProbeService

**Files:** `internal/manager/service/probe.go` + test

Methods:
- `NewToken(ctx, label, ttl) (*ent.ProbeToken, error)` — 16-byte random hex, persist
- `Subscribe(tokenID string) <-chan *ent.ProbeToken` — register a channel in a `sync.Map[string]chan`
- `ConsumeToken(ctx, tokenID, result probe.AgentResult, remoteAddr) error` — load + check expiry + check unused + write fields → tx commit → notify subscriber if any → audit
- `History(ctx, limit) ([]*ent.ProbeToken, error)`
- `Revoke(ctx, tokenID)` — marks expired

Test: NewToken + Subscribe + ConsumeToken roundtrip: subscriber channel receives the populated token within 100ms.

Commit: `feat(manager/service): ProbeService`

### Task 14 — LicenseService

**Files:** `internal/manager/service/license.go` + test

The largest service. Methods per spec §7:
- `Issue(ctx, IssueRequest) (*IssuedLicense, error)`
- `ReIssue(ctx, originalID, opts) (*IssuedLicense, error)`
- `List(ctx, ListFilter) ([]*ent.License, error)`
- `Get(ctx, id) (*LicenseDetail, error)`
- `GetByUUID(ctx, uuid) (*ent.License, error)`
- `Inspect(pem []byte) (*license.License, error)` — wraps `license.Inspect`
- `Import(ctx, pem, label) (*ent.License, error)` — parses + persists
- `ExportPEM(ctx, id) ([]byte, error)`
- `ExportBatch(ctx, ids) ([]byte, error)` — tar.gz
- `HashFile(ctx, path, progress) (string, error)` — sha256 with progress callback

`Issue` flow:
1. Resolve `IssuerID` → row → priv key (decrypt)
2. Resolve `IdentityID` → bytes → sha256
3. For each `BindingSpec`, call `license.BindMachineIDs / BindPasswordWithParams / BindTOTP / BindCustom` from the spec
4. For TOTP bindings, generate a secret via `totp.NewSecret()` and stash it for later persist
5. If SealedFor → `seal.Seal(recipientPub, sealedPlain)` → `IssueOptions.SealedPayload`
6. Call `license.Issue(opts)` → PEM
7. Tx: create License row, create TOTPSecret rows, audit, commit
8. Wipe priv

Test: end-to-end issue with machine + password + TOTP bindings, verify that `license.Verify(pem, trusted, WithPassword, WithMachineID, WithTOTPCode)` succeeds.

Commit: `feat(manager/service): LicenseService — Issue/ReIssue/List/Get/Inspect/Import/Export`

### Task 15 — RevokeService

**Files:** `internal/manager/service/revoke.go` + test

Methods:
- `Revoke(ctx, licenseID, reason, actor)` — creates a Revocation row + sets License.status = revoked
- `Unrevoke(ctx, licenseID)` — deletes Revocation + sets status = active
- `ListRevoked(ctx)` — joined query
- `PublishSignedList(ctx) ([]byte, error)` — reads all revoked, builds `revoke.List` (sequence from a counter in ServerConfig or AuditEvent count), signs with active issuer

Test: revoke 2 licenses, PublishSignedList returns a PEM that `revoke.VerifyBytes` accepts.

Commit: `feat(manager/service): RevokeService + signed list publication`

### Task 16 — services.go bundle + withTx helper

**Files:** `internal/manager/service/services.go`, `internal/manager/service/tx.go`

```go
// services.go
package service

import (
	"github.com/oioio-space/maldev/internal/manager/crypto"
	"github.com/oioio-space/maldev/internal/manager/store"
)

type Services struct {
	Store *store.Store
	KEK   *crypto.KEK

	Audit     *AuditService
	Settings  *SettingsService
	Issuer    *IssuerService
	Identity  *IdentityService
	Recipient *RecipientService
	TOTP      *TOTPService
	Probe     *ProbeService
	License   *LicenseService
	Revoke    *RevokeService
}

func New(s *store.Store, k *crypto.KEK) *Services {
	audit := NewAuditService(s)
	settings := NewSettingsService(s, k)
	issuer := NewIssuerService(s, k, audit)
	identity := NewIdentityService(s, audit)
	recipient := NewRecipientService(s, k, audit)
	totp := NewTOTPService(s, k)
	probe := NewProbeService(s, audit)
	license := NewLicenseService(s, k, audit, issuer, identity, recipient, totp)
	revoke := NewRevokeService(s, k, audit, issuer, license)
	return &Services{
		Store: s, KEK: k,
		Audit: audit, Settings: settings, Issuer: issuer, Identity: identity,
		Recipient: recipient, TOTP: totp, Probe: probe, License: license,
		Revoke: revoke,
	}
}

func (s *Services) Close() error {
	if s.KEK != nil {
		s.KEK.Wipe()
	}
	return s.Store.Close()
}
```

```go
// tx.go
package service

import (
	"context"
	"fmt"

	"github.com/oioio-space/maldev/internal/manager/store"
	"github.com/oioio-space/maldev/internal/manager/store/ent"
)

// withTx runs fn within a transaction, rolling back on error/panic.
func withTx(ctx context.Context, s *store.Store, fn func(ctx context.Context, tx *ent.Tx) error) error {
	tx, err := s.Client.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if r := recover(); r != nil {
			_ = tx.Rollback()
			panic(r)
		}
	}()
	if err := fn(ctx, tx); err != nil {
		if rerr := tx.Rollback(); rerr != nil {
			return fmt.Errorf("%w (rollback: %v)", err, rerr)
		}
		return err
	}
	return tx.Commit()
}
```

Refactor Issuer/License/Revoke/Identity to use `withTx` instead of the shorthand. Run all tests.

Commit: `feat(manager/service): Services bundle + withTx helper, refactor services to use it`

---

## Task 17 — httpsrv/server.go — interface + types

**Files:** `internal/manager/httpsrv/server.go`

```go
package httpsrv

import (
	"context"
	"time"
)

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

Commit: `feat(manager/httpsrv): Server interface + Status/Event types`

---

## Task 18 — httpsrv/revocation.go

**Files:** `internal/manager/httpsrv/revocation.go` + test

`RevocationServer` wraps `license/server.NewRevocationHandler`. The handler is constructed dynamically per Start so admin token rotation = stop+start. The RevocationStore adapter calls `RevokeService.PublishSignedList` (returns the freshly-signed bytes) but the simpler integration is to plug a custom http.Handler that calls PublishSignedList on every GET.

Implementer notes:
- Use an internal `http.Server` field.
- `Events()` is a buffered channel (cap 256, drop-oldest).
- Status counters via atomics (`atomic.Uint64`).

Test: httptest server, GET returns a PEM that `revoke.VerifyBytes` parses.

Commit: `feat(manager/httpsrv): RevocationServer with lifecycle + Events()`

---

## Task 19 — httpsrv/heartbeat.go

Same pattern. Wraps `license/server.NewHeartbeatHandler` with a `LicenseStore` adapter calling `LicenseService.GetByUUID`.

Commit: `feat(manager/httpsrv): HeartbeatServer with LicenseService adapter`

---

## Task 20 — httpsrv/probe.go

**Files:** `internal/manager/httpsrv/probe.go` + test

Routes:
- `GET /probe/{token}/agent/{os-arch}` → `probe.ServeAgent`
- `GET /probe/{token}/snippet` → renders the 3-platform snippet string
- `POST /probe/{token}/result` → decodes JSON `probe.AgentResult`, calls `service.ProbeService.ConsumeToken`

Use `net/http` stdlib (Go 1.22+ pattern-based routing).

Test: httptest server, POST result, verify ProbeService.History shows the result.

Commit: `feat(manager/httpsrv): ProbeServer with /agent + /snippet + /result`

---

## Task 21 — httpsrv/bundle.go

**Files:** `internal/manager/httpsrv/bundle.go`

```go
package httpsrv

import (
	"sync"
	"time"
)

type Bundle struct {
	Revocation *RevocationServer
	Heartbeat  *HeartbeatServer
	Probe      *ProbeServer

	merged chan Event
	mu     sync.Mutex
}

func (b *Bundle) MergedEvents() <-chan Event {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.merged != nil {
		return b.merged
	}
	b.merged = make(chan Event, 512)
	for _, s := range []Server{b.Revocation, b.Heartbeat, b.Probe} {
		go func(src <-chan Event) {
			for e := range src {
				select {
				case b.merged <- e:
				default: // drop-oldest behaviour: consumer too slow
				}
			}
		}(s.Events())
	}
	return b.merged
}

func (b *Bundle) StopAll(timeout time.Duration) error {
	var first error
	for _, s := range []Server{b.Revocation, b.Heartbeat, b.Probe} {
		if err := s.Stop(timeout); err != nil && first == nil {
			first = err
		}
	}
	return first
}
```

Commit: `feat(manager/httpsrv): Bundle with MergedEvents + StopAll`

---

## Task 22 — cmd/license-manager/main.go — boot loader

**Files:** `cmd/license-manager/main.go` (replace stub)

Full bootstrap flow per spec §6. The implementer should:
1. Parse flags
2. Resolve passphrase via the cascade (flag-file → env-file → env → terminal prompt)
3. Open store
4. EnsureSingletons (with kek_salt + canary for fresh DB)
5. Verify canary if existing DB
6. Build Services
7. If `--no-tui` → print a summary and exit 0
8. Otherwise → placeholder for TUI launch (not in this plan)

Use `golang.org/x/term.ReadPassword(int(os.Stdin.Fd()))` for the terminal prompt.

Test: build the binary, run with a temp DB, `--no-tui`, env var passphrase → expect "boot ok" stdout.

Commit: `feat(cmd/license-manager): boot loader with passphrase cascade`

---

## Task 23 — Integration test

**Files:** `internal/manager/manager_test.go`

Full end-to-end: open store, build services, generate keypair (and SetActive), issue a license with machine + password + TOTP bindings + features, verify the PEM with `license.Verify(...)` using a TOTP code computed at the right moment, revoke, publish signed list, verify the list flags the license, start the revocation HTTP server, GET it, verify the response.

Commit: `test(manager): end-to-end issue + verify + revoke + publish`

---

## Task 24 — Documentation

**Files:**
- Create: `docs/license-manager/concepts.md`, `docs/license-manager/workflow.md`
- Modify: `README.md` (add manager to packages table), `docs/SUMMARY.md` (link new pages)

`concepts.md` covers: what the manager is, store layout, encrypted columns, passphrase model, 3 HTTP servers and when each is on, fingerprint probe flow.

`workflow.md` covers: first launch (create issuer + first licence), daily ops (issue, revoke, rotate, batch probe), operations (start/stop servers, rotate admin token).

Run `go run ./internal/tools/docgen` after edits.

Commit: `docs(license-manager): concepts + workflow`

---

## Task 25 — Final sweep + tracker + push + tag

**Files:** `.dev/license-manager-2026/progress.md`, `.dev/license-manager-2026/backlog.md`

```text
# .dev/license-manager-2026/progress.md
---
title: license-manager backend — implementation progress
last_reviewed: 2026-05-20
reflects_commit: HEAD
---

- [x] Task 1 — scaffold
- [x] Task 2 — crypto/KEK
- [x] Task 3 — crypto/Wrap+Unwrap+Canary
- [x] Task 4 — ENT schemas
- [x] Task 5 — ENT generation + Store
- [x] Task 6 — probe agent source
- [x] Task 7 — probe embed.go
- [x] Task 8 — audit + settings services
- [x] Task 9 — issuer service
- [x] Task 10 — identity service
- [x] Task 11 — recipient service
- [x] Task 12 — totp service
- [x] Task 13 — probe service
- [x] Task 14 — license service
- [x] Task 15 — revoke service
- [x] Task 16 — services bundle + withTx
- [x] Task 17 — httpsrv interface
- [x] Task 18 — revocation server
- [x] Task 19 — heartbeat server
- [x] Task 20 — probe server
- [x] Task 21 — httpsrv bundle
- [x] Task 22 — cmd/license-manager boot
- [x] Task 23 — integration test
- [x] Task 24 — docs
- [x] Task 25 — final sweep
```

```text
# .dev/license-manager-2026/backlog.md
- [ ] TUI layer (separate plan, after Claude Design handoff)
- [ ] OS keystore integration for passphrase (DPAPI / Keychain / libsecret)
- [ ] Stateful seat counter
- [ ] Push webhooks
- [ ] License chain graph viz
- [ ] cmd/license CLI (non-TUI)
```

- [ ] **Step 1: Build + race tests + cross-platform**

```bash
go build ./...
go test -race ./internal/manager/...
GOOS=linux GOARCH=amd64 go build ./...
GOOS=darwin GOARCH=amd64 go build ./...
GOOS=windows GOARCH=amd64 go build ./...
```

- [ ] **Step 2: /simplify pass on the new packages**

Use the simplify skill to review all of `internal/manager/` and `cmd/license-manager/`.

- [ ] **Step 3: Commit trackers + push**

```bash
git add .dev/license-manager-2026/
git commit -m "docs(.dev): license-manager backend M1 — implementation complete"
git push origin master
```

- [ ] **Step 4: Tag**

```bash
git tag -a v0.161.0 -m "license-manager backend v0.161.0

Adds internal/manager/ backend for the upcoming TUI: ENT+SQLite store with
column-level encryption at rest, services for issuer/license/revoke/identity/
recipient/totp/probe/audit/settings, three lifecycle-managed HTTP servers
(revocation, heartbeat, fingerprint probe with embedded per-OS agent
binaries), and a boot loader with passphrase cascade.

The TUI layer is out-of-scope of this release; cmd/license-manager exposes
*service.Services for the next milestone."
git push origin v0.161.0
```

---

## Self-review

**Spec coverage:**
- §3 Architecture → Task 1 + structure throughout
- §4 Schema (10 entities) → Task 4
- §5 Crypto → Tasks 2-3
- §6 Bootstrap → Task 22
- §7 Services → Tasks 8-16
- §8 HTTP servers → Tasks 17-21
- §9 Embedded probe → Tasks 6-7
- §10 Atomicity → withTx helper Task 16, used by all mutating services
- §11 Testing → per-task tests + Task 23 integration
- §13 Documentation → Task 24
- §14 Adjustments to Claude Design → already integrated in the design prompt; not implementation

**Placeholder scan:** the plan elides a few helper details ("use the generated predicates", "use canonical ent tx pattern") with explicit implementer notes. No silent TODOs.

**Type consistency:** `Services` fields match across `services.go` and the per-service constructors. `Target` consistent. `Event/Status` shapes single-sourced in `httpsrv/server.go`.
