---
package: github.com/oioio-space/maldev/license
---

# License package — Operator workflow

Concrete step-by-step guide: key generation, optional binary identity setup,
license issuance, verification, revocation, and key rotation. Each step
includes a runnable Go snippet.

## Step 1 — Generate the issuer keypair

Generate once per rotation period. The private key signs every license; the
public key is embedded in each binary that calls `Verify`.

```go
package main

import (
    "log"
    "os"

    "github.com/oioio-space/maldev/license"
)

func main() {
    pub, priv, err := license.GenerateKey()
    if err != nil {
        log.Fatal(err)
    }

    privPEM, err := license.MarshalPrivateKey(priv)
    if err != nil {
        log.Fatal(err)
    }
    if err := os.WriteFile("issuer-priv.pem", privPEM, 0o600); err != nil {
        log.Fatal(err)
    }

    // KID is a short identifier used for key rotation (see Step 7).
    pubPEM, err := license.MarshalPublicKey(pub, "k2026-05")
    if err != nil {
        log.Fatal(err)
    }
    if err := os.WriteFile("issuer-pub.pem", pubPEM, 0o644); err != nil {
        log.Fatal(err)
    }
}
```

> [!IMPORTANT]
> Keep `issuer-priv.pem` off the filesystem of any machine that runs the
> licensed binary. It should only exist on the operator's issuing workstation
> or in an HSM (v2).

---

## Step 2 — Generate a per-binary identity (optional)

Binary identity lets `Verify` reject a license even if the binary's on-disk
hash changes after packing. The `identity` sub-package provides a 32-byte
random seed embedded at build time.

```go
//go:generate go run github.com/oioio-space/maldev/license/identity/cmd/gen-identity

// In your binary's main package:
package main

import (
    _ "embed"

    "github.com/oioio-space/maldev/license/identity"
)

//go:embed identity.bin
var identityBlob []byte

func init() {
    // Register at program start. Panics on double-Set or wrong size.
    identity.Set(identityBlob)
}
```

Run `go generate ./...` once. `gen-identity` writes `identity.bin` if it does
not exist yet (idempotent). Commit `identity.bin` alongside the source.

Obtain the hash the issuer needs:

```go
hash := identity.HashIdentity(identityBlob) // hex-encoded sha256
```

---

## Step 3 — Issue a license

```go
package main

import (
    "encoding/hex"
    "log"
    "os"
    "time"

    "github.com/oioio-space/maldev/license"
)

func main() {
    privPEM, err := os.ReadFile("issuer-priv.pem")
    if err != nil {
        log.Fatal(err)
    }
    priv, err := license.ParsePrivateKey(privPEM)
    if err != nil {
        log.Fatal(err)
    }

    // Build the password binding (argon2id, 64 MiB, t=3, p=4).
    // The plaintext is not stored anywhere after this call.
    pwBind, err := license.BindPassword("s3cr3t-phrase")
    if err != nil {
        log.Fatal(err)
    }

    // Read the machine fingerprint from the target host (out-of-band).
    // In practice the licensee sends `hostid.Local()` to the issuer.
    targetMachineID := "abc123deadbeef" // placeholder

    // Identity hash from Step 2.
    idHash := hex.EncodeToString(identitySHA256[:]) // from your build

    data, err := license.Issue(license.IssueOptions{
        PrivateKey:     priv,
        KeyID:          "k2026-05",
        Subject:        "alice@example.com",
        Issuer:         "maldev-lab-eu",
        Audience:       []string{"rshell", "memscan-server"},
        NotAfter:       time.Now().AddDate(0, 6, 0).UTC(),
        Bindings: []license.Binding{
            license.BindMachineIDs(targetMachineID),
            pwBind,
            license.BindCustom("project", "WRAITH-2026"),
        },
        IdentitySHA256: idHash,
    })
    if err != nil {
        log.Fatal(err)
    }

    if err := os.WriteFile("alice-rshell.pem", data, 0o600); err != nil {
        log.Fatal(err)
    }
}
```

Ship `alice-rshell.pem` to the licensee via a secure channel. The binary
reads this file (or has it embedded) at startup.

---

## Step 4 — Verify in the binary

```go
package main

import (
    "context"
    "crypto/ed25519"
    _ "embed"
    "errors"
    "log/slog"
    "os"
    "time"

    "github.com/oioio-space/maldev/license"
    "github.com/oioio-space/maldev/license/hostid"
    "github.com/oioio-space/maldev/license/identity"
    "github.com/oioio-space/maldev/license/revoke"
)

//go:embed issuer-pub.pem
var pubPEM []byte

//go:embed identity.bin
var identityBlob []byte

func init() { identity.Set(identityBlob) }

func checkLicense(ctx context.Context, licensePath, passphrase string) error {
    data, err := os.ReadFile(licensePath)
    if err != nil {
        return err
    }

    pub, kid, err := license.ParsePublicKey(pubPEM)
    if err != nil {
        return err
    }

    mid, err := hostid.Local()
    if err != nil {
        return err
    }

    v, err := license.Verify(data,
        license.Trusted{Keys: map[string]ed25519.PublicKey{kid: pub}},
        license.WithAudience("rshell"),
        license.WithIssuer("maldev-lab-eu"),
        license.WithMachineID(mid),
        license.WithPassword(passphrase),
        license.WithCustom("project", "WRAITH-2026"),
        license.WithBinaryPinning(),
        license.WithRevocation(
            revoke.HTTPSource("https://lic.maldev.test/revoked.pem", nil),
            24*time.Hour,
            "~/.maldev/revoke-cache.pem",
        ),
        license.WithGracePeriod(7*24*time.Hour),
        license.WithStateFile("~/.maldev/license-state.json"),
        license.WithStateHostID(hostid.Local),
        license.WithMaxClockSkew(5*time.Minute),
        license.WithNTPCheck("pool.ntp.org", 10*time.Minute),
        license.WithLogger(slog.Default()),
        license.WithContext(ctx),
    )
    if err != nil {
        if errors.Is(err, license.ErrLicenseInvalid) {
            // cause was already logged to slog; surface only the opaque error
            return err
        }
        return err
    }

    for _, w := range v.Warnings {
        slog.Warn("license warning", "msg", w)
    }
    slog.Info("license OK", "subject", v.Subject, "key", v.KeyUsed)
    return nil
}
```

> [!NOTE]
> Embed `issuer-pub.pem` and `identity.bin` at build time so an attacker
> cannot swap them without recompiling. Combine with `cmd/packer` to harden
> further.

---

## Step 5 — Publish a revocation list (server side)

Start the revocation server once. Revoke individual licenses via the admin POST
endpoint.

```go
package main

import (
    "log/slog"
    "net/http"
    "os"

    "github.com/oioio-space/maldev/license"
    "github.com/oioio-space/maldev/license/server"
)

func main() {
    privPEM, _ := os.ReadFile("issuer-priv.pem")
    priv, _ := license.ParsePrivateKey(privPEM)

    mux := http.NewServeMux()
    mux.Handle("/revoked.pem", server.NewRevocationHandler(server.RevocationOptions{
        PrivateKey: priv,
        KeyID:      "k2026-05",
        Store:      server.FileStore("./revoked.json"),
        ValidFor:   7 * 24 * time.Hour,
        AdminToken: os.Getenv("MALDEV_ADMIN"),
        Logger:     slog.Default(),
    }))
    _ = http.ListenAndServeTLS(":443", "cert.pem", "key.pem", mux)
}
```

### Revoke a license

```bash
# Extract the license ID from a license file (no signature required).
LIC_ID=$(go run ./cmd/license-test inspect alice-rshell.pem | jq -r .id)

# POST the ID to the admin endpoint.
curl -X POST https://lic.maldev.test/revoked.pem \
  -H "Authorization: Bearer $MALDEV_ADMIN" \
  -H "Content-Type: application/json" \
  -d "{\"add\":[\"$LIC_ID\"]}"
```

The next `Verify` call that fetches the revocation list will return
`ErrLicenseInvalid`. Binaries using a cached list continue to run until the
cache's `ExpiresAt` (default 7 days), unless `WithGracePeriod(0)` was set.

---

## Step 6 — Run the heartbeat server (optional)

Heartbeat provides real-time license liveness checks with nonce echo and signed
server timestamps.

```go
mux.Handle("/heartbeat", server.NewHeartbeatHandler(server.HeartbeatOptions{
    PrivateKey: priv,
    KeyID:      "k2026-05",
    Store:      server.FileStore("./licenses.json"),
    ValidFor:   1 * time.Hour,
}))
```

On the binary side, add `WithHeartbeat`:

```go
license.WithHeartbeat(
    heartbeat.HTTPClient("https://lic.maldev.test/heartbeat"),
    1*time.Hour,
),
```

The binary sends a 16-byte random nonce; the server replies with a signed
`HeartbeatReply` carrying the nonce echo, server time, and `ok: true/false`.
`Verify` rejects if the nonce echo does not match or `ok` is false.

---

## Step 7 — Rotate the signing key

Key rotation does not require re-issuing licenses immediately. Old licenses
remain valid as long as their `NotAfter` has not passed.

```go
// 1. Generate the new keypair.
newPub, newPriv, _ := license.GenerateKey()
newPubPEM, _ := license.MarshalPublicKey(newPub, "k2026-11")

// 2. Add the new public key to every binary's Trusted map
//    (deploy a new binary build or update the embedded PEM).
trusted := license.Trusted{Keys: map[string]ed25519.PublicKey{
    "k2026-05": oldPub,   // keep until all k2026-05 licenses expire
    "k2026-11": newPub,
}}

// 3. Issue new licenses under the new KeyID.
data, _ := license.Issue(license.IssueOptions{
    PrivateKey: newPriv,
    KeyID:      "k2026-11",
    // ...
})

// 4. Once all k2026-05 licenses have passed their NotAfter, remove oldPub.
```

> [!CAUTION]
> Never delete an old public key from `Trusted` before all licenses it signed
> have expired. Removing a key causes `Verify` to return `ErrLicenseInvalid`
> for every holder of that license.
