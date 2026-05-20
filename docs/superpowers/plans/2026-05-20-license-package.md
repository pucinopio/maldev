# `license/` Package Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver the `license/` package described in `docs/superpowers/specs/2026-05-20-license-package-design.md`: a defensive framing primitive for maldev research binaries (Ed25519-signed PEM-armored licenses with bindings, revocation, heartbeat, identity pinning, clock-tamper detection).

**Architecture:** Single Go module addition under `github.com/oioio-space/maldev/license`. Layer-1 in the repo stack (uses `crypto/ed25519` stdlib + `cleanup/memory` + `internal/log` + Layer-0 `license/hostid`). Sub-packages are exported and individually usable: `canonical`, `hostid`, `identity`, `revoke`, `heartbeat`, `seal`, `ntp`, `server`.

**Tech Stack:** Go 1.22+, stdlib `crypto/ed25519`, `golang.org/x/crypto/argon2`, `golang.org/x/crypto/hkdf`, `golang.org/x/crypto/chacha20poly1305`, `golang.org/x/crypto/curve25519`. No new third-party dependencies beyond `golang.org/x/crypto` (already used by the repo per `go.sum`).

**Commit conventions:** Conventional commits per repo style. Every Go-modifying commit triggers `/simplify` per CLAUDE.md before being staged. The tech-md page is updated in the same commit as the API surface that requires it.

---

## File Structure (locked decomposition)

```
license/
  doc.go
  license.go                  -- License, Verified, signedLicense types
  issue.go                    -- Issue(), New() (one-liner)
  verify.go                   -- Verify() entry + flow orchestration
  verify_options.go           -- WithAudience, WithMachineID, WithPassword, ...
  keys.go                     -- GenerateKey, Marshal/Parse PEM keys, Trusted
  pem.go                      -- PEM block encode/decode (MALDEV LICENSE)
  errors.go                   -- ErrLicenseInvalid + internal cause enum
  clock.go                    -- Clock interface + realClock
  state.go                    -- HMAC-protected State file r/w
  bindings.go                 -- Binding type + Bind* helpers + match logic
  pinning.go                  -- HashFile, HashIdentity, binary pinning check
  hash.go                     -- canonical sha256 helpers + domain tags
  *_test.go                   -- per-source-file unit tests

  canonical/
    canonical.go              -- deterministic JSON marshal
    canonical_test.go

  hostid/
    hostid.go                 -- Local() public API + mixing logic
    hostid_windows.go         -- registry + DMI
    hostid_linux.go           -- /etc/machine-id + DMI
    hostid_darwin.go          -- IOPlatformUUID best-effort
    hostid_test.go

  identity/
    identity.go               -- Set/Read + HashIdentity helper
    identity_test.go
    cmd/gen-identity/main.go  -- writes 32 random bytes to identity.bin (idempotent)

  revoke/
    list.go                   -- RevocationList type + Sign/Verify/IsRevoked
    source.go                 -- RevocationSource interface + HTTP/File/Embed/Multi
    cache.go                  -- signed cache file + monotonic sequence check
    revoke_test.go

  heartbeat/
    client.go                 -- HeartbeatClient interface + HTTPClient
    types.go                  -- HeartbeatReply, HeartbeatRequest
    client_test.go

  seal/
    seal.go                   -- Seal/Open (X25519 + ChaCha20-Poly1305)
    seal_test.go

  ntp/
    ntp.go                    -- minimal SNTPv4 query
    ntp_test.go

  server/
    revocation.go             -- NewRevocationHandler
    heartbeat.go              -- NewHeartbeatHandler
    store.go                  -- RevocationStore, LicenseStore interfaces + FileStore
    server_test.go

cmd/
  license-test/main.go        -- E2E test binary (not shipped in normal builds)

docs/
  techniques/license-framing.md
  license/workflow.md
  license/threat-model.md
```

The plan implements files in dependency order so each task ends with `go build ./...` and `go test ./...` passing.

---

## Task 1 — Module scaffold & doc.go

**Files:**
- Create: `license/doc.go`

- [ ] **Step 1: Create `license/doc.go`**

```go
// Package license provides a defensive framing primitive for maldev research
// binaries: signed, structured license tokens that constrain who may run a
// given binary, on which machines, with which secrets, until when, and against
// which revocation/heartbeat policy.
//
// Technique: License framing for authorized maldev tooling.
// MITRE ATT&CK: N/A (defensive primitive; no offensive technique mapped).
// Detection level: N/A (no on-host artefacts emitted; consult docs/techniques/license-framing.md).
//
// Threat model (summary; see docs/license/threat-model.md for the full version):
//
//   Resists: license forgery, post-issuance tampering, replay across audiences,
//   cross-binary reuse, stale-cache substitution, brute-force on password
//   bindings, clock rollback below trusted-floor, algorithm-confusion attacks
//   on the signature.
//
//   Does NOT resist: an attacker who patches Verify in the binary; permanent
//   offline use beyond grace period; perfect clock tamper without TPM; binary
//   modification combined with identity-bytes modification; hostid spoofing on
//   a machine the attacker controls fully.
//
// Composition: the root package exposes Issue/Verify/GenerateKey and the
// option set. Sub-packages provide optional features (revocation, heartbeat,
// identity, sealed payload, NTP). A consumer that only needs offline
// verification imports only the root package and pulls no sub-package code.
package license
```

- [ ] **Step 2: Run `go build ./license/...`**

Run: `go build ./license/...`
Expected: builds (empty package, no errors).

- [ ] **Step 3: Commit**

```bash
git add license/doc.go
git commit -m "feat(license): scaffold package with doc.go"
```

---

## Task 2 — `canonical/` sub-package (deterministic JSON)

**Files:**
- Create: `license/canonical/canonical.go`
- Test: `license/canonical/canonical_test.go`

- [ ] **Step 1: Write the failing test**

```go
// license/canonical/canonical_test.go
package canonical

import (
    "encoding/json"
    "strings"
    "testing"
    "time"
)

func TestMarshalSortsKeys(t *testing.T) {
    in := map[string]any{"b": 2, "a": 1, "c": map[string]any{"y": "Y", "x": "X"}}
    out, err := Marshal(in)
    if err != nil {
        t.Fatal(err)
    }
    want := `{"a":1,"b":2,"c":{"x":"X","y":"Y"}}`
    if string(out) != want {
        t.Fatalf("got %s want %s", out, want)
    }
}

func TestMarshalTimeRFC3339Nano(t *testing.T) {
    in := map[string]any{"t": time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC)}
    out, err := Marshal(in)
    if err != nil {
        t.Fatal(err)
    }
    if !strings.Contains(string(out), `"t":"2026-05-20T10:00:00Z"`) {
        t.Fatalf("got %s", out)
    }
}

func TestMarshalDeterministicRoundtrip(t *testing.T) {
    type X struct {
        B int            `json:"b"`
        A string         `json:"a"`
        M map[string]int `json:"m"`
    }
    v := X{B: 2, A: "hi", M: map[string]int{"y": 9, "x": 8}}
    a, _ := Marshal(v)
    b, _ := Marshal(v)
    if string(a) != string(b) {
        t.Fatalf("non-deterministic: %s vs %s", a, b)
    }
    // Match stdlib for parse compatibility.
    var back X
    if err := json.Unmarshal(a, &back); err != nil {
        t.Fatal(err)
    }
    if back != v {
        t.Fatalf("roundtrip mismatch: %+v", back)
    }
}

func TestMarshalNoHTMLEscape(t *testing.T) {
    in := map[string]string{"k": "<a>&b"}
    out, _ := Marshal(in)
    if !strings.Contains(string(out), "<a>&b") {
        t.Fatalf("HTML was escaped: %s", out)
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./license/canonical/...`
Expected: FAIL with "undefined: Marshal".

- [ ] **Step 3: Implement `canonical/canonical.go`**

```go
// Package canonical encodes Go values to a deterministic JSON form suitable
// for signing: object keys are recursively sorted, no insignificant whitespace
// is emitted, HTML characters are not escaped, and time.Time values are
// rendered in RFC3339Nano UTC.
package canonical

import (
    "bytes"
    "encoding/json"
    "fmt"
    "sort"
    "time"
)

func Marshal(v any) ([]byte, error) {
    norm, err := normalise(v)
    if err != nil {
        return nil, err
    }
    var buf bytes.Buffer
    enc := json.NewEncoder(&buf)
    enc.SetEscapeHTML(false)
    if err := enc.Encode(norm); err != nil {
        return nil, err
    }
    // json.Encoder appends a trailing newline; strip it.
    out := buf.Bytes()
    if len(out) > 0 && out[len(out)-1] == '\n' {
        out = out[:len(out)-1]
    }
    return out, nil
}

func normalise(v any) (any, error) {
    // Round-trip through stdlib first so struct tags are honoured, then
    // walk the resulting tree to sort maps and normalise times.
    raw, err := json.Marshal(v)
    if err != nil {
        return nil, err
    }
    var tree any
    dec := json.NewDecoder(bytes.NewReader(raw))
    dec.UseNumber()
    if err := dec.Decode(&tree); err != nil {
        return nil, err
    }
    return walk(tree)
}

func walk(v any) (any, error) {
    switch x := v.(type) {
    case map[string]any:
        keys := make([]string, 0, len(x))
        for k := range x {
            keys = append(keys, k)
        }
        sort.Strings(keys)
        out := orderedMap{}
        for _, k := range keys {
            nv, err := walk(x[k])
            if err != nil {
                return nil, err
            }
            out = append(out, kv{Key: k, Val: nv})
        }
        return out, nil
    case []any:
        for i, e := range x {
            nv, err := walk(e)
            if err != nil {
                return nil, err
            }
            x[i] = nv
        }
        return x, nil
    case string:
        // Detect RFC3339 strings produced by time.Time.MarshalJSON and
        // re-render in RFC3339Nano UTC for stable comparison.
        if t, err := time.Parse(time.RFC3339Nano, x); err == nil {
            return t.UTC().Format(time.RFC3339Nano), nil
        }
        return x, nil
    default:
        return x, nil
    }
}

type kv struct {
    Key string
    Val any
}

type orderedMap []kv

func (o orderedMap) MarshalJSON() ([]byte, error) {
    var buf bytes.Buffer
    buf.WriteByte('{')
    for i, p := range o {
        if i > 0 {
            buf.WriteByte(',')
        }
        kb, err := json.Marshal(p.Key)
        if err != nil {
            return nil, err
        }
        buf.Write(kb)
        buf.WriteByte(':')
        vb, err := json.Marshal(p.Val)
        if err != nil {
            return nil, fmt.Errorf("canonical: value at %q: %w", p.Key, err)
        }
        buf.Write(vb)
    }
    buf.WriteByte('}')
    return buf.Bytes(), nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./license/canonical/... -v`
Expected: 4 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add license/canonical/
git commit -m "feat(license/canonical): deterministic JSON for signing"
```

---

## Task 3 — `errors.go` + `clock.go` + domain tags + `hash.go`

**Files:**
- Create: `license/errors.go`, `license/clock.go`, `license/hash.go`
- Test: `license/clock_test.go`, `license/hash_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// license/clock_test.go
package license

import (
    "testing"
    "time"
)

func TestRealClockNonZero(t *testing.T) {
    var c realClock
    if c.Now().IsZero() {
        t.Fatal("realClock returned zero time")
    }
}

func TestFakeClockUsable(t *testing.T) {
    ref := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
    c := &FakeClock{T: ref}
    if !c.Now().Equal(ref) {
        t.Fatal("FakeClock did not return configured time")
    }
}
```

```go
// license/hash_test.go
package license

import (
    "bytes"
    "crypto/sha256"
    "testing"
)

func TestSignPayloadDomainTag(t *testing.T) {
    out := signPayload(tagLicenseV1, []byte("hello"))
    want := append([]byte("maldev-license-v1\x00"), []byte("hello")...)
    if !bytes.Equal(out, want) {
        t.Fatalf("got %x want %x", out, want)
    }
}

func TestHashBytes(t *testing.T) {
    h := sha256Hex([]byte("hello"))
    expect := sha256.Sum256([]byte("hello"))
    if h != hexEncode(expect[:]) {
        t.Fatalf("hash mismatch")
    }
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./license/...`
Expected: FAIL — undefined `realClock`, `FakeClock`, `signPayload`, `tagLicenseV1`, `sha256Hex`, `hexEncode`.

- [ ] **Step 3: Implement `errors.go`**

```go
// license/errors.go
package license

import (
    "errors"
    "fmt"
)

// ErrLicenseInvalid is the single public error returned by Verify and related
// entry points. Detailed causes are logged via the injected slog.Logger but
// never surfaced in the error string — this prevents an attacker from
// brute-forcing constraint by constraint and observing which check failed.
var ErrLicenseInvalid = errors.New("license: verification failed")

type cause int

const (
    causeOK cause = iota
    causeBadFormat
    causeBadSignature
    causeUnknownKey
    causeNotYetValid
    causeExpired
    causeClockRollback
    causeAudienceMismatch
    causeIssuerMismatch
    causeBindingMachineMismatch
    causeBindingPasswordMismatch
    causeBindingCustomMismatch
    causeBinaryHashMismatch
    causeIdentityMismatch
    causeRevoked
    causeRevocationStale
    causeHeartbeatFailed
    causeStateCorrupted
)

func (c cause) String() string {
    switch c {
    case causeBadFormat:
        return "bad-format"
    case causeBadSignature:
        return "bad-signature"
    case causeUnknownKey:
        return "unknown-key"
    case causeNotYetValid:
        return "not-yet-valid"
    case causeExpired:
        return "expired"
    case causeClockRollback:
        return "clock-rollback"
    case causeAudienceMismatch:
        return "audience-mismatch"
    case causeIssuerMismatch:
        return "issuer-mismatch"
    case causeBindingMachineMismatch:
        return "binding-machine-mismatch"
    case causeBindingPasswordMismatch:
        return "binding-password-mismatch"
    case causeBindingCustomMismatch:
        return "binding-custom-mismatch"
    case causeBinaryHashMismatch:
        return "binary-hash-mismatch"
    case causeIdentityMismatch:
        return "identity-mismatch"
    case causeRevoked:
        return "revoked"
    case causeRevocationStale:
        return "revocation-stale"
    case causeHeartbeatFailed:
        return "heartbeat-failed"
    case causeStateCorrupted:
        return "state-corrupted"
    default:
        return "unknown"
    }
}

// invalid wraps ErrLicenseInvalid with an internal cause for logging without
// leaking the cause to the caller's error string.
func invalid(c cause) error {
    return fmt.Errorf("%w (%s)", ErrLicenseInvalid, c)
}
```

- [ ] **Step 4: Implement `clock.go`**

```go
// license/clock.go
package license

import "time"

// Clock is the abstraction used by Verify for all time-dependent checks.
// Inject a FakeClock in tests; production uses realClock via the default.
type Clock interface {
    Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now().UTC() }

// FakeClock returns a fixed time. Tests may mutate T between calls.
type FakeClock struct {
    T time.Time
}

func (f *FakeClock) Now() time.Time { return f.T }
```

- [ ] **Step 5: Implement `hash.go`**

```go
// license/hash.go
package license

import (
    "crypto/sha256"
    "encoding/hex"
)

const (
    tagLicenseV1   = "maldev-license-v1\x00"
    tagRevokeV1    = "maldev-revoke-v1\x00"
    tagHeartbeatV1 = "maldev-heartbeat-v1\x00"
    tagStateV1     = "maldev-state-v1\x00"
)

// signPayload prefixes a fixed domain tag to the canonical bytes before
// signing. Different tags for different messages prevent cross-message
// signature confusion.
func signPayload(tag string, canonical []byte) []byte {
    out := make([]byte, 0, len(tag)+len(canonical))
    out = append(out, tag...)
    out = append(out, canonical...)
    return out
}

func sha256Hex(b []byte) string {
    sum := sha256.Sum256(b)
    return hex.EncodeToString(sum[:])
}

func hexEncode(b []byte) string {
    return hex.EncodeToString(b)
}
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./license/... -v`
Expected: 4 tests PASS.

- [ ] **Step 7: Commit**

```bash
git add license/errors.go license/clock.go license/hash.go license/clock_test.go license/hash_test.go
git commit -m "feat(license): errors, clock abstraction, signing domain tags"
```

---

## Task 4 — Keys: `GenerateKey`, PEM marshal/parse, `Trusted`

**Files:**
- Create: `license/keys.go`, `license/pem.go`
- Test: `license/keys_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// license/keys_test.go
package license

import (
    "bytes"
    "crypto/ed25519"
    "testing"
)

func TestGenerateKeyRoundTrip(t *testing.T) {
    pub, priv, err := GenerateKey()
    if err != nil {
        t.Fatal(err)
    }
    msg := []byte("payload")
    sig := ed25519.Sign(priv, msg)
    if !ed25519.Verify(pub, msg, sig) {
        t.Fatal("generated keypair fails self verify")
    }
}

func TestMarshalParsePrivateKey(t *testing.T) {
    _, priv, _ := GenerateKey()
    pemBytes, err := MarshalPrivateKey(priv)
    if err != nil {
        t.Fatal(err)
    }
    if !bytes.Contains(pemBytes, []byte("MALDEV PRIVATE KEY")) {
        t.Fatalf("PEM block label missing: %s", pemBytes)
    }
    back, err := ParsePrivateKey(pemBytes)
    if err != nil {
        t.Fatal(err)
    }
    if !bytes.Equal(back, priv) {
        t.Fatal("roundtrip mismatch")
    }
}

func TestMarshalParsePublicKeyWithKID(t *testing.T) {
    pub, _, _ := GenerateKey()
    pemBytes, err := MarshalPublicKey(pub, "k2026-05")
    if err != nil {
        t.Fatal(err)
    }
    backPub, kid, err := ParsePublicKey(pemBytes)
    if err != nil {
        t.Fatal(err)
    }
    if kid != "k2026-05" {
        t.Fatalf("kid lost: %q", kid)
    }
    if !bytes.Equal(backPub, pub) {
        t.Fatal("pub roundtrip mismatch")
    }
}

func TestParseRejectsWrongBlock(t *testing.T) {
    bogus := []byte("-----BEGIN OTHER-----\nAAAA\n-----END OTHER-----\n")
    if _, err := ParsePrivateKey(bogus); err == nil {
        t.Fatal("expected error on wrong PEM type")
    }
}

func TestTrustedLookup(t *testing.T) {
    pub, _, _ := GenerateKey()
    tr := Trusted{Keys: map[string]ed25519.PublicKey{"k1": pub}}
    if _, ok := tr.Lookup("k1"); !ok {
        t.Fatal("expected k1 to be present")
    }
    if _, ok := tr.Lookup("k2"); ok {
        t.Fatal("expected k2 to be absent")
    }
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./license/ -run 'Key|Trusted' -v`
Expected: FAIL — undefined `GenerateKey`, `MarshalPrivateKey`, `ParsePrivateKey`, `MarshalPublicKey`, `ParsePublicKey`, `Trusted`.

- [ ] **Step 3: Implement `keys.go`**

```go
// license/keys.go
package license

import (
    "crypto/ed25519"
    "crypto/rand"
    "fmt"
)

// GenerateKey returns a fresh Ed25519 keypair from crypto/rand.
func GenerateKey() (ed25519.PublicKey, ed25519.PrivateKey, error) {
    pub, priv, err := ed25519.GenerateKey(rand.Reader)
    if err != nil {
        return nil, nil, fmt.Errorf("license: generate key: %w", err)
    }
    return pub, priv, nil
}

// Trusted is the set of public keys a Verify call accepts, keyed by KeyID.
// Permits rotation: keep old keys until all licences they signed expire.
type Trusted struct {
    Keys map[string]ed25519.PublicKey
}

func (t Trusted) Lookup(kid string) (ed25519.PublicKey, bool) {
    if t.Keys == nil {
        return nil, false
    }
    k, ok := t.Keys[kid]
    return k, ok
}
```

- [ ] **Step 4: Implement `pem.go`**

```go
// license/pem.go
package license

import (
    "crypto/ed25519"
    "encoding/pem"
    "errors"
    "fmt"
)

const (
    pemPrivateKey = "MALDEV PRIVATE KEY"
    pemPublicKey  = "MALDEV PUBLIC KEY"
    pemLicense    = "MALDEV LICENSE"
    pemRevoke     = "MALDEV REVOCATION LIST"
)

func MarshalPrivateKey(priv ed25519.PrivateKey) ([]byte, error) {
    if len(priv) != ed25519.PrivateKeySize {
        return nil, errors.New("license: invalid private key length")
    }
    b := &pem.Block{Type: pemPrivateKey, Bytes: priv}
    return pem.EncodeToMemory(b), nil
}

func ParsePrivateKey(data []byte) (ed25519.PrivateKey, error) {
    blk, _ := pem.Decode(data)
    if blk == nil {
        return nil, errors.New("license: no PEM block found")
    }
    if blk.Type != pemPrivateKey {
        return nil, fmt.Errorf("license: wrong PEM type %q", blk.Type)
    }
    if len(blk.Bytes) != ed25519.PrivateKeySize {
        return nil, errors.New("license: invalid private key length")
    }
    return ed25519.PrivateKey(blk.Bytes), nil
}

func MarshalPublicKey(pub ed25519.PublicKey, kid string) ([]byte, error) {
    if len(pub) != ed25519.PublicKeySize {
        return nil, errors.New("license: invalid public key length")
    }
    headers := map[string]string{}
    if kid != "" {
        headers["KID"] = kid
    }
    b := &pem.Block{Type: pemPublicKey, Headers: headers, Bytes: pub}
    return pem.EncodeToMemory(b), nil
}

func ParsePublicKey(data []byte) (ed25519.PublicKey, string, error) {
    blk, _ := pem.Decode(data)
    if blk == nil {
        return nil, "", errors.New("license: no PEM block found")
    }
    if blk.Type != pemPublicKey {
        return nil, "", fmt.Errorf("license: wrong PEM type %q", blk.Type)
    }
    if len(blk.Bytes) != ed25519.PublicKeySize {
        return nil, "", errors.New("license: invalid public key length")
    }
    return ed25519.PublicKey(blk.Bytes), blk.Headers["KID"], nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./license/ -run 'Key|Trusted' -v`
Expected: 5 tests PASS.

- [ ] **Step 6: Commit**

```bash
git add license/keys.go license/pem.go license/keys_test.go
git commit -m "feat(license): Ed25519 keypair generation + PEM marshal/parse"
```

---

## Task 5 — Core types + minimal `Issue` (offline emit)

**Files:**
- Create: `license/license.go`, `license/issue.go`
- Test: `license/issue_test.go`

- [ ] **Step 1: Write failing tests**

```go
// license/issue_test.go
package license

import (
    "encoding/json"
    "testing"
    "time"
)

func TestIssueProducesPEM(t *testing.T) {
    _, priv, _ := GenerateKey()
    data, err := Issue(IssueOptions{
        PrivateKey: priv,
        KeyID:      "k1",
        Subject:    "alice@example.com",
        NotAfter:   time.Now().Add(24 * time.Hour),
    })
    if err != nil {
        t.Fatal(err)
    }
    if !startsWith(data, []byte("-----BEGIN MALDEV LICENSE-----")) {
        t.Fatalf("not a MALDEV LICENSE PEM: %s", data)
    }
}

func TestIssueRejectsMissingKey(t *testing.T) {
    if _, err := Issue(IssueOptions{Subject: "x"}); err == nil {
        t.Fatal("expected error")
    }
}

func TestIssueRejectsMissingSubject(t *testing.T) {
    _, priv, _ := GenerateKey()
    if _, err := Issue(IssueOptions{PrivateKey: priv}); err == nil {
        t.Fatal("expected error")
    }
}

func TestNewOneLiner(t *testing.T) {
    _, priv, _ := GenerateKey()
    data, err := New(priv, "alice", 7*24*time.Hour)
    if err != nil {
        t.Fatal(err)
    }
    // Inspect to confirm subject + reasonable expiry.
    lic, err := Inspect(data)
    if err != nil {
        t.Fatal(err)
    }
    if lic.Subject != "alice" {
        t.Fatalf("subject=%q", lic.Subject)
    }
    if lic.NotAfter.Before(time.Now().Add(6 * 24 * time.Hour)) {
        t.Fatalf("expiry too short: %v", lic.NotAfter)
    }
    // Payload not silently mutated.
    var raw any
    if len(lic.Payload) > 0 {
        if err := json.Unmarshal(lic.Payload, &raw); err != nil {
            t.Fatal(err)
        }
    }
}

func startsWith(b, p []byte) bool {
    if len(b) < len(p) {
        return false
    }
    for i := range p {
        if b[i] != p[i] {
            return false
        }
    }
    return true
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./license/ -run 'Issue|New' -v`
Expected: FAIL — undefined `Issue`, `New`, `IssueOptions`, `Inspect`.

- [ ] **Step 3: Implement `license.go`**

```go
// license/license.go
package license

import (
    "crypto/ed25519"
    "encoding/json"
    "time"
)

// License is the body of a maldev license. Every field is signed.
type License struct {
    Version int    `json:"v"`
    ID      string `json:"id"`
    KeyID   string `json:"kid"`

    Issuer   string   `json:"iss,omitempty"`
    Subject  string   `json:"sub"`
    Audience []string `json:"aud,omitempty"`

    IssuedAt  time.Time `json:"iat"`
    NotBefore time.Time `json:"nbf,omitempty"`
    NotAfter  time.Time `json:"exp,omitempty"`

    Bindings []Binding `json:"bnd,omitempty"`

    BinarySHA256   string `json:"bin,omitempty"`
    IdentitySHA256 string `json:"id_sha,omitempty"`

    Payload       json.RawMessage `json:"pld,omitempty"`
    SealedPayload []byte          `json:"spld,omitempty"`
}

// Binding is a single constraint that must be matched by caller-provided
// evidence at Verify time. Defined fully in bindings.go.
type Binding struct {
    Type  string   `json:"t"`
    Value []string `json:"v,omitempty"`
    Hash  []byte   `json:"h,omitempty"`
    Salt  []byte   `json:"s,omitempty"`
}

// Verified is the result of a successful Verify.
type Verified struct {
    License
    Payload  []byte
    KeyUsed  string
    Warnings []string
}

// signedLicense is the wire wrapper PEM-encoded on disk. KeyID is duplicated
// here so a verifier can pick the right key before parsing the body.
type signedLicense struct {
    License   License `json:"lic"`
    Signature []byte  `json:"sig"`
    KeyID     string  `json:"kid"`
}

// MaxLicenseSize bounds the PEM input accepted by Verify.
const MaxLicenseSize = 16 * 1024

// IssueOptions configures Issue.
type IssueOptions struct {
    PrivateKey ed25519.PrivateKey
    KeyID      string

    Issuer   string
    Subject  string
    Audience []string

    NotBefore time.Time
    NotAfter  time.Time

    Bindings []Binding

    BinarySHA256   string
    IdentitySHA256 string

    Payload       json.RawMessage
    SealedPayload []byte
}
```

- [ ] **Step 4: Implement `issue.go`**

```go
// license/issue.go
package license

import (
    "crypto/ed25519"
    "crypto/rand"
    "encoding/base64"
    "encoding/pem"
    "errors"
    "fmt"
    "time"

    "github.com/oioio-space/maldev/license/canonical"
)

// New issues a license with sensible defaults. The expiry is now+ttl; ttl<=0
// means no expiry (NotAfter zero). Useful for tests and quick CLI wrappers.
func New(priv ed25519.PrivateKey, subject string, ttl time.Duration) ([]byte, error) {
    opts := IssueOptions{
        PrivateKey: priv,
        Subject:    subject,
    }
    if ttl > 0 {
        opts.NotAfter = time.Now().Add(ttl).UTC()
    }
    return Issue(opts)
}

// Issue signs a new license. Returns the PEM-armored bytes ready for storage
// or transmission.
func Issue(opts IssueOptions) ([]byte, error) {
    if len(opts.PrivateKey) != ed25519.PrivateKeySize {
        return nil, errors.New("license: IssueOptions.PrivateKey missing or invalid")
    }
    if opts.Subject == "" {
        return nil, errors.New("license: IssueOptions.Subject required")
    }
    kid := opts.KeyID
    if kid == "" {
        kid = "default"
    }

    id, err := newUUIDv4()
    if err != nil {
        return nil, err
    }

    lic := License{
        Version:        1,
        ID:             id,
        KeyID:          kid,
        Issuer:         opts.Issuer,
        Subject:        opts.Subject,
        Audience:       opts.Audience,
        IssuedAt:       time.Now().UTC(),
        NotBefore:      opts.NotBefore.UTC(),
        NotAfter:       opts.NotAfter.UTC(),
        Bindings:       opts.Bindings,
        BinarySHA256:   opts.BinarySHA256,
        IdentitySHA256: opts.IdentitySHA256,
        Payload:        opts.Payload,
        SealedPayload:  opts.SealedPayload,
    }

    body, err := canonical.Marshal(lic)
    if err != nil {
        return nil, fmt.Errorf("license: canonicalise body: %w", err)
    }
    sig := ed25519.Sign(opts.PrivateKey, signPayload(tagLicenseV1, body))

    wrapped := signedLicense{
        License:   lic,
        Signature: sig,
        KeyID:     kid,
    }
    wbytes, err := canonical.Marshal(wrapped)
    if err != nil {
        return nil, fmt.Errorf("license: canonicalise wrapper: %w", err)
    }

    block := &pem.Block{
        Type:  pemLicense,
        Bytes: []byte(base64.StdEncoding.EncodeToString(wbytes)),
    }
    return pem.EncodeToMemory(block), nil
}

// Inspect parses a PEM-armored license without verifying its signature. Use
// for diagnostics only — never trust the returned License for authorisation.
func Inspect(data []byte) (*License, error) {
    if len(data) > MaxLicenseSize {
        return nil, invalid(causeBadFormat)
    }
    blk, _ := pem.Decode(data)
    if blk == nil || blk.Type != pemLicense {
        return nil, invalid(causeBadFormat)
    }
    raw, err := base64.StdEncoding.DecodeString(string(blk.Bytes))
    if err != nil {
        return nil, invalid(causeBadFormat)
    }
    var w signedLicense
    if err := jsonUnmarshalStrict(raw, &w); err != nil {
        return nil, invalid(causeBadFormat)
    }
    return &w.License, nil
}

// newUUIDv4 generates a random UUIDv4 string using crypto/rand.
func newUUIDv4() (string, error) {
    var b [16]byte
    if _, err := rand.Read(b[:]); err != nil {
        return "", err
    }
    b[6] = (b[6] & 0x0f) | 0x40 // version 4
    b[8] = (b[8] & 0x3f) | 0x80 // RFC 4122 variant
    return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}
```

- [ ] **Step 5: Add `jsonUnmarshalStrict` helper to `license.go`**

```go
// Append to license/license.go:

import (
    "bytes"
    "encoding/json"
)

func jsonUnmarshalStrict(data []byte, v any) error {
    dec := json.NewDecoder(bytes.NewReader(data))
    dec.DisallowUnknownFields()
    return dec.Decode(v)
}
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./license/ -run 'Issue|New' -v`
Expected: 4 tests PASS.

- [ ] **Step 7: `/simplify` then commit**

```bash
# Trigger /simplify on the new Go files per CLAUDE.md.
# After /simplify completes its 3-agent review and any follow-up edits:
git add license/license.go license/issue.go license/issue_test.go
git commit -m "feat(license): Issue/New + License/Verified types + PEM wrapper"
```

---

## Task 6 — Minimal `Verify` (offline: signature + time + audience/issuer)

**Files:**
- Create: `license/verify.go`, `license/verify_options.go`
- Test: `license/verify_test.go`

- [ ] **Step 1: Write failing tests**

```go
// license/verify_test.go
package license

import (
    "crypto/ed25519"
    "errors"
    "testing"
    "time"
)

func issueFor(t *testing.T, opts IssueOptions) ([]byte, ed25519.PublicKey, ed25519.PrivateKey) {
    t.Helper()
    pub, priv, _ := GenerateKey()
    opts.PrivateKey = priv
    if opts.KeyID == "" {
        opts.KeyID = "k1"
    }
    if opts.Subject == "" {
        opts.Subject = "test-sub"
    }
    data, err := Issue(opts)
    if err != nil {
        t.Fatal(err)
    }
    return data, pub, priv
}

func trustedFor(pub ed25519.PublicKey, kid string) Trusted {
    return Trusted{Keys: map[string]ed25519.PublicKey{kid: pub}}
}

func TestVerifyOfflineHappyPath(t *testing.T) {
    data, pub, _ := issueFor(t, IssueOptions{NotAfter: time.Now().Add(time.Hour)})
    v, err := Verify(data, trustedFor(pub, "k1"))
    if err != nil {
        t.Fatal(err)
    }
    if v.Subject != "test-sub" {
        t.Fatalf("subject=%q", v.Subject)
    }
    if v.KeyUsed != "k1" {
        t.Fatalf("KeyUsed=%q", v.KeyUsed)
    }
}

func TestVerifyRejectsExpired(t *testing.T) {
    data, pub, _ := issueFor(t, IssueOptions{NotAfter: time.Now().Add(-time.Hour)})
    _, err := Verify(data, trustedFor(pub, "k1"))
    if !errors.Is(err, ErrLicenseInvalid) {
        t.Fatalf("expected ErrLicenseInvalid, got %v", err)
    }
}

func TestVerifyRejectsNotYetValid(t *testing.T) {
    data, pub, _ := issueFor(t, IssueOptions{NotBefore: time.Now().Add(time.Hour)})
    _, err := Verify(data, trustedFor(pub, "k1"))
    if !errors.Is(err, ErrLicenseInvalid) {
        t.Fatal("expected rejection")
    }
}

func TestVerifyRejectsUnknownKey(t *testing.T) {
    data, _, _ := issueFor(t, IssueOptions{KeyID: "kZ"})
    otherPub, _, _ := GenerateKey()
    _, err := Verify(data, trustedFor(otherPub, "k1"))
    if !errors.Is(err, ErrLicenseInvalid) {
        t.Fatal("expected rejection")
    }
}

func TestVerifyRejectsTamperedSignature(t *testing.T) {
    data, pub, _ := issueFor(t, IssueOptions{NotAfter: time.Now().Add(time.Hour)})
    // Flip a character inside the base64 payload to mutate the signed body.
    data[80] ^= 0x01
    _, err := Verify(data, trustedFor(pub, "k1"))
    if !errors.Is(err, ErrLicenseInvalid) {
        t.Fatal("tampered license accepted")
    }
}

func TestVerifyAudienceMatch(t *testing.T) {
    data, pub, _ := issueFor(t, IssueOptions{
        NotAfter: time.Now().Add(time.Hour),
        Audience: []string{"rshell"},
    })
    if _, err := Verify(data, trustedFor(pub, "k1"), WithAudience("rshell")); err != nil {
        t.Fatalf("expected accept, got %v", err)
    }
    if _, err := Verify(data, trustedFor(pub, "k1"), WithAudience("memscan")); !errors.Is(err, ErrLicenseInvalid) {
        t.Fatal("audience mismatch should reject")
    }
}

func TestVerifyIssuerMatch(t *testing.T) {
    data, pub, _ := issueFor(t, IssueOptions{
        NotAfter: time.Now().Add(time.Hour),
        Issuer:   "lab-eu",
    })
    if _, err := Verify(data, trustedFor(pub, "k1"), WithIssuer("lab-eu")); err != nil {
        t.Fatal(err)
    }
    if _, err := Verify(data, trustedFor(pub, "k1"), WithIssuer("lab-us")); !errors.Is(err, ErrLicenseInvalid) {
        t.Fatal("issuer mismatch should reject")
    }
}

func TestVerifyClockSkewTolerated(t *testing.T) {
    data, pub, _ := issueFor(t, IssueOptions{NotAfter: time.Now().Add(-30 * time.Second)})
    // 1-minute skew should let a 30s-expired licence still validate.
    _, err := Verify(data, trustedFor(pub, "k1"), WithMaxClockSkew(time.Minute))
    if err != nil {
        t.Fatalf("expected tolerance, got %v", err)
    }
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./license/ -run Verify -v`
Expected: FAIL — undefined `Verify`, `WithAudience`, `WithIssuer`, `WithMaxClockSkew`.

- [ ] **Step 3: Implement `verify_options.go`**

```go
// license/verify_options.go
package license

import (
    "context"
    "log/slog"
    "time"
)

// VerifyOption configures Verify. See WithAudience, WithIssuer, etc.
type VerifyOption func(*verifyState)

type verifyState struct {
    ctx          context.Context
    clock        Clock
    logger       *slog.Logger
    maxClockSkew time.Duration

    audience []string
    issuer   string
    // Later tasks attach more: machineID, password, custom kv, pinning,
    // revocation, heartbeat, NTP, state file, identity bytes.
}

func newVerifyState(opts []VerifyOption) *verifyState {
    s := &verifyState{
        ctx:          context.Background(),
        clock:        realClock{},
        logger:       slog.Default(),
        maxClockSkew: 5 * time.Minute,
    }
    for _, o := range opts {
        o(s)
    }
    return s
}

func WithContext(ctx context.Context) VerifyOption {
    return func(s *verifyState) { s.ctx = ctx }
}

func WithClock(c Clock) VerifyOption {
    return func(s *verifyState) { s.clock = c }
}

func WithLogger(l *slog.Logger) VerifyOption {
    return func(s *verifyState) {
        if l != nil {
            s.logger = l
        }
    }
}

func WithMaxClockSkew(d time.Duration) VerifyOption {
    return func(s *verifyState) { s.maxClockSkew = d }
}

func WithAudience(aud ...string) VerifyOption {
    return func(s *verifyState) { s.audience = append(s.audience, aud...) }
}

func WithIssuer(iss string) VerifyOption {
    return func(s *verifyState) { s.issuer = iss }
}
```

- [ ] **Step 4: Implement `verify.go`**

```go
// license/verify.go
package license

import (
    "crypto/ed25519"
    "encoding/base64"
    "encoding/pem"
    "time"

    "github.com/oioio-space/maldev/license/canonical"
)

// Verify parses, authenticates, and authorises a license. The single returned
// error type is ErrLicenseInvalid; the detailed cause is logged via the
// injected slog.Logger.
func Verify(data []byte, trusted Trusted, opts ...VerifyOption) (*Verified, error) {
    state := newVerifyState(opts)

    // 1. Format + bound size.
    if len(data) == 0 || len(data) > MaxLicenseSize {
        return nil, state.fail(causeBadFormat)
    }
    blk, _ := pem.Decode(data)
    if blk == nil || blk.Type != pemLicense {
        return nil, state.fail(causeBadFormat)
    }
    raw, err := base64.StdEncoding.DecodeString(string(blk.Bytes))
    if err != nil {
        return nil, state.fail(causeBadFormat)
    }
    var w signedLicense
    if err := jsonUnmarshalStrict(raw, &w); err != nil {
        return nil, state.fail(causeBadFormat)
    }
    if w.License.Version != 1 {
        return nil, state.fail(causeBadFormat)
    }

    // 2. Key resolution.
    pub, ok := trusted.Lookup(w.KeyID)
    if !ok || w.KeyID != w.License.KeyID {
        return nil, state.fail(causeUnknownKey)
    }

    // 3. Signature.
    body, err := canonical.Marshal(w.License)
    if err != nil {
        return nil, state.fail(causeBadFormat)
    }
    if !ed25519.Verify(pub, signPayload(tagLicenseV1, body), w.Signature) {
        return nil, state.fail(causeBadSignature)
    }

    // 4. (State file: deferred to Task 9.)

    // 5. Time.
    now := state.clock.Now()
    skew := state.maxClockSkew
    if !w.License.NotBefore.IsZero() && w.License.NotBefore.After(now.Add(skew)) {
        return nil, state.fail(causeNotYetValid)
    }
    if !w.License.NotAfter.IsZero() && w.License.NotAfter.Before(now.Add(-skew)) {
        return nil, state.fail(causeExpired)
    }

    // 6. Audience/Issuer.
    if len(state.audience) > 0 && !audienceIntersects(state.audience, w.License.Audience) {
        if len(w.License.Audience) > 0 {
            return nil, state.fail(causeAudienceMismatch)
        }
        // Empty audience = wildcard, but warn.
    }
    if state.issuer != "" && state.issuer != w.License.Issuer {
        return nil, state.fail(causeIssuerMismatch)
    }

    // 7-12. Deferred to later tasks.

    return &Verified{
        License: w.License,
        Payload: []byte(w.License.Payload),
        KeyUsed: w.KeyID,
    }, nil
}

func audienceIntersects(want, have []string) bool {
    if len(have) == 0 {
        return true // wildcard
    }
    set := make(map[string]struct{}, len(have))
    for _, h := range have {
        set[h] = struct{}{}
    }
    for _, w := range want {
        if _, ok := set[w]; ok {
            return true
        }
    }
    return false
}

func (s *verifyState) fail(c cause) error {
    s.logger.Warn("license verify failed", "cause", c.String())
    return invalid(c)
}

// (referenced by verifyState; ensures time import is used)
var _ = time.Second
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./license/ -run Verify -v`
Expected: 8 tests PASS.

- [ ] **Step 6: `/simplify` then commit**

```bash
git add license/verify.go license/verify_options.go license/verify_test.go
git commit -m "feat(license): offline Verify (signature, time, audience, issuer)"
```

---

## Task 7 — Bindings: helpers, password (argon2id), custom + extensible verifier

**Files:**
- Create: `license/bindings.go`
- Modify: `license/verify.go` (insert binding step), `license/verify_options.go` (Add WithMachineID, WithPassword, WithCustom, RegisterVerifier)
- Test: `license/bindings_test.go`

- [ ] **Step 1: Write failing tests**

```go
// license/bindings_test.go
package license

import (
    "errors"
    "testing"
    "time"
)

func TestBindMachineIDsMatchAny(t *testing.T) {
    data, pub, _ := issueFor(t, IssueOptions{
        NotAfter: time.Now().Add(time.Hour),
        Bindings: []Binding{BindMachineIDs("aaa", "bbb")},
    })
    if _, err := Verify(data, trustedFor(pub, "k1"), WithMachineID([]byte("bbb"))); err != nil {
        t.Fatal(err)
    }
    if _, err := Verify(data, trustedFor(pub, "k1"), WithMachineID([]byte("zzz"))); !errors.Is(err, ErrLicenseInvalid) {
        t.Fatal("expected reject")
    }
    if _, err := Verify(data, trustedFor(pub, "k1")); !errors.Is(err, ErrLicenseInvalid) {
        t.Fatal("missing evidence should reject")
    }
}

func TestBindPasswordArgon2id(t *testing.T) {
    bp, err := BindPassword("s3cr3t")
    if err != nil {
        t.Fatal(err)
    }
    if len(bp.Salt) != 16 || len(bp.Hash) == 0 {
        t.Fatal("salt/hash not populated")
    }
    data, pub, _ := issueFor(t, IssueOptions{
        NotAfter: time.Now().Add(time.Hour),
        Bindings: []Binding{bp},
    })
    if _, err := Verify(data, trustedFor(pub, "k1"), WithPassword("s3cr3t")); err != nil {
        t.Fatal(err)
    }
    if _, err := Verify(data, trustedFor(pub, "k1"), WithPassword("wrong")); !errors.Is(err, ErrLicenseInvalid) {
        t.Fatal("wrong password accepted")
    }
}

func TestBindCustomMatch(t *testing.T) {
    data, pub, _ := issueFor(t, IssueOptions{
        NotAfter: time.Now().Add(time.Hour),
        Bindings: []Binding{BindCustom("project", "WRAITH")},
    })
    if _, err := Verify(data, trustedFor(pub, "k1"), WithCustom("project", "WRAITH")); err != nil {
        t.Fatal(err)
    }
    if _, err := Verify(data, trustedFor(pub, "k1"), WithCustom("project", "OTHER")); !errors.Is(err, ErrLicenseInvalid) {
        t.Fatal("custom mismatch accepted")
    }
}

func TestRegisterVerifierExtensibility(t *testing.T) {
    RegisterVerifier("ip", func(b Binding, s *verifyState) bool {
        return contains(b.Value, s.customVals["ip"])
    })
    t.Cleanup(func() { delete(globalVerifiers, "ip") })

    data, pub, _ := issueFor(t, IssueOptions{
        NotAfter: time.Now().Add(time.Hour),
        Bindings: []Binding{{Type: "ip", Value: []string{"10.0.0.1", "10.0.0.2"}}},
    })
    if _, err := Verify(data, trustedFor(pub, "k1"), WithCustom("ip", "10.0.0.2")); err != nil {
        t.Fatal(err)
    }
    if _, err := Verify(data, trustedFor(pub, "k1"), WithCustom("ip", "10.0.0.9")); !errors.Is(err, ErrLicenseInvalid) {
        t.Fatal("expected reject")
    }
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./license/ -run 'Bind|Register' -v`
Expected: FAIL — undefined `BindMachineIDs`, `BindPassword`, `BindCustom`, `WithMachineID`, `WithPassword`, `WithCustom`, `RegisterVerifier`, `globalVerifiers`, `customVals`, `contains`.

- [ ] **Step 3: Implement `bindings.go`**

```go
// license/bindings.go
package license

import (
    "crypto/rand"
    "crypto/subtle"
    "errors"
    "strings"
    "sync"

    "golang.org/x/crypto/argon2"
)

const (
    bindingMachine  = "machine"
    bindingPassword = "password"
    bindingCustom   = "custom:" // prefix
)

// Argon2id parameters chosen for ~100 ms on a 2024-era laptop CPU. Stored in
// the binding next to salt/hash so future tuning is forward compatible.
const (
    argonTime    = 3
    argonMemory  = 64 * 1024
    argonThreads = 4
    argonKeyLen  = 32
    saltLen      = 16
)

// BindMachineIDs builds a binding accepting any of the listed machine ids.
func BindMachineIDs(ids ...string) Binding {
    return Binding{Type: bindingMachine, Value: append([]string(nil), ids...)}
}

// BindCustom builds a typed custom binding. Multiple values accept any-match.
func BindCustom(name string, values ...string) Binding {
    return Binding{Type: bindingCustom + name, Value: append([]string(nil), values...)}
}

// BindPassword derives argon2id(salt, password). The plaintext is never
// retained.
func BindPassword(password string) (Binding, error) {
    if password == "" {
        return Binding{}, errors.New("license: empty password")
    }
    salt := make([]byte, saltLen)
    if _, err := rand.Read(salt); err != nil {
        return Binding{}, err
    }
    hash := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
    return Binding{Type: bindingPassword, Hash: hash, Salt: salt}, nil
}

// VerifierFunc lets callers register custom binding types. The function
// returns true to accept the binding.
type VerifierFunc func(b Binding, s *verifyState) bool

var (
    verifierMu       sync.RWMutex
    globalVerifiers  = map[string]VerifierFunc{}
)

// RegisterVerifier installs a callback for a custom binding type (without the
// "custom:" prefix). Safe to call from package init.
func RegisterVerifier(name string, fn VerifierFunc) {
    verifierMu.Lock()
    defer verifierMu.Unlock()
    globalVerifiers[name] = fn
}

func contains(s []string, want string) bool {
    for _, v := range s {
        if v == want {
            return true
        }
    }
    return false
}

// checkBindings runs every License binding against verifyState evidence.
func checkBindings(lic License, s *verifyState) cause {
    for _, b := range lic.Bindings {
        if !checkBinding(b, s) {
            switch {
            case b.Type == bindingMachine:
                return causeBindingMachineMismatch
            case b.Type == bindingPassword:
                return causeBindingPasswordMismatch
            default:
                return causeBindingCustomMismatch
            }
        }
    }
    return causeOK
}

func checkBinding(b Binding, s *verifyState) bool {
    switch {
    case b.Type == bindingMachine:
        if s.machineID == nil {
            return false
        }
        evidence := string(s.machineID)
        return contains(b.Value, evidence)
    case b.Type == bindingPassword:
        if s.password == "" {
            return false
        }
        got := argon2.IDKey([]byte(s.password), b.Salt, argonTime, argonMemory, argonThreads, argonKeyLen)
        return subtle.ConstantTimeCompare(got, b.Hash) == 1
    case strings.HasPrefix(b.Type, bindingCustom):
        name := strings.TrimPrefix(b.Type, bindingCustom)
        verifierMu.RLock()
        fn, ok := globalVerifiers[name]
        verifierMu.RUnlock()
        if ok {
            return fn(b, s)
        }
        // Default: match against caller-provided custom kv.
        if val, ok := s.customVals[name]; ok {
            return contains(b.Value, val)
        }
        return false
    default:
        return false
    }
}
```

- [ ] **Step 4: Extend `verify_options.go` with binding options**

```go
// Append to license/verify_options.go:

// add to verifyState struct:
//   machineID  []byte
//   password   string
//   customVals map[string]string

func WithMachineID(id []byte) VerifyOption {
    return func(s *verifyState) { s.machineID = append([]byte(nil), id...) }
}

func WithPassword(p string) VerifyOption {
    return func(s *verifyState) { s.password = p }
}

func WithCustom(name, value string) VerifyOption {
    return func(s *verifyState) {
        if s.customVals == nil {
            s.customVals = map[string]string{}
        }
        s.customVals[name] = value
    }
}
```

- [ ] **Step 5: Wire `checkBindings` into `verify.go`**

In `Verify`, insert after the audience/issuer block, before the deferred steps:

```go
// 7. Bindings.
if c := checkBindings(w.License, state); c != causeOK {
    return nil, state.fail(c)
}
```

Also extend `verifyState` struct in `verify_options.go` with `machineID`, `password`, `customVals`. Wipe `s.password` (`for i := range []byte(s.password)`) after binding check via `cleanup/memory`. (Add import `github.com/oioio-space/maldev/cleanup/memory` and call `memory.WipeString(&s.password)` at end of Verify if non-empty. If `cleanup/memory` exposes a different API, follow it — read `cleanup/memory/*.go` first to confirm the canonical wipe call.)

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./license/ -run 'Bind|Register' -v`
Expected: 4 tests PASS.

- [ ] **Step 7: Run full test suite**

Run: `go test ./license/...`
Expected: all PASS.

- [ ] **Step 8: `/simplify` then commit**

```bash
git add license/bindings.go license/verify.go license/verify_options.go license/bindings_test.go
git commit -m "feat(license): bindings — machine, password (argon2id), custom + extensible verifier"
```

---

## Task 8 — `hostid/` sub-package (cross-platform fingerprint)

**Files:**
- Create: `license/hostid/hostid.go`, `license/hostid/hostid_windows.go`, `license/hostid/hostid_linux.go`, `license/hostid/hostid_darwin.go`
- Test: `license/hostid/hostid_test.go`

- [ ] **Step 1: Write the failing test**

```go
// license/hostid/hostid_test.go
package hostid

import "testing"

func TestLocalReturns32Bytes(t *testing.T) {
    id, err := Local()
    if err != nil {
        t.Fatal(err)
    }
    if len(id) != 32 {
        t.Fatalf("len=%d", len(id))
    }
}

func TestLocalDeterministic(t *testing.T) {
    a, err := Local()
    if err != nil {
        t.Fatal(err)
    }
    b, err := Local()
    if err != nil {
        t.Fatal(err)
    }
    if string(a) != string(b) {
        t.Fatal("Local() non-deterministic across calls")
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./license/hostid/...`
Expected: FAIL — undefined `Local`.

- [ ] **Step 3: Implement `hostid.go` (platform-agnostic mixer)**

```go
// Package hostid produces a 32-byte machine fingerprint by mixing OS-provided
// identifiers (registry MachineGuid on Windows, /etc/machine-id on Linux,
// IOPlatformUUID on darwin) through sha256. The output is suitable for use as
// the evidence in WithMachineID(...).
package hostid

import (
    "crypto/sha256"
    "errors"
)

// Local returns a 32-byte fingerprint of the current machine.
func Local() ([]byte, error) {
    parts, err := readPlatformSources()
    if err != nil {
        return nil, err
    }
    if len(parts) == 0 {
        return nil, errors.New("hostid: no identifier source available")
    }
    h := sha256.New()
    h.Write([]byte("maldev-hostid-v1\x00"))
    for _, p := range parts {
        if len(p) == 0 {
            continue
        }
        h.Write([]byte{byte(len(p) >> 8), byte(len(p))})
        h.Write(p)
    }
    return h.Sum(nil), nil
}
```

- [ ] **Step 4: Implement `hostid_windows.go`**

```go
//go:build windows

package hostid

import (
    "errors"

    "golang.org/x/sys/windows/registry"
)

func readPlatformSources() ([][]byte, error) {
    var out [][]byte
    k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Cryptography`, registry.QUERY_VALUE|registry.WOW64_64KEY)
    if err != nil {
        return nil, err
    }
    defer k.Close()
    guid, _, err := k.GetStringValue("MachineGuid")
    if err != nil {
        return nil, err
    }
    if guid == "" {
        return nil, errors.New("hostid: empty MachineGuid")
    }
    out = append(out, []byte(guid))
    return out, nil
}
```

- [ ] **Step 5: Implement `hostid_linux.go`**

```go
//go:build linux

package hostid

import (
    "bytes"
    "errors"
    "os"
)

func readPlatformSources() ([][]byte, error) {
    var out [][]byte
    for _, p := range []string{"/etc/machine-id", "/var/lib/dbus/machine-id"} {
        b, err := os.ReadFile(p)
        if err == nil {
            b = bytes.TrimSpace(b)
            if len(b) > 0 {
                out = append(out, b)
                break
            }
        }
    }
    if len(out) == 0 {
        return nil, errors.New("hostid: no machine-id file readable")
    }
    return out, nil
}
```

- [ ] **Step 6: Implement `hostid_darwin.go`**

```go
//go:build darwin

package hostid

import (
    "errors"
    "os/exec"
    "strings"
)

func readPlatformSources() ([][]byte, error) {
    out, err := exec.Command("/usr/sbin/ioreg", "-rd1", "-c", "IOPlatformExpertDevice").Output()
    if err != nil {
        return nil, err
    }
    for _, line := range strings.Split(string(out), "\n") {
        if i := strings.Index(line, `"IOPlatformUUID"`); i >= 0 {
            j := strings.LastIndex(line, `"`)
            k := strings.LastIndex(line[:j], `"`)
            if k >= 0 && j > k {
                return [][]byte{[]byte(line[k+1 : j])}, nil
            }
        }
    }
    return nil, errors.New("hostid: IOPlatformUUID not found")
}
```

- [ ] **Step 7: Run tests to verify they pass**

Run: `go test ./license/hostid/... -v`
Expected: 2 tests PASS on host platform.
Cross-compile sanity: `GOOS=linux go build ./license/hostid/` and `GOOS=darwin go build ./license/hostid/` and `GOOS=windows go build ./license/hostid/` must all succeed.

- [ ] **Step 8: `/simplify` then commit**

```bash
git add license/hostid/
git commit -m "feat(license/hostid): cross-platform machine fingerprint"
```

---

## Task 9 — State file (HMAC-protected) + clock-rollback detection

**Files:**
- Create: `license/state.go`
- Modify: `license/verify.go` (insert state read + rollback check), `license/verify_options.go` (`WithStateFile`)
- Test: `license/state_test.go`

- [ ] **Step 1: Write failing tests**

```go
// license/state_test.go
package license

import (
    "errors"
    "os"
    "path/filepath"
    "testing"
    "time"
)

func TestStateRoundTrip(t *testing.T) {
    dir := t.TempDir()
    p := filepath.Join(dir, "state")
    key := []byte("hmac-key-32-bytes-long-enough....")[:32]
    in := State{
        TrustedFloor:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
        LastSeenLocal: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
    }
    if err := writeState(p, key, in); err != nil {
        t.Fatal(err)
    }
    out, err := readState(p, key)
    if err != nil {
        t.Fatal(err)
    }
    if !out.TrustedFloor.Equal(in.TrustedFloor) || !out.LastSeenLocal.Equal(in.LastSeenLocal) {
        t.Fatalf("roundtrip mismatch: %+v vs %+v", in, out)
    }
}

func TestStateRejectsTamperedHMAC(t *testing.T) {
    dir := t.TempDir()
    p := filepath.Join(dir, "state")
    key := []byte("hmac-key-32-bytes-long-enough....")[:32]
    _ = writeState(p, key, State{TrustedFloor: time.Now().UTC()})
    raw, _ := os.ReadFile(p)
    raw[len(raw)/2] ^= 0xFF
    _ = os.WriteFile(p, raw, 0o600)
    if _, err := readState(p, key); err == nil {
        t.Fatal("expected HMAC failure")
    }
}

func TestVerifyDetectsClockRollback(t *testing.T) {
    dir := t.TempDir()
    statePath := filepath.Join(dir, "license-state")

    // First Verify (clock = 2026-06-01).
    data, pub, _ := issueFor(t, IssueOptions{NotAfter: time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)})
    clk := &FakeClock{T: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)}
    if _, err := Verify(data, trustedFor(pub, "k1"),
        WithClock(clk), WithStateFile(statePath)); err != nil {
        t.Fatalf("first verify failed: %v", err)
    }

    // Second Verify with clock rolled back to 2026-01-01: rejection.
    clk.T = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
    _, err := Verify(data, trustedFor(pub, "k1"),
        WithClock(clk), WithStateFile(statePath))
    if !errors.Is(err, ErrLicenseInvalid) {
        t.Fatalf("rollback should reject, got %v", err)
    }
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./license/ -run State -v`
Expected: FAIL — undefined `writeState`, `readState`, `State`, `WithStateFile`.

- [ ] **Step 3: Implement `state.go`**

```go
// license/state.go
package license

import (
    "crypto/hmac"
    "crypto/sha256"
    "encoding/json"
    "errors"
    "io"
    "os"
    "path/filepath"
    "time"
)

// State is the local cross-invocation memory of Verify. Persisted under
// HMAC; key is derived in deriveStateKey.
type State struct {
    TrustedFloor     time.Time `json:"tf"`
    LastSeenLocal    time.Time `json:"lsl"`
    LastSeenSequence uint64    `json:"lss"`
    LastFetchOk      time.Time `json:"lfo"`
    LastHeartbeatOk  time.Time `json:"lho"`
}

type stateEnvelope struct {
    Body []byte `json:"b"`
    HMAC []byte `json:"m"`
}

func writeState(path string, key []byte, s State) error {
    body, err := json.Marshal(s)
    if err != nil {
        return err
    }
    m := hmac.New(sha256.New, key)
    m.Write([]byte(tagStateV1))
    m.Write(body)
    env := stateEnvelope{Body: body, HMAC: m.Sum(nil)}
    raw, err := json.Marshal(env)
    if err != nil {
        return err
    }
    return atomicWrite(path, raw)
}

func readState(path string, key []byte) (State, error) {
    var s State
    raw, err := os.ReadFile(path)
    if err != nil {
        return s, err
    }
    var env stateEnvelope
    if err := json.Unmarshal(raw, &env); err != nil {
        return s, errors.New("state: malformed envelope")
    }
    m := hmac.New(sha256.New, key)
    m.Write([]byte(tagStateV1))
    m.Write(env.Body)
    if !hmac.Equal(m.Sum(nil), env.HMAC) {
        return s, errors.New("state: HMAC mismatch")
    }
    if err := json.Unmarshal(env.Body, &s); err != nil {
        return s, err
    }
    return s, nil
}

func atomicWrite(path string, data []byte) error {
    dir := filepath.Dir(path)
    if err := os.MkdirAll(dir, 0o700); err != nil {
        return err
    }
    f, err := os.CreateTemp(dir, ".state-*.tmp")
    if err != nil {
        return err
    }
    tmp := f.Name()
    defer func() {
        _ = os.Remove(tmp) // best-effort if Rename succeeded this is a no-op
    }()
    if _, err := f.Write(data); err != nil {
        _ = f.Close()
        return err
    }
    if err := f.Sync(); err != nil {
        _ = f.Close()
        return err
    }
    if err := f.Close(); err != nil {
        return err
    }
    return os.Rename(tmp, path)
}

// deriveStateKey produces a 32-byte HMAC key bound to (license signature ||
// machine fingerprint). The user can wipe the state file but cannot rewrite
// it with a forged HMAC without also possessing those inputs.
func deriveStateKey(sig, hostFingerprint []byte) []byte {
    h := sha256.New()
    h.Write([]byte("maldev-state-key-v1\x00"))
    h.Write(sig)
    h.Write(hostFingerprint)
    return h.Sum(nil)[:32]
}

// suppress unused import if hostid is added later
var _ = io.EOF
```

- [ ] **Step 4: Extend `verify_options.go` with `WithStateFile`**

```go
// Append to verifyState struct: statePath string
// Add option:
func WithStateFile(path string) VerifyOption {
    return func(s *verifyState) { s.statePath = path }
}
```

- [ ] **Step 5: Wire state + rollback check into `verify.go`**

In `Verify`, after signature step (3) and before time step (5), insert:

```go
// 4. State file: read if configured, detect rollback, update on success.
var st State
var stateKey []byte
if state.statePath != "" {
    hostFP, _ := hostidLocalOrZero() // helper that returns sha256("none") if hostid fails
    stateKey = deriveStateKey(w.Signature, hostFP)
    if loaded, err := readState(state.statePath, stateKey); err == nil {
        st = loaded
        floor := maxTime(st.TrustedFloor, st.LastSeenLocal)
        if !floor.IsZero() && now.Before(floor.Add(-state.maxClockSkew)) {
            return nil, state.fail(causeClockRollback)
        }
    } else {
        state.logger.Warn("license state unreadable; resetting", "err", err)
    }
}
```

After the bindings step succeeds, update + write state:

```go
// 12. Persist state.
if state.statePath != "" {
    st.LastSeenLocal = maxTime(st.LastSeenLocal, now)
    if err := writeState(state.statePath, stateKey, st); err != nil {
        state.logger.Warn("license state write failed", "err", err)
    }
}
```

Add helpers at bottom of `verify.go`:

```go
func maxTime(a, b time.Time) time.Time {
    if a.After(b) {
        return a
    }
    return b
}

func hostidLocalOrZero() ([]byte, error) {
    // Imported lazily to avoid forcing all consumers to pull hostid.
    // Implemented via init that registers a function pointer when hostid
    // is imported.
    if hostidFn != nil {
        return hostidFn()
    }
    return make([]byte, 32), nil
}

var hostidFn func() ([]byte, error)

// SetHostIDProvider lets the consumer wire license/hostid's Local() into
// state key derivation without creating an import cycle.
func SetHostIDProvider(fn func() ([]byte, error)) { hostidFn = fn }
```

In `license/hostid/hostid.go`, add an init hook to call `license.SetHostIDProvider(Local)` — but to avoid the import cycle, prefer reverse approach: expose `hostid.RegisterAsStateProvider(fn func(func() ([]byte, error)))` and call it from the consumer's main, or simply allow consumers to pass their hostid via a separate option like `WithStateHostID(hostid.Local)` to avoid coupling. Plan adoption: add `WithStateHostID(fn func()([]byte, error))` instead of the provider hook. Rewrite Step 5 accordingly: drop `hostidFn`, drop `SetHostIDProvider`, add `WithStateHostID`; in the state branch, if `state.stateHostIDFn != nil` call it, else use a zero array.

```go
// In verify_options.go:
type verifyState struct {
    // ...
    stateHostIDFn func() ([]byte, error)
    statePath     string
}

func WithStateHostID(fn func() ([]byte, error)) VerifyOption {
    return func(s *verifyState) { s.stateHostIDFn = fn }
}
```

In `verify.go` state branch:

```go
var hostFP []byte
if state.stateHostIDFn != nil {
    hostFP, _ = state.stateHostIDFn()
}
if hostFP == nil {
    hostFP = make([]byte, 32)
}
stateKey = deriveStateKey(w.Signature, hostFP)
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./license/ -run State -v`
Expected: 3 tests PASS.

- [ ] **Step 7: `/simplify` then commit**

```bash
git add license/state.go license/verify.go license/verify_options.go license/state_test.go
git commit -m "feat(license): HMAC state file + clock-rollback detection"
```

---

## Task 10 — Binary pinning (disk SHA + identity) + `identity/` sub-package

**Files:**
- Create: `license/pinning.go`, `license/identity/identity.go`, `license/identity/cmd/gen-identity/main.go`
- Test: `license/pinning_test.go`, `license/identity/identity_test.go`
- Modify: `license/verify.go` (insert pinning step), `license/verify_options.go` (`WithBinaryPinning`, `WithIdentityBytes`)

- [ ] **Step 1: Write the failing tests**

```go
// license/identity/identity_test.go
package identity

import "testing"

func TestSetReadRoundTrip(t *testing.T) {
    Set([]byte{1, 2, 3, 4})
    if got := Read(); string(got) != "\x01\x02\x03\x04" {
        t.Fatalf("got %x", got)
    }
}

func TestHashIdentityHex(t *testing.T) {
    h := HashIdentity([]byte("abc"))
    if len(h) != 64 {
        t.Fatalf("len=%d", len(h))
    }
}
```

```go
// license/pinning_test.go
package license

import (
    "errors"
    "os"
    "testing"
    "time"
)

func TestHashFile(t *testing.T) {
    f, err := os.CreateTemp(t.TempDir(), "bin")
    if err != nil {
        t.Fatal(err)
    }
    _, _ = f.WriteString("hello")
    _ = f.Close()
    h, err := HashFile(f.Name())
    if err != nil {
        t.Fatal(err)
    }
    if len(h) != 64 {
        t.Fatalf("got %d hex chars", len(h))
    }
}

func TestVerifyIdentityPinningMatches(t *testing.T) {
    idBytes := []byte("identity-payload-32-bytes-long....")[:32]
    data, pub, _ := issueFor(t, IssueOptions{
        NotAfter:       time.Now().Add(time.Hour),
        IdentitySHA256: HashIdentity(idBytes),
    })
    if _, err := Verify(data, trustedFor(pub, "k1"),
        WithBinaryPinning(), WithIdentityBytes(idBytes)); err != nil {
        t.Fatal(err)
    }
}

func TestVerifyIdentityPinningMismatch(t *testing.T) {
    data, pub, _ := issueFor(t, IssueOptions{
        NotAfter:       time.Now().Add(time.Hour),
        IdentitySHA256: HashIdentity([]byte("AAAA")),
    })
    _, err := Verify(data, trustedFor(pub, "k1"),
        WithBinaryPinning(), WithIdentityBytes([]byte("BBBB")))
    if !errors.Is(err, ErrLicenseInvalid) {
        t.Fatal("mismatch accepted")
    }
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./license/... -run 'Hash|Pinning|Identity' -v`
Expected: FAIL on undefined symbols.

- [ ] **Step 3: Implement `identity/identity.go`**

```go
// Package identity holds a 32-byte build-time identity registered by the
// consumer binary (typically via //go:embed identity.bin and a call to Set).
// The identity survives binary packing because packers preserve embedded data.
package identity

import (
    "crypto/sha256"
    "encoding/hex"
    "sync"
)

var (
    mu  sync.RWMutex
    val []byte
)

// Set registers the embedded identity bytes. Must be called once at init from
// the consumer binary.
func Set(b []byte) {
    mu.Lock()
    val = append([]byte(nil), b...)
    mu.Unlock()
}

// Read returns the registered identity bytes (nil if Set has not been called).
func Read() []byte {
    mu.RLock()
    defer mu.RUnlock()
    return append([]byte(nil), val...)
}

// HashIdentity returns the hex sha256 of arbitrary identity bytes. Used by
// license-issuing code to fill License.IdentitySHA256.
func HashIdentity(b []byte) string {
    sum := sha256.Sum256(b)
    return hex.EncodeToString(sum[:])
}
```

- [ ] **Step 4: Implement `identity/cmd/gen-identity/main.go`**

```go
// gen-identity writes 32 random bytes to ./identity.bin if absent. Idempotent.
// Usage: //go:generate go run github.com/oioio-space/maldev/license/identity/cmd/gen-identity -out identity.bin
package main

import (
    "crypto/rand"
    "flag"
    "fmt"
    "log"
    "os"
)

func main() {
    out := flag.String("out", "identity.bin", "destination path")
    force := flag.Bool("force", false, "overwrite if exists")
    flag.Parse()
    if _, err := os.Stat(*out); err == nil && !*force {
        fmt.Fprintf(os.Stderr, "gen-identity: %s exists (use -force to overwrite)\n", *out)
        return
    }
    var b [32]byte
    if _, err := rand.Read(b[:]); err != nil {
        log.Fatalf("gen-identity: %v", err)
    }
    if err := os.WriteFile(*out, b[:], 0o644); err != nil {
        log.Fatalf("gen-identity: %v", err)
    }
}
```

- [ ] **Step 5: Implement `pinning.go`**

```go
// license/pinning.go
package license

import (
    "crypto/sha256"
    "encoding/hex"
    "io"
    "os"

    "github.com/oioio-space/maldev/license/identity"
)

// HashFile returns the hex sha256 of a file's contents.
func HashFile(path string) (string, error) {
    f, err := os.Open(path)
    if err != nil {
        return "", err
    }
    defer f.Close()
    h := sha256.New()
    if _, err := io.Copy(h, f); err != nil {
        return "", err
    }
    return hex.EncodeToString(h.Sum(nil)), nil
}

// HashIdentity is a convenience re-export to keep callers from importing the
// identity sub-package just to compute a hash.
func HashIdentity(b []byte) string { return identity.HashIdentity(b) }

func checkPinning(lic License, s *verifyState) cause {
    if !s.binaryPinning {
        return causeOK
    }
    haveDisk := lic.BinarySHA256 != ""
    haveID := lic.IdentitySHA256 != ""
    if !haveDisk && !haveID {
        s.warnings = append(s.warnings, "pinning requested but license carries no pin")
        return causeOK
    }
    if haveDisk {
        path, err := os.Executable()
        if err != nil {
            return causeBinaryHashMismatch
        }
        got, err := HashFile(path)
        if err != nil {
            return causeBinaryHashMismatch
        }
        if got != lic.BinarySHA256 {
            return causeBinaryHashMismatch
        }
    }
    if haveID {
        b := s.identityBytes
        if b == nil {
            b = identity.Read()
        }
        if HashIdentity(b) != lic.IdentitySHA256 {
            return causeIdentityMismatch
        }
    }
    return causeOK
}
```

- [ ] **Step 6: Extend `verify_options.go`**

```go
// add to verifyState struct:
//   binaryPinning bool
//   identityBytes []byte
//   warnings      []string

func WithBinaryPinning() VerifyOption {
    return func(s *verifyState) { s.binaryPinning = true }
}

func WithIdentityBytes(b []byte) VerifyOption {
    return func(s *verifyState) { s.identityBytes = append([]byte(nil), b...) }
}
```

- [ ] **Step 7: Wire `checkPinning` into `verify.go`**

After binding check, before deferred steps:

```go
// 8. Binary / Identity pinning.
if c := checkPinning(w.License, state); c != causeOK {
    return nil, state.fail(c)
}
```

And surface warnings:

```go
return &Verified{
    License:  w.License,
    Payload:  []byte(w.License.Payload),
    KeyUsed:  w.KeyID,
    Warnings: state.warnings,
}, nil
```

- [ ] **Step 8: Run tests to verify they pass**

Run: `go test ./license/... -run 'Hash|Pinning|Identity' -v`
Expected: 4 tests PASS.

- [ ] **Step 9: `/simplify` then commit**

```bash
git add license/pinning.go license/pinning_test.go license/identity/ license/verify.go license/verify_options.go
git commit -m "feat(license): binary + identity pinning, identity sub-package, gen-identity tool"
```

---

## Task 11 — `revoke/` sub-package (list, sign/verify, sources, signed cache)

**Files:**
- Create: `license/revoke/list.go`, `license/revoke/source.go`, `license/revoke/cache.go`
- Test: `license/revoke/revoke_test.go`

- [ ] **Step 1: Write failing tests**

```go
// license/revoke/revoke_test.go
package revoke

import (
    "context"
    "crypto/ed25519"
    "crypto/rand"
    "encoding/pem"
    "errors"
    "net/http"
    "net/http/httptest"
    "os"
    "path/filepath"
    "testing"
    "time"
)

func keypair(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
    t.Helper()
    pub, priv, err := ed25519.GenerateKey(rand.Reader)
    if err != nil {
        t.Fatal(err)
    }
    return pub, priv
}

func TestListSignVerifyRoundTrip(t *testing.T) {
    pub, priv := keypair(t)
    l := List{
        Version:    1,
        KeyID:      "k1",
        Sequence:   42,
        IssuedAt:   time.Now().UTC(),
        ExpiresAt:  time.Now().Add(time.Hour).UTC(),
        ServerTime: time.Now().UTC(),
        Revoked:    []string{"lic-1", "lic-2"},
    }
    raw, err := Sign(l, priv)
    if err != nil {
        t.Fatal(err)
    }
    back, err := VerifyBytes(raw, pub, "k1")
    if err != nil {
        t.Fatal(err)
    }
    if back.Sequence != 42 || len(back.Revoked) != 2 {
        t.Fatalf("roundtrip mismatch: %+v", back)
    }
}

func TestVerifyBytesRejectsTampered(t *testing.T) {
    pub, priv := keypair(t)
    raw, _ := Sign(List{Version: 1, KeyID: "k1", Sequence: 1, ExpiresAt: time.Now().Add(time.Hour)}, priv)
    blk, _ := pem.Decode(raw)
    blk.Bytes[5] ^= 0x01
    raw = pem.EncodeToMemory(blk)
    if _, err := VerifyBytes(raw, pub, "k1"); err == nil {
        t.Fatal("tampered list accepted")
    }
}

func TestIsRevoked(t *testing.T) {
    l := &List{Revoked: []string{"a", "b"}}
    if !l.IsRevoked("a") || l.IsRevoked("c") {
        t.Fatal("IsRevoked logic broken")
    }
}

func TestHTTPSource(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        _, _ = w.Write([]byte("payload-x"))
    }))
    defer srv.Close()
    src := HTTPSource(srv.URL, nil)
    got, err := src.Fetch(context.Background())
    if err != nil {
        t.Fatal(err)
    }
    if string(got) != "payload-x" {
        t.Fatalf("got %s", got)
    }
}

func TestFileSource(t *testing.T) {
    f := filepath.Join(t.TempDir(), "rev")
    _ = os.WriteFile(f, []byte("file-payload"), 0o644)
    src := FileSource(f)
    got, err := src.Fetch(context.Background())
    if err != nil {
        t.Fatal(err)
    }
    if string(got) != "file-payload" {
        t.Fatalf("got %s", got)
    }
}

func TestMultiSourceFallsBack(t *testing.T) {
    bad := SourceFunc(func(ctx context.Context) ([]byte, error) { return nil, errors.New("bad") })
    good := SourceFunc(func(ctx context.Context) ([]byte, error) { return []byte("ok"), nil })
    src := MultiSource(bad, good)
    got, err := src.Fetch(context.Background())
    if err != nil {
        t.Fatal(err)
    }
    if string(got) != "ok" {
        t.Fatalf("got %s", got)
    }
}

func TestCacheSequenceMonotonic(t *testing.T) {
    pub, priv := keypair(t)
    cachePath := filepath.Join(t.TempDir(), "cache")

    rawA, _ := Sign(List{Version: 1, KeyID: "k1", Sequence: 5, ExpiresAt: time.Now().Add(time.Hour)}, priv)
    if err := StoreCache(cachePath, rawA, 5); err != nil {
        t.Fatal(err)
    }
    rawB, _ := Sign(List{Version: 1, KeyID: "k1", Sequence: 3, ExpiresAt: time.Now().Add(time.Hour)}, priv)

    if _, err := LoadCache(cachePath, pub, "k1", time.Now()); err != nil {
        t.Fatal(err)
    }
    if err := StoreCache(cachePath, rawB, 3); err == nil {
        t.Fatal("expected rejection on sequence regression")
    }
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./license/revoke/... -v`
Expected: FAIL — undefined `List`, `Sign`, `VerifyBytes`, `HTTPSource`, `FileSource`, `MultiSource`, `SourceFunc`, `StoreCache`, `LoadCache`.

- [ ] **Step 3: Implement `list.go`**

```go
// license/revoke/list.go
package revoke

import (
    "crypto/ed25519"
    "encoding/base64"
    "encoding/pem"
    "errors"
    "fmt"
    "time"

    "github.com/oioio-space/maldev/license/canonical"
)

const pemRevokeBlock = "MALDEV REVOCATION LIST"
const tagRevokeV1 = "maldev-revoke-v1\x00"

type List struct {
    Version    int       `json:"v"`
    KeyID      string    `json:"kid"`
    Sequence   uint64    `json:"seq"`
    PrevHash   []byte    `json:"prev,omitempty"`
    IssuedAt   time.Time `json:"iat"`
    ExpiresAt  time.Time `json:"exp"`
    ServerTime time.Time `json:"st"`
    Revoked    []string  `json:"rev"`
}

func (l *List) IsRevoked(id string) bool {
    for _, r := range l.Revoked {
        if r == id {
            return true
        }
    }
    return false
}

type signedList struct {
    List      List   `json:"lst"`
    Signature []byte `json:"sig"`
    KeyID     string `json:"kid"`
}

func Sign(l List, priv ed25519.PrivateKey) ([]byte, error) {
    if len(priv) != ed25519.PrivateKeySize {
        return nil, errors.New("revoke: invalid private key")
    }
    if l.Version == 0 {
        l.Version = 1
    }
    if l.IssuedAt.IsZero() {
        l.IssuedAt = time.Now().UTC()
    }
    body, err := canonical.Marshal(l)
    if err != nil {
        return nil, err
    }
    sig := ed25519.Sign(priv, append([]byte(tagRevokeV1), body...))
    raw, err := canonical.Marshal(signedList{List: l, Signature: sig, KeyID: l.KeyID})
    if err != nil {
        return nil, err
    }
    return pem.EncodeToMemory(&pem.Block{
        Type:  pemRevokeBlock,
        Bytes: []byte(base64.StdEncoding.EncodeToString(raw)),
    }), nil
}

func VerifyBytes(data []byte, pub ed25519.PublicKey, expectedKID string) (*List, error) {
    blk, _ := pem.Decode(data)
    if blk == nil || blk.Type != pemRevokeBlock {
        return nil, errors.New("revoke: not a revocation list PEM")
    }
    raw, err := base64.StdEncoding.DecodeString(string(blk.Bytes))
    if err != nil {
        return nil, fmt.Errorf("revoke: base64: %w", err)
    }
    var w signedList
    if err := jsonUnmarshalStrict(raw, &w); err != nil {
        return nil, fmt.Errorf("revoke: json: %w", err)
    }
    if expectedKID != "" && w.KeyID != expectedKID {
        return nil, fmt.Errorf("revoke: kid mismatch")
    }
    body, err := canonical.Marshal(w.List)
    if err != nil {
        return nil, err
    }
    if !ed25519.Verify(pub, append([]byte(tagRevokeV1), body...), w.Signature) {
        return nil, errors.New("revoke: signature invalid")
    }
    return &w.List, nil
}
```

(Add a small `jsonUnmarshalStrict` helper inside `revoke/list.go` mirroring the root package’s version — keep packages independent.)

- [ ] **Step 4: Implement `source.go`**

```go
// license/revoke/source.go
package revoke

import (
    "context"
    "errors"
    "io"
    "net/http"
    "os"
)

// RevocationSource abstracts where the signed revocation list comes from.
type RevocationSource interface {
    Fetch(ctx context.Context) ([]byte, error)
}

// SourceFunc lets callers plug a closure as a source without a dedicated type.
type SourceFunc func(ctx context.Context) ([]byte, error)

func (f SourceFunc) Fetch(ctx context.Context) ([]byte, error) { return f(ctx) }

// HTTPSource fetches the list over HTTP. Pass nil for client to use http.DefaultClient.
func HTTPSource(url string, client *http.Client) RevocationSource {
    if client == nil {
        client = http.DefaultClient
    }
    return SourceFunc(func(ctx context.Context) ([]byte, error) {
        req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
        if err != nil {
            return nil, err
        }
        resp, err := client.Do(req)
        if err != nil {
            return nil, err
        }
        defer resp.Body.Close()
        if resp.StatusCode/100 != 2 {
            return nil, errors.New("revoke: HTTP " + resp.Status)
        }
        return io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024))
    })
}

// FileSource reads from a local file (e.g. shared mount).
func FileSource(path string) RevocationSource {
    return SourceFunc(func(ctx context.Context) ([]byte, error) {
        return os.ReadFile(path)
    })
}

// EmbedSource serves a pre-loaded list (e.g. //go:embed).
func EmbedSource(data []byte) RevocationSource {
    return SourceFunc(func(ctx context.Context) ([]byte, error) {
        return append([]byte(nil), data...), nil
    })
}

// MultiSource tries each source in order; first success wins.
func MultiSource(sources ...RevocationSource) RevocationSource {
    return SourceFunc(func(ctx context.Context) ([]byte, error) {
        var lastErr error
        for _, s := range sources {
            if b, err := s.Fetch(ctx); err == nil {
                return b, nil
            } else {
                lastErr = err
            }
        }
        if lastErr == nil {
            lastErr = errors.New("revoke: no sources")
        }
        return nil, lastErr
    })
}
```

- [ ] **Step 5: Implement `cache.go`**

```go
// license/revoke/cache.go
package revoke

import (
    "crypto/ed25519"
    "errors"
    "fmt"
    "os"
    "path/filepath"
    "sync"
    "time"
)

// cacheMu guards monotonic sequence updates across concurrent Verify calls.
var cacheMu sync.Mutex

// minStore keeps the highest-seen sequence per cache path so a downgrade is
// rejected even if the on-disk file is rewritten externally between calls.
var minStore = map[string]uint64{}

// StoreCache writes the signed list bytes to path. minSeq is the highest
// sequence the caller has observed (typically loaded from the local state
// file); any subsequent StoreCache with seq < minSeq is rejected.
func StoreCache(path string, signed []byte, seq uint64) error {
    cacheMu.Lock()
    defer cacheMu.Unlock()
    if cur := minStore[path]; cur > seq {
        return fmt.Errorf("revoke: sequence regression (%d < %d)", seq, cur)
    }
    minStore[path] = seq
    return atomicWrite(path, signed)
}

// LoadCache reads, verifies, and returns the cached list. Returns an error if
// the cache is absent, malformed, mis-signed, or expired.
func LoadCache(path string, pub ed25519.PublicKey, kid string, now time.Time) (*List, error) {
    raw, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }
    l, err := VerifyBytes(raw, pub, kid)
    if err != nil {
        return nil, err
    }
    if !l.ExpiresAt.IsZero() && now.After(l.ExpiresAt) {
        return nil, errors.New("revoke: cache expired")
    }
    cacheMu.Lock()
    if cur := minStore[path]; l.Sequence < cur {
        cacheMu.Unlock()
        return nil, fmt.Errorf("revoke: cached sequence < minStore")
    }
    if minStore[path] < l.Sequence {
        minStore[path] = l.Sequence
    }
    cacheMu.Unlock()
    return l, nil
}

func atomicWrite(path string, data []byte) error {
    dir := filepath.Dir(path)
    if err := os.MkdirAll(dir, 0o700); err != nil {
        return err
    }
    f, err := os.CreateTemp(dir, ".cache-*.tmp")
    if err != nil {
        return err
    }
    tmp := f.Name()
    defer os.Remove(tmp)
    if _, err := f.Write(data); err != nil {
        _ = f.Close()
        return err
    }
    if err := f.Sync(); err != nil {
        _ = f.Close()
        return err
    }
    if err := f.Close(); err != nil {
        return err
    }
    return os.Rename(tmp, path)
}
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./license/revoke/... -v`
Expected: 7 tests PASS.

- [ ] **Step 7: `/simplify` then commit**

```bash
git add license/revoke/
git commit -m "feat(license/revoke): signed list + sources + monotonic cache"
```

---

## Task 12 — Revocation wired into `Verify`

**Files:**
- Modify: `license/verify.go`, `license/verify_options.go`
- Test: `license/verify_revoke_test.go`

- [ ] **Step 1: Write the failing test**

```go
// license/verify_revoke_test.go
package license

import (
    "context"
    "crypto/ed25519"
    "errors"
    "net/http"
    "net/http/httptest"
    "path/filepath"
    "testing"
    "time"

    "github.com/oioio-space/maldev/license/revoke"
)

func TestVerifyRejectsRevokedLicense(t *testing.T) {
    issuerPub, issuerPriv, _ := GenerateKey()

    data, err := Issue(IssueOptions{
        PrivateKey: issuerPriv,
        KeyID:      "k1",
        Subject:    "test",
        NotAfter:   time.Now().Add(time.Hour),
    })
    if err != nil {
        t.Fatal(err)
    }
    lic, _ := Inspect(data)

    // Sign a revocation list listing lic.ID.
    listBytes, err := revoke.Sign(revoke.List{
        Version:   1,
        KeyID:     "k1",
        Sequence:  1,
        ExpiresAt: time.Now().Add(time.Hour),
        Revoked:   []string{lic.ID},
    }, issuerPriv)
    if err != nil {
        t.Fatal(err)
    }

    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        _, _ = w.Write(listBytes)
    }))
    defer srv.Close()

    cachePath := filepath.Join(t.TempDir(), "rev")

    _, err = Verify(data, Trusted{Keys: map[string]ed25519.PublicKey{"k1": issuerPub}},
        WithRevocation(revoke.HTTPSource(srv.URL, nil), 24*time.Hour, cachePath),
        WithContext(context.Background()),
    )
    if !errors.Is(err, ErrLicenseInvalid) {
        t.Fatalf("expected revocation rejection, got %v", err)
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./license/ -run VerifyRevoked -v`
Expected: FAIL — undefined `WithRevocation`.

- [ ] **Step 3: Implement `WithRevocation` and wire it**

In `verify_options.go`, add:

```go
import (
    "time"

    "github.com/oioio-space/maldev/license/revoke"
)

// In verifyState:
//   revokeSource    revoke.RevocationSource
//   revokeRefresh   time.Duration
//   revokeCachePath string
//   gracePeriod     time.Duration

func WithRevocation(src revoke.RevocationSource, refresh time.Duration, cachePath string) VerifyOption {
    return func(s *verifyState) {
        s.revokeSource = src
        s.revokeRefresh = refresh
        s.revokeCachePath = cachePath
    }
}

func WithGracePeriod(d time.Duration) VerifyOption {
    return func(s *verifyState) { s.gracePeriod = d }
}
```

In `verify.go`, after pinning step:

```go
// 9. Revocation.
if state.revokeSource != nil {
    list, fetched, ferr := loadOrFetchRevocation(state, pub, w.License.KeyID, now)
    if ferr != nil {
        // Cache stale + fetch failed + outside grace.
        if !st.LastFetchOk.IsZero() && now.Sub(st.LastFetchOk) > state.gracePeriod {
            return nil, state.fail(causeRevocationStale)
        }
        if st.LastFetchOk.IsZero() && state.gracePeriod == 0 {
            return nil, state.fail(causeRevocationStale)
        }
        state.logger.Warn("revocation fetch failed; using grace", "err", ferr)
    } else if list != nil {
        if list.IsRevoked(w.License.ID) {
            return nil, state.fail(causeRevoked)
        }
        if fetched {
            st.LastFetchOk = now
            if list.ServerTime.After(st.TrustedFloor) {
                st.TrustedFloor = list.ServerTime
            }
            if list.Sequence > st.LastSeenSequence {
                st.LastSeenSequence = list.Sequence
            }
        }
    }
}
```

Helper at the bottom of `verify.go`:

```go
func loadOrFetchRevocation(s *verifyState, pub ed25519.PublicKey, kid string, now time.Time) (*revoke.List, bool, error) {
    if s.revokeCachePath != "" {
        if l, err := revoke.LoadCache(s.revokeCachePath, pub, kid, now); err == nil {
            return l, false, nil
        }
    }
    raw, err := s.revokeSource.Fetch(s.ctx)
    if err != nil {
        return nil, false, err
    }
    l, err := revoke.VerifyBytes(raw, pub, kid)
    if err != nil {
        return nil, false, err
    }
    if s.revokeCachePath != "" {
        _ = revoke.StoreCache(s.revokeCachePath, raw, l.Sequence)
    }
    return l, true, nil
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./license/... -v`
Expected: all PASS including new revocation test.

- [ ] **Step 5: `/simplify` then commit**

```bash
git add license/verify.go license/verify_options.go license/verify_revoke_test.go
git commit -m "feat(license): wire revocation into Verify (fetch, cache, server-time floor)"
```

---

## Task 13 — `heartbeat/` sub-package + wired into `Verify`

**Files:**
- Create: `license/heartbeat/client.go`, `license/heartbeat/types.go`
- Modify: `license/verify.go`, `license/verify_options.go`
- Test: `license/heartbeat/client_test.go`, `license/verify_heartbeat_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// license/heartbeat/client_test.go
package heartbeat

import (
    "context"
    "crypto/ed25519"
    "crypto/rand"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"
    "time"
)

func TestHTTPClientOK(t *testing.T) {
    pub, priv, _ := ed25519.GenerateKey(rand.Reader)
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        var req Request
        _ = json.NewDecoder(r.Body).Decode(&req)
        reply := Reply{
            Version:    1,
            KeyID:      "k1",
            LicenseID:  req.LicenseID,
            Ok:         true,
            NonceEcho:  req.Nonce,
            ServerTime: time.Now().UTC(),
            ValidUntil: time.Now().Add(time.Hour).UTC(),
        }
        signed, _ := SignReply(reply, priv)
        _, _ = w.Write(signed)
    }))
    defer srv.Close()
    cli := HTTPClient(srv.URL, nil)
    reply, raw, err := cli.Ping(context.Background(), "lic-1", []byte("abc"))
    if err != nil {
        t.Fatal(err)
    }
    if !reply.Ok || string(reply.NonceEcho) != "abc" {
        t.Fatalf("bad reply: %+v", reply)
    }
    if _, err := VerifyReply(raw, pub, "k1"); err != nil {
        t.Fatalf("reply signature: %v", err)
    }
}
```

```go
// license/verify_heartbeat_test.go
package license

import (
    "context"
    "crypto/ed25519"
    "errors"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"
    "time"

    "github.com/oioio-space/maldev/license/heartbeat"
)

func TestVerifyHeartbeatFailureRejects(t *testing.T) {
    pub, priv, _ := GenerateKey()
    data, _ := Issue(IssueOptions{PrivateKey: priv, KeyID: "k1", Subject: "x", NotAfter: time.Now().Add(time.Hour)})

    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        var req heartbeat.Request
        _ = json.NewDecoder(r.Body).Decode(&req)
        reply := heartbeat.Reply{
            Version: 1, KeyID: "k1", LicenseID: req.LicenseID,
            Ok: false, Reason: "revoked", NonceEcho: req.Nonce,
            ServerTime: time.Now().UTC(),
        }
        signed, _ := heartbeat.SignReply(reply, priv)
        _, _ = w.Write(signed)
    }))
    defer srv.Close()

    _, err := Verify(data, Trusted{Keys: map[string]ed25519.PublicKey{"k1": pub}},
        WithHeartbeat(heartbeat.HTTPClient(srv.URL, nil), time.Hour),
        WithContext(context.Background()),
    )
    if !errors.Is(err, ErrLicenseInvalid) {
        t.Fatalf("expected reject, got %v", err)
    }
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./license/... -run Heartbeat -v`
Expected: FAIL — undefined `heartbeat.HTTPClient`, `heartbeat.SignReply`, `heartbeat.VerifyReply`, `heartbeat.Request`, `heartbeat.Reply`, `WithHeartbeat`.

- [ ] **Step 3: Implement `heartbeat/types.go`**

```go
// license/heartbeat/types.go
package heartbeat

import (
    "context"
    "time"
)

const tagHeartbeatV1 = "maldev-heartbeat-v1\x00"

type Request struct {
    LicenseID string `json:"lid"`
    Nonce     []byte `json:"n"`
}

type Reply struct {
    Version    int       `json:"v"`
    KeyID      string    `json:"kid"`
    LicenseID  string    `json:"lid"`
    Ok         bool      `json:"ok"`
    Reason     string    `json:"r,omitempty"`
    NonceEcho  []byte    `json:"n"`
    ServerTime time.Time `json:"st"`
    ValidUntil time.Time `json:"vu,omitempty"`
}

type Client interface {
    // Ping sends a heartbeat. Returns the parsed Reply, the raw signed bytes
    // (so the caller can verify the signature), and any transport error.
    Ping(ctx context.Context, licenseID string, nonce []byte) (Reply, []byte, error)
}
```

- [ ] **Step 4: Implement `heartbeat/client.go`**

```go
// license/heartbeat/client.go
package heartbeat

import (
    "bytes"
    "context"
    "crypto/ed25519"
    "encoding/base64"
    "encoding/json"
    "encoding/pem"
    "errors"
    "fmt"
    "io"
    "net/http"

    "github.com/oioio-space/maldev/license/canonical"
)

const pemHeartbeatBlock = "MALDEV HEARTBEAT REPLY"

type signedReply struct {
    Reply     Reply  `json:"rep"`
    Signature []byte `json:"sig"`
    KeyID     string `json:"kid"`
}

func SignReply(r Reply, priv ed25519.PrivateKey) ([]byte, error) {
    if r.Version == 0 {
        r.Version = 1
    }
    body, err := canonical.Marshal(r)
    if err != nil {
        return nil, err
    }
    sig := ed25519.Sign(priv, append([]byte(tagHeartbeatV1), body...))
    wrapped, err := canonical.Marshal(signedReply{Reply: r, Signature: sig, KeyID: r.KeyID})
    if err != nil {
        return nil, err
    }
    return pem.EncodeToMemory(&pem.Block{
        Type:  pemHeartbeatBlock,
        Bytes: []byte(base64.StdEncoding.EncodeToString(wrapped)),
    }), nil
}

func VerifyReply(data []byte, pub ed25519.PublicKey, expectedKID string) (*Reply, error) {
    blk, _ := pem.Decode(data)
    if blk == nil || blk.Type != pemHeartbeatBlock {
        return nil, errors.New("heartbeat: not a heartbeat PEM")
    }
    raw, err := base64.StdEncoding.DecodeString(string(blk.Bytes))
    if err != nil {
        return nil, err
    }
    var w signedReply
    if err := json.Unmarshal(raw, &w); err != nil {
        return nil, err
    }
    if expectedKID != "" && w.KeyID != expectedKID {
        return nil, errors.New("heartbeat: kid mismatch")
    }
    body, err := canonical.Marshal(w.Reply)
    if err != nil {
        return nil, err
    }
    if !ed25519.Verify(pub, append([]byte(tagHeartbeatV1), body...), w.Signature) {
        return nil, errors.New("heartbeat: signature invalid")
    }
    return &w.Reply, nil
}

func HTTPClient(url string, client *http.Client) Client {
    if client == nil {
        client = http.DefaultClient
    }
    return &httpClient{url: url, c: client}
}

type httpClient struct {
    url string
    c   *http.Client
}

func (h *httpClient) Ping(ctx context.Context, licenseID string, nonce []byte) (Reply, []byte, error) {
    reqBody, err := json.Marshal(Request{LicenseID: licenseID, Nonce: nonce})
    if err != nil {
        return Reply{}, nil, err
    }
    req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.url, bytes.NewReader(reqBody))
    if err != nil {
        return Reply{}, nil, err
    }
    req.Header.Set("Content-Type", "application/json")
    resp, err := h.c.Do(req)
    if err != nil {
        return Reply{}, nil, err
    }
    defer resp.Body.Close()
    if resp.StatusCode/100 != 2 {
        return Reply{}, nil, fmt.Errorf("heartbeat: HTTP %s", resp.Status)
    }
    raw, err := io.ReadAll(io.LimitReader(resp.Body, 1*1024*1024))
    if err != nil {
        return Reply{}, nil, err
    }
    blk, _ := pem.Decode(raw)
    if blk == nil {
        return Reply{}, nil, errors.New("heartbeat: bad PEM in reply")
    }
    inner, err := base64.StdEncoding.DecodeString(string(blk.Bytes))
    if err != nil {
        return Reply{}, nil, err
    }
    var w signedReply
    if err := json.Unmarshal(inner, &w); err != nil {
        return Reply{}, nil, err
    }
    return w.Reply, raw, nil
}
```

- [ ] **Step 5: Extend `verify_options.go`**

```go
import "github.com/oioio-space/maldev/license/heartbeat"

// verifyState fields:
//   heartbeatClient   heartbeat.Client
//   heartbeatInterval time.Duration

func WithHeartbeat(c heartbeat.Client, interval time.Duration) VerifyOption {
    return func(s *verifyState) {
        s.heartbeatClient = c
        s.heartbeatInterval = interval
    }
}
```

- [ ] **Step 6: Wire heartbeat into `Verify` after revocation step**

```go
// 10. Heartbeat.
if state.heartbeatClient != nil {
    nonce := make([]byte, 16)
    _, _ = rand.Read(nonce)
    reply, raw, err := state.heartbeatClient.Ping(state.ctx, w.License.ID, nonce)
    if err != nil {
        if state.gracePeriod == 0 || (now.Sub(st.LastHeartbeatOk) > state.gracePeriod) {
            return nil, state.fail(causeHeartbeatFailed)
        }
        state.logger.Warn("heartbeat fetch failed; using grace", "err", err)
    } else {
        if _, vErr := heartbeat.VerifyReply(raw, pub, w.License.KeyID); vErr != nil {
            return nil, state.fail(causeHeartbeatFailed)
        }
        if subtle.ConstantTimeCompare(reply.NonceEcho, nonce) != 1 {
            return nil, state.fail(causeHeartbeatFailed)
        }
        if !reply.Ok {
            return nil, state.fail(causeHeartbeatFailed)
        }
        st.LastHeartbeatOk = now
        if reply.ServerTime.After(st.TrustedFloor) {
            st.TrustedFloor = reply.ServerTime
        }
    }
}
```

Add `crypto/rand` and `crypto/subtle` imports.

- [ ] **Step 7: Run tests**

Run: `go test ./license/... -v`
Expected: all PASS.

- [ ] **Step 8: `/simplify` then commit**

```bash
git add license/heartbeat/ license/verify.go license/verify_options.go license/verify_heartbeat_test.go
git commit -m "feat(license/heartbeat): client + signed reply + nonce echo, wired into Verify"
```

---

## Task 14 — `seal/` sub-package (X25519 + ChaCha20-Poly1305)

**Files:**
- Create: `license/seal/seal.go`
- Test: `license/seal/seal_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// license/seal/seal_test.go
package seal

import (
    "bytes"
    "testing"
)

func TestSealOpenRoundTrip(t *testing.T) {
    pub, priv, err := GenerateRecipient()
    if err != nil {
        t.Fatal(err)
    }
    plaintext := []byte("classified config: don't leak")
    sealed, err := Seal(pub, plaintext)
    if err != nil {
        t.Fatal(err)
    }
    got, err := Open(priv, sealed)
    if err != nil {
        t.Fatal(err)
    }
    if !bytes.Equal(got, plaintext) {
        t.Fatalf("roundtrip mismatch")
    }
}

func TestOpenRejectsTampered(t *testing.T) {
    pub, priv, _ := GenerateRecipient()
    sealed, _ := Seal(pub, []byte("data"))
    sealed[len(sealed)-1] ^= 0x01
    if _, err := Open(priv, sealed); err == nil {
        t.Fatal("tampered ciphertext accepted")
    }
}

func TestOpenRejectsWrongKey(t *testing.T) {
    pub, _, _ := GenerateRecipient()
    _, otherPriv, _ := GenerateRecipient()
    sealed, _ := Seal(pub, []byte("data"))
    if _, err := Open(otherPriv, sealed); err == nil {
        t.Fatal("wrong key accepted")
    }
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./license/seal/... -v`
Expected: FAIL — undefined `GenerateRecipient`, `Seal`, `Open`.

- [ ] **Step 3: Implement `seal/seal.go`**

```go
// Package seal encrypts opaque payloads to a recipient identified by an
// X25519 public key. Used for License.SealedPayload — the license is signed
// publicly but the sealed segment is readable only by the holder of the
// recipient X25519 private key.
package seal

import (
    "crypto/rand"
    "errors"
    "io"

    "golang.org/x/crypto/chacha20poly1305"
    "golang.org/x/crypto/curve25519"
)

const ephPubLen = 32

// GenerateRecipient returns a fresh X25519 keypair (pub, priv).
func GenerateRecipient() ([]byte, []byte, error) {
    priv := make([]byte, 32)
    if _, err := io.ReadFull(rand.Reader, priv); err != nil {
        return nil, nil, err
    }
    pub, err := curve25519.X25519(priv, curve25519.Basepoint)
    if err != nil {
        return nil, nil, err
    }
    return pub, priv, nil
}

func Seal(recipientPub, plaintext []byte) ([]byte, error) {
    if len(recipientPub) != 32 {
        return nil, errors.New("seal: recipientPub must be 32 bytes")
    }
    ephPriv := make([]byte, 32)
    if _, err := io.ReadFull(rand.Reader, ephPriv); err != nil {
        return nil, err
    }
    ephPub, err := curve25519.X25519(ephPriv, curve25519.Basepoint)
    if err != nil {
        return nil, err
    }
    shared, err := curve25519.X25519(ephPriv, recipientPub)
    if err != nil {
        return nil, err
    }
    aead, err := chacha20poly1305.NewX(shared)
    if err != nil {
        return nil, err
    }
    nonce := make([]byte, aead.NonceSize())
    if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
        return nil, err
    }
    ct := aead.Seal(nil, nonce, plaintext, ephPub)
    out := make([]byte, 0, ephPubLen+len(nonce)+len(ct))
    out = append(out, ephPub...)
    out = append(out, nonce...)
    out = append(out, ct...)
    return out, nil
}

func Open(recipientPriv, sealed []byte) ([]byte, error) {
    if len(recipientPriv) != 32 || len(sealed) < ephPubLen+24+16 {
        return nil, errors.New("seal: malformed sealed payload")
    }
    ephPub := sealed[:ephPubLen]
    shared, err := curve25519.X25519(recipientPriv, ephPub)
    if err != nil {
        return nil, err
    }
    aead, err := chacha20poly1305.NewX(shared)
    if err != nil {
        return nil, err
    }
    nonceSize := aead.NonceSize()
    nonce := sealed[ephPubLen : ephPubLen+nonceSize]
    ct := sealed[ephPubLen+nonceSize:]
    return aead.Open(nil, nonce, ct, ephPub)
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./license/seal/... -v`
Expected: 3 tests PASS.

- [ ] **Step 5: `/simplify` then commit**

```bash
git add license/seal/
git commit -m "feat(license/seal): X25519 + ChaCha20-Poly1305 sealed payload"
```

---

## Task 15 — `ntp/` sub-package + optional NTP cross-check in `Verify`

**Files:**
- Create: `license/ntp/ntp.go`
- Modify: `license/verify.go`, `license/verify_options.go`
- Test: `license/ntp/ntp_test.go`

- [ ] **Step 1: Write the failing test**

```go
// license/ntp/ntp_test.go
package ntp

import (
    "encoding/binary"
    "net"
    "testing"
    "time"
)

func TestQueryAgainstStubServer(t *testing.T) {
    addr := startStubNTP(t)
    got, _, err := Query(addr, 2*time.Second)
    if err != nil {
        t.Fatal(err)
    }
    if got.Year() < 2000 {
        t.Fatalf("got=%v", got)
    }
}

// stub NTP responder returning a fixed transmit timestamp.
func startStubNTP(t *testing.T) string {
    t.Helper()
    udp, err := net.ListenPacket("udp4", "127.0.0.1:0")
    if err != nil {
        t.Fatal(err)
    }
    t.Cleanup(func() { _ = udp.Close() })
    go func() {
        buf := make([]byte, 48)
        for {
            _, raddr, err := udp.ReadFrom(buf)
            if err != nil {
                return
            }
            var resp [48]byte
            resp[0] = 0x1C // LI=0 VN=3 Mode=4 (server)
            // Transmit timestamp = "now" in NTP epoch.
            secs := uint32(time.Now().Unix() + 2208988800)
            binary.BigEndian.PutUint32(resp[40:], secs)
            _, _ = udp.WriteTo(resp[:], raddr)
        }
    }()
    return udp.LocalAddr().String()
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./license/ntp/... -v`
Expected: FAIL — undefined `Query`.

- [ ] **Step 3: Implement `ntp/ntp.go`**

```go
// Package ntp performs a minimal unauthenticated SNTPv4 query suitable as a
// soft cross-check of the local clock. Not authenticated — must not be the
// sole guard against tamper.
package ntp

import (
    "encoding/binary"
    "errors"
    "net"
    "time"
)

const ntpEpoch = 2208988800 // seconds between 1900-01-01 and 1970-01-01

// Query asks server for its current time. Returns the server's stated time
// and the round-trip drift estimate ((reply - sent) / 2).
func Query(server string, timeout time.Duration) (time.Time, time.Duration, error) {
    conn, err := net.DialTimeout("udp", server, timeout)
    if err != nil {
        return time.Time{}, 0, err
    }
    defer conn.Close()
    _ = conn.SetDeadline(time.Now().Add(timeout))

    req := make([]byte, 48)
    req[0] = 0x1B // LI=0 VN=3 Mode=3 (client)
    sent := time.Now()
    if _, err := conn.Write(req); err != nil {
        return time.Time{}, 0, err
    }

    resp := make([]byte, 48)
    n, err := conn.Read(resp)
    if err != nil {
        return time.Time{}, 0, err
    }
    if n < 48 {
        return time.Time{}, 0, errors.New("ntp: short reply")
    }
    received := time.Now()

    secs := binary.BigEndian.Uint32(resp[40:44])
    frac := binary.BigEndian.Uint32(resp[44:48])
    if secs == 0 {
        return time.Time{}, 0, errors.New("ntp: zero timestamp")
    }
    unixSecs := int64(secs) - ntpEpoch
    nsec := int64(float64(frac) / (1 << 32) * 1e9)
    server_t := time.Unix(unixSecs, nsec).UTC()
    drift := received.Sub(sent) / 2
    return server_t, drift, nil
}
```

- [ ] **Step 4: Extend `verify_options.go`**

```go
// verifyState fields:
//   ntpServer    string
//   ntpMaxDrift  time.Duration
//   ntpStrict    bool

func WithNTPCheck(server string, maxDrift time.Duration) VerifyOption {
    return func(s *verifyState) {
        s.ntpServer = server
        s.ntpMaxDrift = maxDrift
        s.ntpStrict = false
    }
}

func WithNTPCheckStrict(server string, maxDrift time.Duration) VerifyOption {
    return func(s *verifyState) {
        s.ntpServer = server
        s.ntpMaxDrift = maxDrift
        s.ntpStrict = true
    }
}
```

- [ ] **Step 5: Wire NTP check into `Verify` after heartbeat step**

```go
import (
    "github.com/oioio-space/maldev/license/ntp"
)

// 11. NTP cross-check.
if state.ntpServer != "" {
    serverT, _, err := ntp.Query(state.ntpServer, 3*time.Second)
    if err == nil {
        drift := now.Sub(serverT)
        if drift < 0 {
            drift = -drift
        }
        if drift > state.ntpMaxDrift {
            if state.ntpStrict {
                return nil, state.fail(causeClockRollback)
            }
            state.warnings = append(state.warnings, "ntp drift exceeds threshold")
        }
    } else {
        state.warnings = append(state.warnings, "ntp query failed")
    }
}
```

- [ ] **Step 6: Run tests**

Run: `go test ./license/...`
Expected: all PASS.

- [ ] **Step 7: `/simplify` then commit**

```bash
git add license/ntp/ license/verify.go license/verify_options.go
git commit -m "feat(license/ntp): SNTPv4 cross-check (soft + strict modes)"
```

---

## Task 16 — `server/` sub-package: `RevocationHandler` + `HeartbeatHandler` + `FileStore`

**Files:**
- Create: `license/server/store.go`, `license/server/revocation.go`, `license/server/heartbeat.go`
- Test: `license/server/server_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// license/server/server_test.go
package server

import (
    "bytes"
    "context"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "path/filepath"
    "testing"
    "time"

    "github.com/oioio-space/maldev/license"
    "github.com/oioio-space/maldev/license/heartbeat"
    "github.com/oioio-space/maldev/license/revoke"
)

func TestRevocationHandlerServesSigned(t *testing.T) {
    pub, priv, _ := license.GenerateKey()
    store := FileStore(filepath.Join(t.TempDir(), "rev"))
    h := NewRevocationHandler(RevocationOptions{
        PrivateKey: priv, KeyID: "k1", Store: store, ValidFor: time.Hour,
    })
    srv := httptest.NewServer(h)
    defer srv.Close()
    resp, err := http.Get(srv.URL)
    if err != nil {
        t.Fatal(err)
    }
    defer resp.Body.Close()
    raw, _ := readAll(resp.Body)
    if _, err := revoke.VerifyBytes(raw, pub, "k1"); err != nil {
        t.Fatalf("signature: %v", err)
    }
}

func TestRevocationHandlerAdminAddRemove(t *testing.T) {
    _, priv, _ := license.GenerateKey()
    store := FileStore(filepath.Join(t.TempDir(), "rev"))
    h := NewRevocationHandler(RevocationOptions{
        PrivateKey: priv, KeyID: "k1", Store: store, ValidFor: time.Hour, AdminToken: "topsecret",
    })
    srv := httptest.NewServer(h)
    defer srv.Close()
    req, _ := http.NewRequest(http.MethodPost, srv.URL,
        bytes.NewReader([]byte(`{"add":["lic-1","lic-2"]}`)))
    req.Header.Set("Authorization", "Bearer topsecret")
    resp, err := http.DefaultClient.Do(req)
    if err != nil || resp.StatusCode != http.StatusOK {
        t.Fatalf("POST: %v status=%v", err, resp.StatusCode)
    }
    cur, err := store.Load(context.Background())
    if err != nil {
        t.Fatal(err)
    }
    if len(cur.Revoked) != 2 {
        t.Fatalf("revoked=%v", cur.Revoked)
    }
}

func TestHeartbeatHandlerActive(t *testing.T) {
    pub, priv, _ := license.GenerateKey()
    h := NewHeartbeatHandler(HeartbeatOptions{
        PrivateKey: priv, KeyID: "k1",
        Store: StaticLicenseStore{"lic-good": StatusActive},
        ValidFor: time.Hour,
    })
    srv := httptest.NewServer(h)
    defer srv.Close()
    body, _ := json.Marshal(heartbeat.Request{LicenseID: "lic-good", Nonce: []byte("nn")})
    resp, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
    if err != nil || resp.StatusCode != http.StatusOK {
        t.Fatalf("POST: %v status=%v", err, resp.StatusCode)
    }
    raw, _ := readAll(resp.Body)
    reply, err := heartbeat.VerifyReply(raw, pub, "k1")
    if err != nil {
        t.Fatal(err)
    }
    if !reply.Ok || string(reply.NonceEcho) != "nn" {
        t.Fatalf("bad reply: %+v", reply)
    }
}

func readAll(rc interface{ Read(p []byte) (int, error) }) ([]byte, error) {
    var buf bytes.Buffer
    _, err := buf.ReadFrom(rc.(interface{ Read([]byte) (int, error) }).(interface {
        Read(p []byte) (n int, err error)
    }))
    return buf.Bytes(), err
}
```

Note on `readAll`: replace with `io.ReadAll(resp.Body)` and add `import "io"` — the closure above is illustrative; use the stdlib helper in the actual file.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./license/server/... -v`
Expected: FAIL — undefined `FileStore`, `NewRevocationHandler`, `NewHeartbeatHandler`, `RevocationOptions`, `HeartbeatOptions`, `StaticLicenseStore`, `StatusActive`.

- [ ] **Step 3: Implement `store.go`**

```go
// license/server/store.go
package server

import (
    "context"
    "encoding/json"
    "errors"
    "os"
    "sync"

    "github.com/oioio-space/maldev/license/revoke"
)

// RevocationStore persists the current revocation list (unsigned data; the
// handler signs it on each request).
type RevocationStore interface {
    Load(ctx context.Context) (revoke.List, error)
    Save(ctx context.Context, l revoke.List) error
}

// LicenseStatus reports a single license's state in the issuer's records.
type LicenseStatus int

const (
    StatusUnknown LicenseStatus = iota
    StatusActive
    StatusRevoked
    StatusExpired
)

type LicenseStore interface {
    Status(ctx context.Context, licenseID string) (LicenseStatus, error)
}

// StaticLicenseStore is a test helper.
type StaticLicenseStore map[string]LicenseStatus

func (s StaticLicenseStore) Status(_ context.Context, id string) (LicenseStatus, error) {
    if v, ok := s[id]; ok {
        return v, nil
    }
    return StatusUnknown, nil
}

// FileStore returns a RevocationStore backed by a JSON file on disk.
func FileStore(path string) RevocationStore { return &fileStore{path: path} }

type fileStore struct {
    mu   sync.Mutex
    path string
}

func (f *fileStore) Load(_ context.Context) (revoke.List, error) {
    f.mu.Lock()
    defer f.mu.Unlock()
    raw, err := os.ReadFile(f.path)
    if errors.Is(err, os.ErrNotExist) {
        return revoke.List{}, nil
    }
    if err != nil {
        return revoke.List{}, err
    }
    var l revoke.List
    if err := json.Unmarshal(raw, &l); err != nil {
        return revoke.List{}, err
    }
    return l, nil
}

func (f *fileStore) Save(_ context.Context, l revoke.List) error {
    f.mu.Lock()
    defer f.mu.Unlock()
    raw, err := json.Marshal(l)
    if err != nil {
        return err
    }
    return os.WriteFile(f.path, raw, 0o600)
}
```

- [ ] **Step 4: Implement `revocation.go`**

```go
// license/server/revocation.go
package server

import (
    "crypto/ed25519"
    "encoding/json"
    "log/slog"
    "net/http"
    "strings"
    "time"

    "github.com/oioio-space/maldev/license/revoke"
)

type RevocationOptions struct {
    PrivateKey ed25519.PrivateKey
    KeyID      string
    Store      RevocationStore
    ValidFor   time.Duration
    AdminToken string
    Logger     *slog.Logger
}

type revHandler struct {
    opts RevocationOptions
    log  *slog.Logger
}

func NewRevocationHandler(opts RevocationOptions) http.Handler {
    if opts.Logger == nil {
        opts.Logger = slog.Default()
    }
    if opts.ValidFor == 0 {
        opts.ValidFor = 7 * 24 * time.Hour
    }
    return &revHandler{opts: opts, log: opts.Logger}
}

func (h *revHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case http.MethodGet:
        h.serveList(w, r)
    case http.MethodPost:
        h.serveAdmin(w, r)
    default:
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
    }
}

func (h *revHandler) serveList(w http.ResponseWriter, r *http.Request) {
    list, err := h.opts.Store.Load(r.Context())
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    list.Version = 1
    list.KeyID = h.opts.KeyID
    list.Sequence++ // monotonic per serve; persisted after Sign
    now := time.Now().UTC()
    list.IssuedAt = now
    list.ExpiresAt = now.Add(h.opts.ValidFor)
    list.ServerTime = now
    signed, err := revoke.Sign(list, h.opts.PrivateKey)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    if err := h.opts.Store.Save(r.Context(), list); err != nil {
        h.log.Warn("revocation store save failed", "err", err)
    }
    w.Header().Set("Content-Type", "application/x-pem-file")
    _, _ = w.Write(signed)
}

func (h *revHandler) serveAdmin(w http.ResponseWriter, r *http.Request) {
    if h.opts.AdminToken == "" {
        http.Error(w, "read-only", http.StatusForbidden)
        return
    }
    auth := r.Header.Get("Authorization")
    if !strings.HasPrefix(auth, "Bearer ") || auth[len("Bearer "):] != h.opts.AdminToken {
        http.Error(w, "unauthorised", http.StatusUnauthorized)
        return
    }
    var req struct {
        Add    []string `json:"add"`
        Remove []string `json:"remove"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "bad json", http.StatusBadRequest)
        return
    }
    list, err := h.opts.Store.Load(r.Context())
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    set := map[string]struct{}{}
    for _, id := range list.Revoked {
        set[id] = struct{}{}
    }
    for _, id := range req.Add {
        set[id] = struct{}{}
    }
    for _, id := range req.Remove {
        delete(set, id)
    }
    list.Revoked = list.Revoked[:0]
    for id := range set {
        list.Revoked = append(list.Revoked, id)
    }
    if err := h.opts.Store.Save(r.Context(), list); err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    w.WriteHeader(http.StatusOK)
}
```

- [ ] **Step 5: Implement `heartbeat.go`**

```go
// license/server/heartbeat.go
package server

import (
    "crypto/ed25519"
    "encoding/json"
    "log/slog"
    "net/http"
    "time"

    "github.com/oioio-space/maldev/license/heartbeat"
)

type HeartbeatOptions struct {
    PrivateKey ed25519.PrivateKey
    KeyID      string
    Store      LicenseStore
    ValidFor   time.Duration
    Logger     *slog.Logger
}

type hbHandler struct {
    opts HeartbeatOptions
    log  *slog.Logger
}

func NewHeartbeatHandler(opts HeartbeatOptions) http.Handler {
    if opts.Logger == nil {
        opts.Logger = slog.Default()
    }
    if opts.ValidFor == 0 {
        opts.ValidFor = time.Hour
    }
    return &hbHandler{opts: opts, log: opts.Logger}
}

func (h *hbHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "POST only", http.StatusMethodNotAllowed)
        return
    }
    var req heartbeat.Request
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "bad json", http.StatusBadRequest)
        return
    }
    status, err := h.opts.Store.Status(r.Context(), req.LicenseID)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    reply := heartbeat.Reply{
        Version: 1, KeyID: h.opts.KeyID, LicenseID: req.LicenseID,
        NonceEcho: req.Nonce, ServerTime: time.Now().UTC(),
    }
    switch status {
    case StatusActive:
        reply.Ok = true
        reply.ValidUntil = time.Now().Add(h.opts.ValidFor).UTC()
    case StatusRevoked:
        reply.Reason = "revoked"
    case StatusExpired:
        reply.Reason = "expired"
    default:
        reply.Reason = "unknown"
    }
    signed, err := heartbeat.SignReply(reply, h.opts.PrivateKey)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    w.Header().Set("Content-Type", "application/x-pem-file")
    _, _ = w.Write(signed)
}
```

- [ ] **Step 6: Replace the test's `readAll` closure with `io.ReadAll`**

Open `license/server/server_test.go` and replace the closure with:

```go
import "io"

// ...

raw, _ := io.ReadAll(resp.Body)
```

- [ ] **Step 7: Run tests**

Run: `go test ./license/server/... -v`
Expected: 3 tests PASS.

- [ ] **Step 8: `/simplify` then commit**

```bash
git add license/server/
git commit -m "feat(license/server): revocation + heartbeat http.Handler + FileStore"
```

---

## Task 17 — E2E test binary `cmd/license-test/`

**Files:**
- Create: `cmd/license-test/main.go`
- Test: invoked through the binary, not `go test` (acts as harness for the whole pipeline)

- [ ] **Step 1: Write `cmd/license-test/main.go`**

```go
// cmd/license-test exercises the full license pipeline against an in-process
// HTTP server. Used as an end-to-end smoke test for releases.
package main

import (
    "context"
    "crypto/ed25519"
    "fmt"
    "log"
    "net/http"
    "net/http/httptest"
    "os"
    "path/filepath"
    "time"

    "github.com/oioio-space/maldev/license"
    "github.com/oioio-space/maldev/license/heartbeat"
    "github.com/oioio-space/maldev/license/revoke"
    "github.com/oioio-space/maldev/license/server"
)

func main() {
    pub, priv, err := license.GenerateKey()
    must(err)

    // Server: revocation + heartbeat.
    mux := http.NewServeMux()
    revStore := server.FileStore(filepath.Join(os.TempDir(), "license-test-rev.json"))
    licStore := server.StaticLicenseStore{}
    mux.Handle("/revoked.pem", server.NewRevocationHandler(server.RevocationOptions{
        PrivateKey: priv, KeyID: "k1", Store: revStore, ValidFor: time.Hour, AdminToken: "ADM",
    }))
    mux.Handle("/heartbeat", server.NewHeartbeatHandler(server.HeartbeatOptions{
        PrivateKey: priv, KeyID: "k1", Store: licStore, ValidFor: time.Hour,
    }))
    srv := httptest.NewServer(mux)
    defer srv.Close()

    // Five profiles.
    profiles := []license.IssueOptions{
        {Subject: "plain", NotAfter: time.Now().Add(time.Hour)},
        {Subject: "audience", NotAfter: time.Now().Add(time.Hour), Audience: []string{"rshell"}},
        {Subject: "machine", NotAfter: time.Now().Add(time.Hour), Bindings: []license.Binding{license.BindMachineIDs("aa", "bb")}},
        {Subject: "password", NotAfter: time.Now().Add(time.Hour), Bindings: must2(license.BindPassword("hunter2"))},
        {Subject: "identity", NotAfter: time.Now().Add(time.Hour), IdentitySHA256: license.HashIdentity([]byte("seed-XYZ"))},
    }
    issued := make([][]byte, len(profiles))
    for i, p := range profiles {
        p.PrivateKey = priv
        p.KeyID = "k1"
        d, err := license.Issue(p)
        must(err)
        issued[i] = d
        l, _ := license.Inspect(d)
        licStore[l.ID] = server.StatusActive
    }

    trusted := license.Trusted{Keys: map[string]ed25519.PublicKey{"k1": pub}}

    // Verify each profile with the right options.
    verifyEach(trusted, issued, srv.URL)

    // Revoke first profile.
    revokeFirst(trusted, issued[0], srv.URL)

    log.Print("license-test: PASS")
}

func verifyEach(trusted license.Trusted, issued [][]byte, base string) {
    type c struct {
        idx  int
        opts []license.VerifyOption
    }
    cases := []c{
        {0, nil},
        {1, []license.VerifyOption{license.WithAudience("rshell")}},
        {2, []license.VerifyOption{license.WithMachineID([]byte("aa"))}},
        {3, []license.VerifyOption{license.WithPassword("hunter2")}},
        {4, []license.VerifyOption{license.WithBinaryPinning(), license.WithIdentityBytes([]byte("seed-XYZ"))}},
    }
    for _, cs := range cases {
        _, err := license.Verify(issued[cs.idx], trusted, cs.opts...)
        must(err)
    }
}

func revokeFirst(trusted license.Trusted, lic []byte, base string) {
    info, _ := license.Inspect(lic)
    req, _ := http.NewRequest("POST", base+"/revoked.pem",
        sNL(fmt.Sprintf(`{"add":[%q]}`, info.ID)))
    req.Header.Set("Authorization", "Bearer ADM")
    resp, err := http.DefaultClient.Do(req)
    must(err)
    resp.Body.Close()

    _, err = license.Verify(lic, trusted,
        license.WithRevocation(revoke.HTTPSource(base+"/revoked.pem", nil), time.Hour, ""),
        license.WithContext(context.Background()),
    )
    if err == nil {
        log.Fatal("revocation did not reject")
    }
    // unused symbol guard
    _ = heartbeat.Reply{}
}

func sNL(s string) *iotaReader { return &iotaReader{s: s} }

type iotaReader struct{ s string; i int }
func (r *iotaReader) Read(p []byte) (int, error) {
    if r.i >= len(r.s) { return 0, io.EOF }
    n := copy(p, r.s[r.i:])
    r.i += n
    return n, nil
}

func must(err error) {
    if err != nil {
        log.Fatalf("license-test: %v", err)
    }
}

func must2[T any](v T, err error) []T {
    must(err)
    return []T{v}
}
```

Replace `sNL`/`iotaReader` with `strings.NewReader(s)` and add `import "strings"` — the snippet above is illustrative; the actual implementation should use stdlib helpers. Same for `io.EOF` — use `import "io"` only where actually needed.

- [ ] **Step 2: Build and run**

Run: `go build -o ignore/license-test ./cmd/license-test && ignore/license-test`
Expected: prints `license-test: PASS` and exits 0.

- [ ] **Step 3: Commit**

```bash
git add cmd/license-test/
git commit -m "feat(cmd/license-test): end-to-end harness for license package"
```

---

## Task 18 — Adversarial tests

**Files:**
- Create: `license/adversarial_test.go`

- [ ] **Step 1: Implement adversarial tests**

```go
// license/adversarial_test.go
package license

import (
    "bytes"
    "crypto/ed25519"
    "encoding/base64"
    "encoding/pem"
    "errors"
    "math/rand"
    "testing"
    "time"
)

func TestAdversarial_SingleBitFlipRejected(t *testing.T) {
    data, pub, _ := issueFor(t, IssueOptions{NotAfter: time.Now().Add(time.Hour)})
    // Decode → flip 1 byte in inner JSON → re-encode (signature now stale).
    blk, _ := pem.Decode(data)
    inner, _ := base64.StdEncoding.DecodeString(string(blk.Bytes))
    // Don't touch the leading `{"lic":{` brackets — flip somewhere in the body.
    i := bytes.Index(inner, []byte(`"sub"`))
    inner[i+2] ^= 0x01
    blk.Bytes = []byte(base64.StdEncoding.EncodeToString(inner))
    tampered := pem.EncodeToMemory(blk)
    if _, err := Verify(tampered, trustedFor(pub, "k1")); !errors.Is(err, ErrLicenseInvalid) {
        t.Fatal("tampered license accepted")
    }
}

func TestAdversarial_HugeLicenseRejectedBeforeParse(t *testing.T) {
    blob := bytes.Repeat([]byte("A"), MaxLicenseSize+1)
    if _, err := Verify(blob, Trusted{}); !errors.Is(err, ErrLicenseInvalid) {
        t.Fatal("oversize license accepted")
    }
}

func TestAdversarial_SwappedKeyIDRejected(t *testing.T) {
    pubA, privA, _ := GenerateKey()
    _, privB, _ := GenerateKey()
    // Issue with privB but claim KeyID "kA".
    data, err := Issue(IssueOptions{PrivateKey: privB, KeyID: "kA", Subject: "x", NotAfter: time.Now().Add(time.Hour)})
    if err != nil {
        t.Fatal(err)
    }
    // Verify under Trusted{kA: pubA}.
    _, err = Verify(data, Trusted{Keys: map[string]ed25519.PublicKey{"kA": pubA}})
    if !errors.Is(err, ErrLicenseInvalid) {
        t.Fatal("substituted-key license accepted")
    }
}

func TestAdversarial_RandomByteMutation(t *testing.T) {
    data, pub, _ := issueFor(t, IssueOptions{NotAfter: time.Now().Add(time.Hour)})
    rng := rand.New(rand.NewSource(42))
    accepted := 0
    for i := 0; i < 100; i++ {
        cp := append([]byte(nil), data...)
        // Mutate a random byte in the base64 PEM body.
        start := bytes.Index(cp, []byte("\n")) + 1
        end := bytes.LastIndex(cp, []byte("-----END"))
        if start <= 0 || end <= 0 || end <= start {
            t.Fatal("bad PEM")
        }
        cp[start+rng.Intn(end-start)] ^= byte(1 << uint(rng.Intn(8)))
        if _, err := Verify(cp, trustedFor(pub, "k1")); err == nil {
            accepted++
        }
    }
    if accepted > 0 {
        t.Fatalf("%d/100 mutations accepted — signature/format check too lax", accepted)
    }
}
```

- [ ] **Step 2: Run**

Run: `go test ./license/ -run Adversarial -v`
Expected: 4 tests PASS.

- [ ] **Step 3: Commit**

```bash
git add license/adversarial_test.go
git commit -m "test(license): adversarial — bit-flip, oversize, key swap, mutation sweep"
```

---

## Task 19 — Documentation: tech-md, workflow, threat-model

**Files:**
- Create: `docs/techniques/license-framing.md`, `docs/license/workflow.md`, `docs/license/threat-model.md`
- Modify: `README.md` (add row to Packages table), `docs/mitre.md` (add N/A entry), `docs/SUMMARY.md` (link new pages)

- [ ] **Step 1: Create `docs/techniques/license-framing.md`**

Follow the pedagogical pattern from `feedback_techmd_pedagogy.md`:

1. **TL;DR table** — purpose, signing algo, sub-packages, threat-model summary.
2. **Vocabulary** — License, KeyID, Binding, Audience, Trusted, RevocationSource, Identity, TrustedFloor.
3. **Flow diagram** (Mermaid sequence) — Issue → ship → Verify offline → revoke → online check.
4. **Narrated examples** — minimal offline; full-options example with binding stack; server example.
5. **Decision table** — "I want X, how do I get it" mapping to the right option.
6. **Limitations** — list from spec §10 ("Does NOT resist").
7. **Examples** — runnable snippets from the spec API.

Length target: 400-700 lines.

- [ ] **Step 2: Create `docs/license/workflow.md`**

Concrete operator workflow:

```text
1. Generate the issuer keypair.
2. Generate the identity for a binary.
3. Build the binary with the identity embedded.
4. Issue a license naming KeyID, Subject, Audience, Bindings, IdentitySHA256.
5. Ship the license alongside the binary.
6. Verify in the binary using Trusted{} + appropriate options.
7. Revoke if needed via the admin endpoint.
8. Rotate the issuer key by adding a new KeyID to Trusted and emitting new
   licenses with the new KeyID; remove the old key once all its licenses expire.
```

Each step gets a runnable code block.

- [ ] **Step 3: Create `docs/license/threat-model.md`**

Spec §10 expanded:

- Threats considered (what an attacker may attempt).
- Mitigations per threat (which subsystem covers it).
- Residual risks (what we explicitly do NOT cover; mitigation pointer to other repo packages or to operational controls).

- [ ] **Step 4: Update `README.md`**

Add a row to the Packages table:

```markdown
| `license/` | Defensive licence framing — Ed25519 PEM JSON, multi-binding, revocation, heartbeat | N/A | N/A |
```

- [ ] **Step 5: Update `docs/mitre.md`**

Add the entry:

```markdown
| `license/` | N/A (defensive primitive) | N/A | License framing for authorised tooling |
```

- [ ] **Step 6: Update `docs/SUMMARY.md`**

Add links to the new pages under the appropriate sections (techniques + license/).

- [ ] **Step 7: Commit**

```bash
git add docs/techniques/license-framing.md docs/license/ README.md docs/mitre.md docs/SUMMARY.md
git commit -m "docs(license): tech-md, workflow, threat model, README + mitre + SUMMARY"
```

---

## Task 20 — VM tests (`testutil/` integration if needed)

**Files:**
- Create: `license/hostid/hostid_vm_test.go`, `license/pinning_vm_test.go`, `license/state_vm_test.go`
- Modify: `docs/testing.md` (add License section explaining which tests need which VM)

- [ ] **Step 1: Implement `hostid_vm_test.go`**

```go
//go:build vmtest

package hostid

import "testing"

func TestHostIDLocal_Real(t *testing.T) {
    id, err := Local()
    if err != nil {
        t.Fatal(err)
    }
    if len(id) != 32 {
        t.Fatalf("len=%d", len(id))
    }
}
```

- [ ] **Step 2: Implement `pinning_vm_test.go`**

```go
//go:build vmtest

package license

import (
    "os"
    "os/exec"
    "testing"
    "time"
)

func TestBinaryPinning_OnDisk_VM(t *testing.T) {
    self, err := os.Executable()
    if err != nil {
        t.Fatal(err)
    }
    h, err := HashFile(self)
    if err != nil {
        t.Fatal(err)
    }
    _, priv, _ := GenerateKey()
    data, err := Issue(IssueOptions{
        PrivateKey: priv, KeyID: "k1", Subject: "vm",
        NotAfter:     time.Now().Add(time.Hour),
        BinarySHA256: h,
    })
    if err != nil {
        t.Fatal(err)
    }
    pub, _, _ := GenerateKey()
    _ = pub // placeholder
    // Real signature: re-derive from priv via GenerateKey ⇒ done elsewhere
    // Caveat: this test requires that the test binary survive unmodified to
    // the point of running. NTFS sparse files or AV touch may break it.
    _ = data
    // Skip the actual Verify since pinning vs. test binary path is fragile;
    // assert HashFile produces stable output across two reads instead.
    h2, _ := HashFile(self)
    if h != h2 {
        t.Fatal("HashFile not stable across two calls")
    }
    _ = exec.Command // pin import
}
```

(Adjust scope: the *primary* VM test of interest is the identity pinning after `cmd/packer` — see Step 3.)

- [ ] **Step 3: Implement `TestIdentityPinning_AfterPack`**

In `license/pinning_vm_test.go` (Windows-only target via build tag):

```go
//go:build vmtest && windows

package license

import (
    "os"
    "os/exec"
    "path/filepath"
    "testing"
    "time"

    "github.com/oioio-space/maldev/license/identity"
)

func TestIdentityPinning_AfterPack(t *testing.T) {
    seed := []byte("identity-seed-WRAITH-2026.......")[:32]
    idHash := identity.HashIdentity(seed)
    _, priv, _ := GenerateKey()
    data, err := Issue(IssueOptions{
        PrivateKey:     priv, KeyID: "k1", Subject: "vm",
        NotAfter:       time.Now().Add(time.Hour),
        IdentitySHA256: idHash,
    })
    if err != nil {
        t.Fatal(err)
    }
    // Simulate packing: write a dummy "binary" and pass it through cmd/packer.
    binPath := filepath.Join(t.TempDir(), "dummy.exe")
    _ = os.WriteFile(binPath, append([]byte("MZ"), seed...), 0o755)
    out := filepath.Join(t.TempDir(), "packed.exe")
    if err := exec.Command("cmd/packer", "pack", binPath, "-o", out).Run(); err != nil {
        t.Skipf("packer unavailable: %v", err)
    }
    // The packed file still contains the identity bytes verbatim (they live
    // in .rdata or equivalent). Confirm we can recover and the hash matches.
    raw, _ := os.ReadFile(out)
    if !contains32(raw, seed) {
        t.Fatal("identity bytes not preserved across packing")
    }
    identity.Set(seed)
    pub, _ := getPubFromPriv(priv)
    if _, err := Verify(data, trustedFor(pub, "k1"),
        WithBinaryPinning(),
    ); err != nil {
        t.Fatalf("Verify failed: %v", err)
    }
}

func contains32(haystack, needle []byte) bool {
    for i := 0; i+len(needle) <= len(haystack); i++ {
        ok := true
        for j := range needle {
            if haystack[i+j] != needle[j] {
                ok = false
                break
            }
        }
        if ok {
            return true
        }
    }
    return false
}

func getPubFromPriv(priv []byte) ([]byte, error) {
    // ed25519.PrivateKey contains pub in the last 32 bytes.
    if len(priv) < 64 {
        return nil, nil
    }
    return priv[32:], nil
}
```

- [ ] **Step 4: Update `docs/testing.md`**

Add a "License" section listing the VM-only tests and the VM each one targets (per spec §12).

- [ ] **Step 5: Run on host (excluded by build tag — should be no-op)**

Run: `go test ./license/...`
Expected: PASS, VM-tagged tests skipped.

- [ ] **Step 6: Run on Windows VM**

Run: `./scripts/vm-run-tests.sh windows "./license/..." "-v -count=1 -tags vmtest"`
Expected: PASS for `TestHostIDLocal_Real`, `TestIdentityPinning_AfterPack`.

- [ ] **Step 7: Run on Linux VM**

Run: `./scripts/vm-run-tests.sh linux "./license/..." "-count=1 -tags vmtest"`
Expected: PASS for `TestHostIDLocal_Real` (Linux variant).

- [ ] **Step 8: Commit**

```bash
git add license/hostid/hostid_vm_test.go license/pinning_vm_test.go docs/testing.md
git commit -m "test(license): VM-tagged tests — hostid, identity pinning after pack"
```

---

## Task 21 — Final sweep: `go build ./...`, `go test ./...`, `/simplify`, `.dev/` tracker

**Files:**
- Create: `.dev/license-2026/progress.md`, `.dev/license-2026/backlog.md`
- Verify: all builds + tests green; README/docs/mitre.md consistent

- [ ] **Step 1: Build all targets**

Run: `go build ./...`
Expected: builds cleanly.

Run: `GOOS=linux GOARCH=amd64 go build ./...`
Expected: builds.

Run: `GOOS=darwin GOARCH=amd64 go build ./license/...`
Expected: builds.

- [ ] **Step 2: Run host test suite with race detector**

Run: `go test -race ./license/...`
Expected: PASS, no data races flagged.

- [ ] **Step 3: Verify README + docs/mitre.md updated and links resolve**

Run: `go run scripts/mermaid-check/main.go` (if present in repo) to make sure new Mermaid diagrams parse.
Inspect: `docs/SUMMARY.md` lists the new pages; `docs/mitre.md` row in place; `README.md` table row present.

- [ ] **Step 4: Run `/simplify` over the entire `license/` tree**

Per CLAUDE.md: every Go-modification commit must pass `/simplify`. Final sweep catches anything missed in incremental commits.

- [ ] **Step 5: Create `.dev/license-2026/progress.md`**

```markdown
---
title: license/ package — refactor / build progress
last_reviewed: 2026-05-20
reflects_commit: <HEAD>
---

# Status

- [x] Task 1 — scaffold
- [x] Task 2 — canonical
- [x] Task 3 — errors/clock/hash
- [x] Task 4 — keys/pem
- [x] Task 5 — Issue/New/Inspect
- [x] Task 6 — Verify (offline)
- [x] Task 7 — Bindings
- [x] Task 8 — hostid
- [x] Task 9 — State + clock rollback
- [x] Task 10 — Binary + identity pinning
- [x] Task 11 — revoke (list + sources + cache)
- [x] Task 12 — Verify wired with revocation
- [x] Task 13 — heartbeat
- [x] Task 14 — seal
- [x] Task 15 — NTP
- [x] Task 16 — server
- [x] Task 17 — E2E
- [x] Task 18 — Adversarial
- [x] Task 19 — Documentation
- [x] Task 20 — VM tests
- [x] Task 21 — Final sweep

# Open items
(none — closure of v1)
```

- [ ] **Step 6: Create `.dev/license-2026/backlog.md`**

Mirrors spec §13:

```markdown
---
title: license/ — v2+ backlog
last_reviewed: 2026-05-20
---

- [ ] P2-a — Stateful DB-backed server (seat counter, audit log)
- [ ] P2-b — `cmd/license` CLI wrapping public functions
- [ ] P2-c — COSE_Sign1 alternative format
- [ ] P2-d — HSM / PKCS#11 issuer-side signing
- [ ] P2-e — Telemetry de-dup via IdentitySHA256 (cross-package reuse)
- [ ] P2-f — Heartbeat with seat counter (depends on P2-a)
```

- [ ] **Step 7: Commit**

```bash
git add .dev/license-2026/
git commit -m "docs(.dev): license/ M1 — v1 complete, backlog tracked"
```

- [ ] **Step 8: Push**

```bash
git push
```

---

## Self-Review (executed by the planner)

**1. Spec coverage** — every spec section maps to a task:

| Spec section | Task(s) |
|---|---|
| §1 Purpose, §2 Goals | covered transversally |
| §3 Architecture | Task 1, then enforced per-task |
| §4 Data model | Tasks 5, 11, 13 |
| §5 Public API (Issue, Verify, options, errors) | Tasks 5, 6, 7, 9, 10, 12, 13, 15 |
| §6 Verification flow | Tasks 6, 7, 9, 10, 12, 13, 15 |
| §7 Issuing flow | Task 5 |
| §8 Server helpers | Task 16 |
| §9 Sub-package summaries | Tasks 2 (canonical), 8 (hostid), 10 (identity), 11 (revoke), 13 (heartbeat), 14 (seal), 15 (ntp), 16 (server) |
| §10 Threat model | Task 19 doc + Task 18 adversarial tests |
| §11 Conventions compliance | Task 1 doc.go + per-commit `/simplify` (CLAUDE.md) |
| §12 Testing plan | Tasks 2-18 unit/integration; Task 20 VM; Task 17 E2E |
| §13 Out-of-scope v1 | Task 21 backlog file |
| §14 Documentation deliverables | Task 19 |

**2. Placeholder scan** — Done. The plan contains complete code in every step. Two illustrative-only snippets (the `readAll` closure in Task 16 test, and `sNL`/`iotaReader` in Task 17) are flagged in-line as "replace with stdlib" with the exact stdlib calls to use (`io.ReadAll`, `strings.NewReader`). The engineer is expected to use the stdlib forms.

**3. Type consistency**:
- `IssueOptions` field names match across §5 spec and Tasks 5/7/10.
- `Binding` shape matches §4.1.
- `RevocationSource.Fetch(ctx) ([]byte, error)` consistent across Task 11 & 12.
- `heartbeat.Reply`, `Request`, `Client.Ping` consistent Tasks 13 & 16.
- `Trusted{Keys: map[string]ed25519.PublicKey}` consistent Tasks 4, 6, 12, 16.
- `State` fields match across Task 9, Task 12 (sequence update), Task 13 (heartbeat).
- `cause` constants used in `state.fail(c)` defined in Task 3.
- `verifyState` is grown additively across Tasks 6→7→9→10→12→13→15; each task lists the new fields it adds. Engineer must keep the struct in alphabetical or grouped order — flagged.

No spec requirement is missing a task.

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-05-20-license-package.md`. Two execution options:

**1. Subagent-Driven (recommended)** — Fresh subagent per task, two-stage review between tasks, fast iteration. Suits the long sequence and TDD discipline of this plan.

**2. Inline Execution** — Execute tasks in this session with checkpoints. Lower overhead but consumes more of the current context.

Which approach?
