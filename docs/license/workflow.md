---
package: github.com/oioio-space/maldev/license
---

# License — Cookbook

Copy-paste programmes complets. Chaque recette compile telle quelle après
`go get github.com/oioio-space/maldev@latest`.

- [Recette 1 — Hello license en 15 lignes](#recette-1)
- [Recette 2 — Émettre depuis un CLI, vérifier depuis un binaire](#recette-2)
- [Recette 3 — Binding multi-machines + mot de passe](#recette-3)
- [Recette 4 — Pinning d'identité qui survit au `cmd/packer`](#recette-4)
- [Recette 5 — Serveur de révocation HTTP minimal](#recette-5)
- [Recette 6 — Verify complet avec révocation + heartbeat + state file](#recette-6)
- [Recette 7 — Rotation de clé sans casser les licences existantes](#recette-7)
- [Recette 8 — Payload chiffré dans une licence (sealed payload)](#recette-8)

---

## Recette 1

**Génère une paire de clés, signe une licence, vérifie-la. Tout en RAM, zéro fichier.**

```go
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/oioio-space/maldev/license"
)

func main() {
	pub, priv, err := license.GenerateKey()
	if err != nil {
		log.Fatal(err)
	}

	// Émission "one-liner" (KeyID = "default", aucune contrainte hors NotAfter).
	data, err := license.New(priv, "alice@example.com", 24*time.Hour)
	if err != nil {
		log.Fatal(err)
	}

	// Vérification.
	v, err := license.Verify(data, license.Trusted{Keys: license.SingleKey("default", pub)})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("OK — délivrée à %s, expire %s\n", v.Subject, v.NotAfter)
}
```

> `license.New(priv, subject, ttl)` est le raccourci minimal. Pour ajouter
> Issuer, Audience, Bindings, Payload, etc., passe à `license.Issue` — Recette 2.

---

## Recette 2

**Émission CLI, vérification binaire, fichiers PEM sur disque.**

### a. Côté émetteur

```go
package main

import (
	"log"
	"os"
	"time"

	"github.com/oioio-space/maldev/license"
)

func main() {
	dir, _ := os.UserHomeDir()
	dir += "/.maldev-issuer"
	_ = os.MkdirAll(dir, 0o700)

	// GenerateAndSave : génère + écrit issuer.key (0600) + issuer.pub (KID embarqué).
	_, priv, err := license.GenerateAndSave(dir, "k2026-05")
	if err != nil {
		log.Fatal(err)
	}

	data, err := license.Issue(license.IssueOptions{
		PrivateKey: priv,
		KeyID:      "k2026-05",
		Subject:    "alice@example.com",
		Issuer:     "lab-eu",
		Audience:   []string{"rshell"},
		NotAfter:   time.Now().Add(90 * 24 * time.Hour),
	})
	if err != nil {
		log.Fatal(err)
	}

	if err := license.SaveLicense("./alice.license", data); err != nil {
		log.Fatal(err)
	}
	log.Print("Licence écrite : ./alice.license")
	log.Print("Distribue ./alice.license et ~/.maldev-issuer/issuer.pub")
}
```

### b. Côté binaire consommateur

```go
package main

import (
	"log"

	"github.com/oioio-space/maldev/license"
)

func main() {
	pub, kid, err := license.LoadPublicKey("./issuer.pub")
	if err != nil {
		log.Fatal(err)
	}

	v, err := license.VerifyFile("./alice.license",
		license.Trusted{Keys: license.SingleKey(kid, pub)},
		license.WithAudience("rshell"),
		license.WithIssuer("lab-eu"),
	)
	if err != nil {
		log.Fatalf("ACCESS DENIED: %v", err)
	}
	log.Printf("Autorisé — Subject=%s, KeyUsed=%s", v.Subject, v.KeyUsed)
}
```

---

## Recette 3

**Limiter à un set de machines + exiger un mot de passe.**

### Émission

```go
import "github.com/oioio-space/maldev/license/hostid"

// Obtient le fingerprint de la machine cible (depuis cette machine).
machineA, _ := hostid.Local()

pwBinding, err := license.BindPassword("hunter2")
if err != nil {
	log.Fatal(err)
}

data, err := license.Issue(license.IssueOptions{
	PrivateKey: priv,
	KeyID:      "k2026-05",
	Subject:    "alice@example.com",
	NotAfter:   time.Now().Add(30 * 24 * time.Hour),
	Bindings: []license.Binding{
		license.BindMachineIDs(string(machineA)),
		pwBinding,
	},
})
```

### Vérification

```go
me, _ := hostid.Local()
v, err := license.Verify(data, trusted,
	license.WithMachineID(me),
	license.WithPassword("hunter2"),
)
```

Si `me` ∉ liste OU password ≠ argon2id stocké → `ErrLicenseInvalid` (cause
précise loguée mais absente du message — pour ne pas guider un attaquant).

---

## Recette 4

**Identité embarquée — résiste au packer.**

### a. Générer l'identité (une fois par série de builds)

```bash
go run github.com/oioio-space/maldev/license/identity/cmd/gen-identity \
    -out cmd/rshell/identity.bin
```

Idempotent. Commit `identity.bin` pour que toute l'équipe partage la même
identité par binaire.

### b. Embarquer dans le binaire

```go
package main

import (
	_ "embed"
	"log"

	"github.com/oioio-space/maldev/license"
	"github.com/oioio-space/maldev/license/identity"
)

//go:embed identity.bin
var identityBytes []byte

func main() {
	identity.Set(identityBytes)

	pub, kid, _ := license.LoadPublicKey("issuer.pub")
	if _, err := license.VerifyFile("rshell.license",
		license.Trusted{Keys: license.SingleKey(kid, pub)},
		license.WithBinaryPinning(),
	); err != nil {
		log.Fatal(err)
	}
}
```

### c. Émettre pour cette identité

```go
identityBytes, _ := os.ReadFile("cmd/rshell/identity.bin")
data, _ := license.Issue(license.IssueOptions{
	PrivateKey:     priv,
	KeyID:          "k2026-05",
	Subject:        "alice",
	IdentitySHA256: license.HashIdentity(identityBytes),
	NotAfter:       time.Now().Add(180 * 24 * time.Hour),
})
```

La licence reste valide à travers `cmd/packer pack`, re-packing, strip,
signature Authenticode — tant que `identity.bin` reste embarqué.

---

## Recette 5

**Serveur HTTP minimal — revocation list signée + heartbeat. ~30 lignes.**

```go
package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/oioio-space/maldev/license"
	"github.com/oioio-space/maldev/license/server"
)

func main() {
	priv, err := license.LoadPrivateKey("/etc/maldev/issuer.key")
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.Handle("/revoked.pem", server.NewRevocationHandler(server.RevocationOptions{
		PrivateKey: priv,
		KeyID:      "k2026-05",
		Store:      server.FileStore("/var/lib/maldev/revoked.json"),
		ValidFor:   7 * 24 * time.Hour,
		AdminToken: os.Getenv("MALDEV_ADMIN_TOKEN"),
	}))
	mux.Handle("/heartbeat", server.NewHeartbeatHandler(server.HeartbeatOptions{
		PrivateKey: priv,
		KeyID:      "k2026-05",
		Store:      server.StaticLicenseStore{}, // remplace par ta propre LicenseStore (DB, fichier…)
		ValidFor:   time.Hour,
	}))

	log.Fatal(http.ListenAndServeTLS(":8443", "cert.pem", "key.pem", mux))
}
```

### Révoquer / annuler une révocation (admin)

```bash
# Révoquer
curl -X POST https://lic.example.com/revoked.pem \
     -H "Authorization: Bearer $MALDEV_ADMIN_TOKEN" \
     -d '{"add":["<license-id>"]}'

# Annuler
curl -X POST https://lic.example.com/revoked.pem \
     -H "Authorization: Bearer $MALDEV_ADMIN_TOKEN" \
     -d '{"remove":["<license-id>"]}'
```

---

## Recette 6

**Verify "production" — pinning + révocation + heartbeat + state file + NTP soft + grace period offline.**

```go
package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/oioio-space/maldev/license"
	"github.com/oioio-space/maldev/license/heartbeat"
	"github.com/oioio-space/maldev/license/hostid"
	"github.com/oioio-space/maldev/license/revoke"
)

func main() {
	pub, kid, _ := license.LoadPublicKey("issuer.pub")
	state := os.Getenv("HOME") + "/.maldev/license-state"

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	v, err := license.VerifyFile("rshell.license",
		license.Trusted{Keys: license.SingleKey(kid, pub)},

		// Scope.
		license.WithAudience("rshell"),
		license.WithIssuer("lab-eu"),
		license.WithMachineID(must(hostid.Local())),

		// Pinning binaire/identité.
		license.WithBinaryPinning(),

		// Révocation hybride : HTTP en priorité, fallback fichier, cache local.
		license.WithRevocation(
			revoke.MultiSource(
				revoke.HTTPSource("https://lic.example.com/revoked.pem", nil),
				revoke.FileSource("/etc/maldev/revoked.pem.cached"),
			),
			24*time.Hour,
			state+".revoke",
		),
		license.WithGracePeriod(7*24*time.Hour),

		// Heartbeat : ping rate-limité à 1/h.
		license.WithHeartbeat(
			heartbeat.HTTPClient("https://lic.example.com/heartbeat", nil),
			time.Hour,
		),

		// Anti-tamper d'horloge.
		license.WithStateFile(state),
		license.WithStateHostID(hostid.Local),
		license.WithMaxClockSkew(5*time.Minute),

		// NTP en soft warning (ne refuse pas).
		license.WithNTPCheck("pool.ntp.org:123", 10*time.Minute),

		// Timeout global réseau.
		license.WithContext(ctx),
	)
	if err != nil {
		log.Fatalf("license check failed: %v", err)
	}
	for _, w := range v.Warnings {
		log.Printf("[warn] %s", w)
	}
	log.Printf("autorisé — %s (key %s)", v.Subject, v.KeyUsed)
}

func must[T any](v T, err error) T {
	if err != nil {
		log.Fatal(err)
	}
	return v
}
```

---

## Recette 7

**Rotation de clé sans casser les licences existantes.**

```go
oldPub, _, _ := license.LoadPublicKey("/etc/maldev/issuer-2026-05.pub")
newPub, _, _ := license.LoadPublicKey("/etc/maldev/issuer-2026-11.pub")

trusted := license.Trusted{
	Keys: map[string]ed25519.PublicKey{
		"k2026-05": oldPub, // gardée jusqu'à expiry des dernières licences
		"k2026-11": newPub, // active maintenant
	},
}
_, err := license.VerifyFile("./client.license", trusted, /* options */)
```

**Workflow recommandé :**

1. Génère et déploie la nouvelle clé :
   `license.GenerateAndSave("/etc/maldev/issuer-2026-11/", "k2026-11")`
2. Push la nouvelle `issuer-2026-11.pub` à tous les binaires via release.
3. Émets les nouvelles licences avec `KeyID: "k2026-11"`.
4. Attends que toutes les licences `k2026-05` aient expiré (`max(NotAfter)`).
5. Retire `k2026-05` de `Trusted.Keys` dans les binaires à la release suivante.
6. Détruis `issuer-2026-05.key` (HSM/disque).

---

## Recette 8

**Payload chiffré pour un destinataire — sealed payload.**

```go
import "github.com/oioio-space/maldev/license/seal"

// 1. Le destinataire publie sa clé publique X25519.
recipientPub, recipientPriv, _ := seal.GenerateRecipient()

// 2. L'émetteur scelle un payload pour ce destinataire.
secretConfig := []byte(`{"endpoint":"https://c2.example.com","token":"xxx"}`)
sealed, _ := seal.Seal(recipientPub, secretConfig)

// 3. La licence transporte le scellé. Signée publiquement par l'issuer,
//    mais le contenu n'est lisible que par recipientPriv.
data, _ := license.Issue(license.IssueOptions{
	PrivateKey:    priv,
	KeyID:         "k1",
	Subject:       "alice",
	NotAfter:      time.Now().Add(24 * time.Hour),
	SealedPayload: sealed,
})

// 4. Côté binaire : déchiffre après Verify.
v, err := license.Verify(data, trusted)
if err != nil { /* ... */ }
config, err := seal.Open(recipientPriv, v.SealedPayload)
if err != nil { /* clé X25519 incorrecte */ }
fmt.Println("config:", string(config))
```

Sous le capot : X25519 ECDH → HKDF-SHA256 → XChaCha20-Poly1305 AEAD avec
l'ephemeral public key comme AAD.

---

## API helpers (cheatsheet)

| Fonction | Quand l'utiliser |
|---|---|
| `license.GenerateKey()` | Crée une paire Ed25519 en RAM. |
| `license.GenerateAndSave(dir, kid)` | Génère + écrit `issuer.key` (0600) et `issuer.pub` (0644). |
| `license.SavePrivateKey(path, priv)` | PEM `MALDEV PRIVATE KEY`, atomique, 0600. |
| `license.LoadPrivateKey(path)` | Lit + parse. |
| `license.SavePublicKey(path, pub, kid)` | PEM `MALDEV PUBLIC KEY` avec header `KID:`. |
| `license.LoadPublicKey(path)` | Renvoie `(pub, kid)`. |
| `license.SingleKey(kid, pub)` | Sucre pour `map[string]ed25519.PublicKey{kid: pub}`. |
| `license.New(priv, sub, ttl)` | Émission one-liner. |
| `license.Issue(IssueOptions{...})` | Émission complète. |
| `license.SaveLicense(path, data)` | Écrit le PEM `MALDEV LICENSE`. |
| `license.LoadLicense(path)` | Lit les bytes (à passer à `Verify`). |
| `license.Verify(data, trusted, ...)` | Vérification bytes-in. |
| `license.VerifyFile(path, trusted, ...)` | Vérification fichier-in. |
| `license.Inspect(data)` | Parse sans signature — **diagnostic uniquement**. |
| `license.HashFile(path)` | sha256 hex d'un fichier (→ `BinarySHA256`). |
| `license.HashIdentity(b)` | sha256 hex d'octets (→ `IdentitySHA256`). |
| `license.BindMachineIDs(...)` | Binding liste OU. |
| `license.BindPassword(p)` | Binding password (argon2id). |
| `license.BindCustom(name, v...)` | Binding custom k/v. |
| `license.RegisterVerifier(name, fn)` | Hook pour types de binding custom. |

---

## Référence des champs

Documentation détaillée champ par champ dans
[license-framing.md](../techniques/license-framing.md#référence-des-champs).
