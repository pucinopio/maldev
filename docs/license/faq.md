---
package: github.com/oioio-space/maldev/license
---

# License — FAQ

Questions et réponses pratiques.

## Démarrage

### Quel est le code minimal côté binaire ?

```go
pub, kid, _ := license.LoadPublicKey("issuer.pub")
if _, err := license.VerifyFile("user.license",
    license.Trusted{Keys: license.SingleKey(kid, pub)}); err != nil {
    log.Fatal("ACCESS DENIED")
}
```

Côté émetteur :

```go
license.GenerateAndSave("./keys", "k1") // une fois
data, _ := license.New(priv, "alice@example.com", 30*24*time.Hour)
license.SaveLicense("./alice.license", data)
```

### Faut-il un serveur ou une base de données ?

Non. Le mode hors-ligne est le mode par défaut. Une licence est un fichier autonome ; la vérification ne fait aucun appel réseau. Le serveur n'est nécessaire que si tu veux la révocation à distance (`WithRevocation`) ou le heartbeat (`WithHeartbeat`).

### Quelles dépendances le package ajoute-t-il ?

`golang.org/x/crypto` (argon2, hkdf, chacha20poly1305, curve25519) — déjà présent dans le `go.sum` du repo. Aucune autre dépendance externe.

---

## Cas d'usage courants

### Donner accès à un testeur pour 1 mois

```go
data, _ := license.New(priv, "tester@example.com", 30*24*time.Hour)
license.SaveLicense("./tester.license", data)
```

Au bout de 30 jours, le binaire refuse automatiquement.

### Donner accès à plusieurs testeurs, chacun individuellement révocable

Émets une licence par testeur. Chaque licence a un `ID` distinct (UUID v4 généré à l'émission). Pour révoquer Alice sans affecter Bob, mets l'`ID` d'Alice dans la revocation list.

Voir [Recette 11](./workflow.md#recette-11) pour la génération en batch.

### Limiter une licence à une machine spécifique

```go
import "github.com/oioio-space/maldev/license/hostid"

// Sur la machine cible :
me, _ := hostid.Local()
fmt.Printf("%x\n", me) // → tu envoies cette valeur à l'émetteur

// Côté émetteur :
Bindings: []license.Binding{
    license.BindMachineIDs(string(me)),
},
```

Voir [Recette 3](./workflow.md#recette-3).

### Période d'essai gratuite

`NotAfter = time.Now().Add(14 * 24 * time.Hour)` à l'émission. Le binaire refuse après. Voir [Recette 9](./workflow.md#recette-9) pour un essai par utilisateur.

### Niveaux de licence (basic / pro / enterprise)

Mets le tier dans le payload signé :

```go
type Tier struct {
    Level    string   `json:"level"`
    Features []string `json:"features"`
}

raw, _ := license.MarshalPayload(Tier{Level: "pro", Features: []string{"export", "api"}})
data, _ := license.Issue(license.IssueOptions{
    PrivateKey: priv, KeyID: "k1", Subject: "client",
    NotAfter: time.Now().Add(365 * 24 * time.Hour),
    Payload: raw,
})
```

Côté binaire :

```go
v, _ := license.Verify(data, trusted)
tier, _ := license.PayloadAs[Tier](v)
if !slices.Contains(tier.Features, "export") {
    return errors.New("feature not available in your tier")
}
```

Voir [Recette 10](./workflow.md#recette-10).

### Plusieurs binaires partagent la même clé publique

C'est le cas typique. Une seule paire de clés émet pour `rshell`, `memscan`, etc. Utilise `Audience` pour scoper :

```go
// licence rshell-only :
license.Issue(license.IssueOptions{Audience: []string{"rshell"}, ...})

// dans chaque binaire :
license.Verify(data, trusted, license.WithAudience("rshell"))
```

Voir [Recette 14](./workflow.md#recette-14).

---

## Sécurité

### Une licence peut-elle être modifiée pour repousser la date d'expiration ?

Non. La signature Ed25519 couvre tous les champs, y compris `NotAfter`. Toute modification invalide la signature et `Verify` rejette.

### Et si quelqu'un patche `Verify` dans le binaire ?

C'est possible et hors scope du package. Le package suppose l'intégrité du binaire. Pour résister à ce scénario, combine avec :

- Le `cmd/packer` du repo (chiffrement du `.text`)
- Une signature OS (Authenticode sur Windows, Apple Notarization sur macOS)
- Des techniques d'anti-tamper externes

### Comment est stocké un mot de passe lié à une licence ?

En `argon2id(password, salt)` — fonction de dérivation lente conçue contre le brute-force. La licence contient `hash` + `salt`, jamais le mot de passe. Paramètres : t=3, m=64 MiB, p=4. Une tentative coûte environ 100 ms.

### Le contenu d'une licence est-il confidentiel ?

Non. Le PEM est trivialement décodable (`base64 -d`). Toute personne qui détient une licence peut lire `Subject`, `Issuer`, `Audience`, `NotAfter`, et le contenu de `Payload`. Pour du confidentiel, utilise `SealedPayload` (chiffré X25519 + XChaCha20-Poly1305 pour un destinataire spécifique). Voir [Recette 8](./workflow.md#recette-8).

### L'utilisateur peut-il modifier l'horloge système pour contourner `NotAfter` ?

Pas indéfiniment. Avec `WithStateFile(path)` + `WithStateHostID(hostid.Local)`, le binaire mémorise dans un fichier HMAC :

- le plus récent `time.Now()` observé localement,
- le plus récent `server_time` signé par la revocation list ou le heartbeat.

Un `time.Now()` antérieur déclenche `causeClockRollback`. L'utilisateur peut effacer le state file, mais le prochain contact serveur rétablit le plancher signé. Sans aucun contact serveur, la protection se limite au "plus récent local observé".

### Que faire si ma clé privée fuite ?

1. Génère une nouvelle paire avec un nouveau `KeyID` (ex. `k2026-05-emergency`).
2. Release des binaires avec **les deux** clés publiques dans `Trusted.Keys`.
3. Émets de nouvelles licences avec le nouveau `KeyID`.
4. Révoque toutes les licences signées par l'ancienne clé.
5. Attends la fin de la fenêtre de migration.
6. Release suivante : retire l'ancienne clé publique de `Trusted.Keys`.

Voir [Recette 7](./workflow.md#recette-7).

### Que se passe-t-il si une licence légitime fuite ?

Mets son `ID` dans la revocation list (`POST /revoked.pem` avec ton admin token). Au prochain refresh de la liste (configurable, typ. 24h), tous les binaires refuseront.

---

## Comportements et limites

### Combien de licences puis-je émettre ?

Pas de limite pratique. Chaque licence a un UUID v4 (2^122 valeurs) — collision impossible.

### Quelle taille fait une licence ?

500–1500 octets pour une licence typique. Limite dure : `MaxLicenseSize = 16 KiB`. Au-delà, refus avant parse.

### Combien coûte un `Verify` ?

| Configuration | Coût typique |
|---|---|
| Minimal (signature + dates) | < 1 ms |
| + binding password (argon2id) | ~100 ms |
| + revocation HTTP | dépend du réseau (50-500 ms) |
| + heartbeat HTTP | idem |
| + binary pinning | ~10 ms première fois, 0 après (cached via `sync.Once`) |

Pour les binaires lancés en boucle, fais `Verify` une fois au démarrage et cache le résultat.

### Linux ? macOS ? Windows ?

Les trois. `license/hostid` a des sources d'identifiant par plateforme :

- Windows : `HKLM\SOFTWARE\Microsoft\Cryptography\MachineGuid`
- Linux : `/etc/machine-id` (fallback `/var/lib/dbus/machine-id`)
- macOS : `IOPlatformUUID` via `ioreg`

Le reste du package est pure Go stdlib + `golang.org/x/crypto`.

### Et si l'utilisateur n'a pas de réseau ?

Mode hors-ligne pur : n'utilise ni `WithRevocation` ni `WithHeartbeat`. La licence est auto-suffisante jusqu'à `NotAfter`.

Mode hybride : `WithRevocation(...)` + `WithGracePeriod(7*24*time.Hour)`. Le binaire tolère 7 jours sans contact serveur, après quoi il refuse.

### Que se passe-t-il si le state file n'est pas accessible en écriture ?

Le state file est optionnel (activé par `WithStateFile`). Sans état, tu perds la détection anti-rollback d'horloge mais le reste fonctionne. Une erreur d'écriture du state file logue un warning et ne refuse pas la licence.

### Mon CI tourne dans un container minimal sans `/etc/machine-id`

Le binding `machine` est optionnel. Pour le CI ou les tests, ne l'utilise simplement pas. Tu peux aussi injecter une valeur connue via `WithMachineID([]byte("ci-runner"))`.

---

## Debug

### Le binaire affiche "license: verification failed", comment connaître la cause ?

Le message est volontairement opaque — il ne dit pas pourquoi le check a échoué pour ne pas guider un attaquant. La cause précise va dans le logueur :

```go
import "log/slog"

logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
license.Verify(data, trusted, license.WithLogger(logger))
```

Tu verras `WARN license verify failed cause=binding-machine-mismatch` ou équivalent.

### Causes les plus fréquentes

| Cause | Sens | Action |
|---|---|---|
| `expired` | dépassé `NotAfter` | inspecter avec `license.Inspect(data)` |
| `binding-machine-mismatch` | mauvaise machine | comparer `hostid.Local()` aux IDs autorisés |
| `binding-password-mismatch` | mauvais mot de passe | re-saisir |
| `unknown-key` | `Trusted.Keys` ne contient pas le `KeyID` | mettre à jour la clé publique embarquée |
| `bad-signature` | licence modifiée OU mauvaise clé publique | vérifier qu'on utilise la bonne `issuer.pub` |
| `bad-format` | PEM corrompu ou JSON invalide | re-télécharger la licence |
| `audience-mismatch` | mauvaise `Audience` | vérifier que `WithAudience("X")` matche `License.Audience` |
| `clock-rollback` | horloge système avant `trusted_floor` | corriger l'horloge ou (debug) supprimer le state file |
| `revoked` | licence sur la revocation list | obtenir une nouvelle licence |

### Une licence "passe" `Inspect` mais échoue à `Verify`

C'est normal. `Inspect` lit le contenu sans vérifier la signature — c'est un outil de diagnostic. Si `Verify` rejette avec `bad-signature`, soit la licence a été modifiée, soit tu utilises la mauvaise clé publique.

### Tester sans infrastructure

N'active pas `WithRevocation` ni `WithHeartbeat`. La vérification hors-ligne suffit pour tester la logique d'émission, les bindings, les dates, l'audience. Voir [Recette 13](./workflow.md#recette-13).

---

## Performance & opérations

### Capacité du serveur de révocation

Stateless, signe la liste à chaque requête. Sur un VPS modeste, plusieurs milliers de requêtes/s. Signature Ed25519 de quelques KiB : < 1 ms.

### Limite du nombre d'IDs dans une revocation list

Pas de limite dure. Côté client, le source HTTP limite à 1 MiB par réponse (~25 000 IDs). Au-delà, segmente avec `revoke.MultiSource`.

### Logs côté production

Le logueur reçoit chaque succès et chaque échec en JSON structuré. Recommandation : pipe vers ton stack d'observabilité (Loki, ELK, Splunk) et alerte sur `cause=clock-rollback` ou `cause=binding-password-mismatch` répétés (indices d'abus).

---

## Voir aussi

- [Concepts](./concepts.md)
- [Cookbook](./workflow.md)
- [Référence des champs](../techniques/license-framing.md#référence-des-champs)
- [Threat model](./threat-model.md)
