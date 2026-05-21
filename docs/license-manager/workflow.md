---
package: github.com/oioio-space/maldev/cmd/license-manager
---

# License Manager — Cookbook

Recettes opérationnelles copier-coller. Chaque recette décrit le flux Go côté Services — la TUI expose les mêmes opérations graphiquement dès son implémentation.

> **Nouveau ?** Lis d'abord [concepts.md](./concepts.md) pour le vocabulaire et l'architecture. La [Configuration](./configuration.md) liste les flags et variables d'environnement.

**Recettes essentielles :**
- [Recette 1 — Première utilisation : créer la DB + premier Issuer](#recette-1)
- [Recette 2 — Émettre une licence simple](#recette-2)
- [Recette 3 — Émettre avec machine + password + TOTP](#recette-3)

**Recettes opérationnelles :**
- [Recette 4 — Fingerprint probe d'une machine distante](#recette-4)
- [Recette 5 — Révoquer + publier la CRL HTTP](#recette-5)
- [Recette 6 — Démarrer et arrêter les serveurs HTTP](#recette-6)
- [Recette 7 — Rotation de clé](#recette-7)
- [Recette 8 — Importer une licence PEM existante](#recette-8)
- [Recette 9 — Changer la passphrase de la DB](#recette-9)
- [Recette 10 — Re-issue d'une licence (remplace une existante)](#recette-10)

---

## Recette 1

**Première utilisation : initialiser la DB et créer le premier Issuer.**

Au premier lancement, si la DB n'existe pas, le wizard de démarrage s'enclenche. Voici l'équivalent programmatique :

```go
package main

import (
    "context"
    "log"

    "github.com/oioio-space/maldev/internal/manager/crypto"
    "github.com/oioio-space/maldev/internal/manager/service"
    "github.com/oioio-space/maldev/internal/manager/store"
)

func main() {
    ctx := context.Background()
    passphrase := "ma-passphrase-secrete"

    // 1. Générer un sel KEK et dériver la KEK.
    salt, err := crypto.GenerateSalt()
    if err != nil {
        log.Fatal(err)
    }
    kek, err := crypto.DeriveFromPassphrase(passphrase, salt, crypto.PresetDefault)
    if err != nil {
        log.Fatal(err)
    }

    // 2. Ouvrir (ou créer) la DB — migrations automatiques.
    db, err := store.New("/var/lib/maldev/manager.db", kek)
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    // 3. Construire le bundle de services.
    svc := service.New(db, kek)
    defer svc.Close()

    // 4. Créer le premier Issuer (active=true automatiquement si c'est le seul).
    issuer, err := svc.Issuer.Generate(ctx, "Lab EU primary", "k2026-05")
    if err != nil {
        log.Fatal(err)
    }
    log.Printf("Issuer créé : %s (kid=%s)", issuer.Name, issuer.KeyID)
}
```

La DB est prête. La clé privée de l'Issuer est stockée chiffrée (`encrypted_priv`). La passphrase n'est jamais écrite sur disque.

---

## Recette 2

**Émettre une licence simple avec Subject + Audience + durée.**

```go
import (
    "time"
    "github.com/oioio-space/maldev/internal/manager/service"
)

// svc est déjà construit (voir Recette 1).
issued, err := svc.License.Issue(ctx, service.IssueRequest{
    IssuerID:     issuer.ID,
    Subject:      "alice@example.com",
    AudienceList: []string{"rshell"},
    NotBefore:    time.Now(),
    NotAfter:     time.Now().Add(30 * 24 * time.Hour),
})
if err != nil {
    log.Fatal(err)
}

// issued.PEM est le blob PEM prêt à être distribué.
log.Printf("Licence : %s\n%s", issued.Row.LicenseUUID, issued.PEM)
```

Le PEM est stocké dans la DB (`License.pem`) et peut être réexporté à tout moment via `svc.License.ExportPEM(ctx, id)`.

---

## Recette 3

**Émettre avec binding machine + mot de passe + TOTP.**

Cette recette combine les trois bindings les plus courants. L'opérateur fournit les fingerprints de la machine autorisée (obtenus via Recette 4 ou `hostid.Local()` local).

```go
import (
    "time"
    "github.com/oioio-space/maldev/internal/manager/service"
)

issued, err := svc.License.Issue(ctx, service.IssueRequest{
    IssuerID:     issuer.ID,
    Subject:      "bob@example.com",
    AudienceList: []string{"rshell"},
    NotBefore:    time.Now(),
    NotAfter:     time.Now().Add(90 * 24 * time.Hour),
    Bindings: []service.BindingSpec{
        {
            Type:   "machine",
            Values: []string{"a1b2c3d4e5f6..."},  // hostid.Local() hex
        },
        {
            Type:   "password",
            Values: []string{"hunter2"},           // haché Argon2id à l'émission
        },
        {
            Type: "totp",                          // génère un secret TOTP automatiquement
        },
    },
    Features: []string{"pro"},
})
if err != nil {
    log.Fatal(err)
}

// Récupérer les infos de provisioning TOTP (secret, URI, QR).
if len(issued.TOTPs) > 0 {
    t := issued.TOTPs[0]
    log.Printf("TOTP URI : %s", t.OtpauthURI)
    log.Printf("QR ASCII :\n%s", t.QRImageASCII)
}
```

Le secret TOTP est stocké chiffré dans `TOTPSecret.encrypted_secret`. Pour le réafficher plus tard :

```go
view, err := svc.TOTP.PrintQRASCII(ctx, issued.Row.ID)
// ou
err = svc.TOTP.ExportQRPNG(ctx, issued.Row.ID, "/tmp/totp.png")
```

---

## Recette 4

**Fingerprint probe : obtenir le `hostid` d'une machine distante.**

Cette recette suppose que le serveur Probe est démarré (voir Recette 6).

```go
import "time"

// 1. Créer un token probe (valide 24h par défaut).
token, err := svc.Probe.NewToken(ctx, "Alice prod box", 24*time.Hour)
if err != nil {
    log.Fatal(err)
}
log.Printf("Token : %s", token.ID)
log.Printf("URL de base : https://<manager>:8445/probe/%s", token.ID)

// 2. S'abonner au résultat (channel notifié quand la machine POSTe).
resultCh := svc.Probe.Subscribe(token.ID)

// 3. Afficher le one-liner à copier-coller (aussi disponible via /snippet).
log.Printf(`One-liner Linux/macOS :
  URL="https://<manager>:8445/probe/%s"
  curl -fsSL "$URL/agent/$(uname -s | tr A-Z a-z)-$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')" \
    -o /tmp/maldev-probe && chmod +x /tmp/maldev-probe \
    && /tmp/maldev-probe "$URL/result"`, token.ID)

// 4. Attendre le résultat (goroutine ou select avec timeout).
result := <-resultCh
log.Printf("Hostname : %s", result.Hostname)
log.Printf("Local    : %s", result.LocalHex)
log.Printf("Composite: %s", result.CompositeHex)
```

Le résultat est persisté dans `ProbeToken` (colonnes `local_hex`, `composite_hex`, `hostname`, `os`, `arch`, `used_at`). `svc.Probe.History(ctx, 20)` liste les 20 derniers résultats.

---

## Recette 5

**Révoquer une licence + publier la CRL HTTP.**

```go
import "github.com/google/uuid"

licID := uuid.MustParse("73f56081-5cce-4073-9632-...")

// 1. Révoquer dans la DB (status → revoked, Revocation row créée).
err := svc.Revoke.Revoke(ctx, licID, "fin de mission")
if err != nil {
    log.Fatal(err)
}

// 2. Générer et afficher la CRL signée (même opération que le serveur HTTP).
crlPEM, err := svc.Revoke.PublishSignedList(ctx)
if err != nil {
    log.Fatal(err)
}
log.Printf("CRL:\n%s", crlPEM)
```

Si le serveur Revocation est démarré, chaque `GET /revoked.pem` appelle `PublishSignedList` à la volée — la CRL est toujours fraîche.

Pour annuler une révocation (cas d'erreur) :

```go
err = svc.Revoke.Unrevoke(ctx, licID)
```

---

## Recette 6

**Démarrer et arrêter les serveurs HTTP.**

Les serveurs sont configurés via `ServerConfig` (singleton PK=1). Ils sont tous OFF par défaut.

```go
import (
    "context"
    "time"

    "github.com/oioio-space/maldev/internal/manager/httpsrv"
)

// Construire le Bundle (à faire une fois, après service.New).
bundle := httpsrv.NewBundle(svc)
svc.AttachServers(bundle)

// Configurer les adresses d'écoute.
_, err := svc.Settings.UpdateServerConfig(ctx, func(u *ent.ServerConfigUpdate) {
    u.SetRevocationListen(":8443").
        SetHeartbeatListen(":8444").
        SetProbeListen(":8445")
})
if err != nil {
    log.Fatal(err)
}

// Démarrer individuellement.
if err := bundle.Revocation.Start(context.Background()); err != nil {
    log.Fatal(err)
}
if err := bundle.Heartbeat.Start(context.Background()); err != nil {
    log.Fatal(err)
}
if err := bundle.Probe.Start(context.Background()); err != nil {
    log.Fatal(err)
}

// Statut courant.
s := bundle.Revocation.Status()
log.Printf("Revocation: running=%v addr=%s requests=%d", s.Running, s.ListenAddr, s.Requests)

// Arrêt propre (10 s timeout).
bundle.StopAll(10 * time.Second)
```

Les événements temps-réel (requêtes entrantes, erreurs) sont disponibles via `bundle.MergedEvents()`.

---

## Recette 7

**Rotation de clé : générer un nouvel Issuer, le désigner actif, retirer l'ancien.**

La rotation ne casse pas les licences existantes : le binaire consommateur charge plusieurs `kid` dans son `Trusted`. Seules les nouvelles licences seront signées avec la nouvelle clé.

```go
// 1. Générer le nouvel Issuer (inactive par défaut si un autre est déjà actif).
newIssuer, err := svc.Issuer.Generate(ctx, "Lab EU rotation", "k2026-08")
if err != nil {
    log.Fatal(err)
}

// 2. Désigner le nouvel Issuer comme actif.
//    L'ancien Issuer perd son flag active=true automatiquement.
err = svc.Issuer.SetActive(ctx, newIssuer.ID)
if err != nil {
    log.Fatal(err)
}

// 3. Exporter la nouvelle clé publique pour la distribuer avec les binaires.
pubPEM, err := svc.Issuer.ExportPublic(ctx, newIssuer.ID)
if err != nil {
    log.Fatal(err)
}
log.Printf("Nouvelle clé publique :\n%s", pubPEM)

// 4. Retirer l'ancien Issuer (optionnel — signe encore les licences existantes).
oldIssuerID := uuid.MustParse("...")
err = svc.Issuer.Retire(ctx, oldIssuerID)
```

> `Issuer.Delete` refuse si des licences ont été signées par cet Issuer. Utilise `Retire` pour le marquer inactif sans le supprimer.

---

## Recette 8

**Importer une licence PEM existante dans la DB.**

Utile si une licence a été émise manuellement via le package `license/` ou reçue d'un autre manager.

```go
pemBytes, err := os.ReadFile("/path/to/licence.pem")
if err != nil {
    log.Fatal(err)
}

row, err := svc.License.Import(ctx, pemBytes, "licence Alice importée")
if err != nil {
    log.Fatal(err)
}
log.Printf("Importée : %s (status=%s)", row.LicenseUUID, row.Status)
```

La licence est décodée et vérifiée (signature Ed25519) avant insertion. `status` est mis à `active` si `not_after` est dans le futur, `expired` sinon.

Pour inspecter un PEM sans l'insérer :

```go
lic, err := svc.License.Inspect(pemBytes)
// lic est un *license.License décodé, non inséré en DB.
```

---

## Recette 9

**Changer la passphrase de la DB.**

Le changement de passphrase re-dérive la KEK et re-chiffre toutes les colonnes sensibles dans une unique transaction.

```go
err := svc.Settings.ChangePassphrase(ctx, "ancienne-passphrase", "nouvelle-passphrase")
if err != nil {
    log.Fatal(err)
}
log.Println("Passphrase mise à jour.")
```

`ChangePassphrase` :
1. Vérifie l'ancienne passphrase via le canary.
2. Génère un nouveau sel KEK.
3. Dérive la nouvelle KEK.
4. Dans une transaction unique : re-wrap chaque colonne chiffrée + met à jour `Setting.kek_salt` + `Setting.kek_canary`.
5. Remplace la KEK en mémoire + l'efface.

Si la transaction échoue, la DB reste cohérente avec l'ancienne passphrase.

---

## Recette 10

**Re-issue d'une licence (remplace une licence existante).**

Re-issue est utilisé pour étendre la durée, modifier les bindings ou mettre à jour le payload d'une licence existante. La licence originale passe en `status=superseded` et la nouvelle porte `replaces_license_id`.

```go
import "github.com/oioio-space/maldev/internal/manager/service"

originalID := uuid.MustParse("73f56081-...")

reissued, err := svc.License.ReIssue(ctx, originalID, service.ReIssueOptions{
    // Uniquement les champs à modifier — les autres sont hérités de l'original.
    NotAfter: time.Now().Add(180 * 24 * time.Hour),
    Features: []string{"pro", "extended"},
})
if err != nil {
    log.Fatal(err)
}

log.Printf("Nouvelle licence : %s", reissued.Row.LicenseUUID)
log.Printf("Remplace : %s", originalID)
```

L'original est listé dans l'audit avec `kind=license.supersede`. La chaîne `replaces_license_id` est navigable via `svc.License.Get(ctx, id)` (champ `Row` + `Successors` dans `LicenseDetail`).

---

## Voir aussi

- [Concepts](./concepts.md)
- [Configuration](./configuration.md)
- [License framing — Cookbook](../license/workflow.md)
- [FAQ licence](../license/faq.md)
