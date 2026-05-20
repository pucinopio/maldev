---
package: github.com/oioio-space/maldev/license
---

# License — Concepts

Cette page pose le modèle mental du package : ce qu'est une licence, comment elle se vérifie, ce qu'elle garantit, et ce qu'elle ne garantit pas. Aucun pré-requis crypto.

## Qu'est-ce qu'une licence ici

Une licence est un **document JSON signé** par toi (l'émetteur), qui déclare une autorisation et ses conditions. Exemple narratif :

> "Cette licence autorise `alice@example.com` à exécuter `rshell` jusqu'au 31 décembre 2026, uniquement sur les machines A, B ou C, à condition de fournir le mot de passe `hunter2` au démarrage."

Le document est encodé en PEM (un format texte standard, lisible) :

```
-----BEGIN MALDEV LICENSE-----
eyJsaWMiOnsidiI6MSwiaWQiOiI3M2Y1NjA4MS01Y2NlLTQwNzMtOTYzMi0...
-----END MALDEV LICENSE-----
```

Une fois décodé en base64, c'est du JSON :

```json
{
  "lic": {
    "v": 1,
    "id": "73f56081-5cce-4073-9632-...",
    "kid": "k2026-05",
    "sub": "alice@example.com",
    "aud": ["rshell"],
    "iat": "2026-05-20T10:00:00Z",
    "exp": "2026-08-18T10:00:00Z",
    "bnd": [
      {"t": "machine", "v": ["abc123", "def456"]},
      {"t": "password", "h": "...", "s": "..."}
    ]
  },
  "sig": "<64 octets Ed25519>",
  "kid": "k2026-05"
}
```

La **signature** (`sig`) couvre tous les champs de `lic`. Modifier un seul octet de `lic` invalide la signature : le binaire détectera la modification et refusera.

## Quelle clé distribuer

Le système repose sur **deux** clés liées mathématiquement par Ed25519. Elles n'ont absolument pas le même statut :

| Clé | Statut | À faire | À ne pas faire |
|---|---|---|---|
| **Privée** (`MALDEV PRIVATE KEY`, 64 octets) | Secret absolu | La garder hors-ligne, idéalement sur un poste dédié ou un HSM. Sauvegarder chiffré. Permissions `0600`. | Ne **jamais** la committer, la pusher sur un repo, l'envoyer par email, la stocker dans un container CI, l'embarquer dans un binaire. Si elle fuite, **tout est compromis** : il faut rotation immédiate. |
| **Publique** (`MALDEV PUBLIC KEY`, 32 octets) | Information publique | La distribuer avec chaque binaire, la committer, la mettre dans un README, l'embarquer en dur via `//go:embed`. | Rien d'interdit. Ce qu'on craint avec la clé privée (forger des licences) est impossible avec la publique. |

> En cas de doute : la **publique** se reconnaît à son en-tête `-----BEGIN MALDEV PUBLIC KEY-----` ; la **privée** à `-----BEGIN MALDEV PRIVATE KEY-----`. La privée fait ~115 octets en PEM, la publique ~80 octets.

### Embarquer la clé publique avec `//go:embed`

Le pattern recommandé pour les binaires distribués : commit `issuer.pub` à côté du `main.go`, embarque-le à la compilation, parse-le au démarrage. Aucun fichier externe à packager.

```go
package main

import (
	_ "embed"
	"log"

	"github.com/oioio-space/maldev/license"
)

//go:embed issuer.pub
var issuerPub []byte

func main() {
	pub, kid, err := license.ParsePublicKey(issuerPub) // []byte → ed25519.PublicKey + KID
	if err != nil {
		log.Fatalf("clé publique corrompue : %v", err)
	}
	trusted := license.Trusted{Keys: license.SingleKey(kid, pub)}

	if _, err := license.VerifyFile("user.license", trusted); err != nil {
		log.Fatal("ACCESS DENIED")
	}
}
```

Toutes les fonctions de chargement existent en deux variantes : **file-based** (`LoadPublicKey`, `LoadPrivateKey`, `LoadLicense`, `VerifyFile`) qui prennent un chemin, et **bytes-based** (`ParsePublicKey`, `ParsePrivateKey`, `Verify`) qui prennent un `[]byte` directement utilisable avec `//go:embed`.

## Trois rôles, deux clés

```mermaid
flowchart LR
    A["Toi<br/>(émetteur)"] -- "signe avec<br/>clé privée" --> L["licence.pem"]
    L --> U["Utilisateur"]
    U --> B["Binaire<br/>(consommateur)"]
    A -. "distribue<br/>clé publique" .-> B
    B -- "vérifie avec<br/>clé publique" --> R["accepte / refuse"]
```

- L'**émetteur** détient la clé privée. Il signe les licences.
- L'**utilisateur** reçoit une licence (un fichier). Il ne signe rien.
- Le **binaire consommateur** embarque la clé publique et vérifie les licences présentées.

La clé privée est le seul secret du système. Tant qu'elle reste privée, personne ne peut émettre de licence valide en ton nom. La clé publique est, comme son nom l'indique, publique : elle se distribue avec le binaire.

## Vocabulaire

| Terme | Définition |
|---|---|
| **Issuer** | Toi. La signature t'identifie. Tu peux aussi mettre ton nom dans le champ `Issuer` de la licence (texte libre, signé). |
| **Subject** | Le destinataire de la licence. Texte libre (email, agent, hostname…). Sert d'identifiant humain et figure dans les logs. |
| **KeyID** (`kid`) | Le nom court de la clé qui a signé (ex. `"k2026-05"`). Permet de tourner les clés sans casser les licences existantes : plusieurs `kid` peuvent coexister côté consommateur. |
| **Audience** (`aud`) | Liste des binaires pour lesquels la licence est valable. Si vide, la licence est valable partout (avec warning). |
| **Binding** | Une contrainte spécifique : machine, mot de passe, paire clé/valeur custom. Lors de la vérification, l'appelant fournit l'« évidence » qui doit matcher le binding. |
| **Pinning** | Lier la licence à un binaire précis, soit par hash du fichier (`BinarySHA256`), soit par identité embarquée (`IdentitySHA256`). Empêche la réutilisation avec un binaire modifié. |
| **Revocation** | Annulation après émission. Le serveur publie une liste signée d'IDs révoqués ; le binaire la consulte avant d'autoriser. |
| **Heartbeat** | Ping périodique vers un serveur qui répond signé si la licence est toujours active. Tolérance offline configurable via grace period. |
| **State file** | Fichier local HMAC qui mémorise le plus récent `time.Now()` et le plus récent `server_time` observés. Détecte le rollback d'horloge. |

## Le cycle de vie

```mermaid
sequenceDiagram
    autonumber
    participant You as "Émetteur"
    participant Bin as "Binaire"
    participant Srv as "Serveur révocation (optionnel)"

    You->>You: GenerateAndSave(dir, kid)
    Note over You: une fois par rotation
    You->>Bin: distribue issuer.pub
    Note over Bin: au moment du build
    You->>You: Issue(IssueOptions{...})
    You->>Bin: distribue licence.pem
    Note over Bin: au moment de l'onboarding
    Bin->>Bin: Verify(licence, pub)
    Bin->>Srv: (optionnel) fetch revocation list
    Srv-->>Bin: signed list
    Bin-->>You: autorisé / ErrLicenseInvalid
```

Le **mode hors-ligne pur** (les deux premières étapes seulement, sans serveur) couvre la majorité des cas : le binaire vérifie offline, et la licence expire d'elle-même au `NotAfter`.

Le **mode en-ligne** s'ajoute si tu veux pouvoir révoquer une licence avant son expiration (fuite, fin de contrat) ou imposer un check serveur récurrent.

## Pourquoi pas un simple mot de passe ?

| | Mot de passe partagé | Licence signée |
|---|---|---|
| Identifier qui utilise quoi | non | oui (`Subject` par licence) |
| Expiration automatique | non | oui (`NotAfter`) |
| Révocation ciblée | non (tu change le mot de passe pour tous) | oui (par `ID` de licence) |
| Lier à une machine | non | oui (binding `machine`) |
| Scope par binaire | non | oui (`Audience`) |
| Données métier attachées et signées | non | oui (`Payload` typé) |
| Résiste à la modification du token | non | oui (signature Ed25519) |

## Garanties

Le package garantit, sous l'hypothèse que la clé privée reste confidentielle :

- **Authenticité** : seule la clé privée peut produire une licence acceptée. Une licence forgée échoue à `Verify`.
- **Intégrité** : toute modification post-signature (même d'un seul bit) est détectée.
- **Non-réutilisation cross-audience** : une licence émise pour `aud=["rshell"]` ne validera pas un `WithAudience("memscan")`.
- **Non-réutilisation cross-binaire** : si tu actives `WithBinaryPinning()`, une licence émise pour le binaire A ne validera pas le binaire B.
- **Anti-replay de revocation list ancienne** : la liste est signée, expirable, et porte un numéro de séquence monotone.
- **Anti-brute-force des bindings password** : argon2id avec paramètres tuned (~100 ms par tentative).
- **Anti-rollback d'horloge sous le plancher signé** : si le state file est activé et a déjà observé un `server_time`, un `time.Now()` antérieur est rejeté.

## Limitations honnêtes

- **Le binaire peut être patché.** Un attaquant qui modifie `Verify` pour `return nil` contourne tout. Ce scénario relève du hardening binaire (packer, code-signing OS) — hors scope de ce package.
- **Une licence partagée volontairement reste utilisable.** Si Alice donne sa licence et son mot de passe à Bob, Bob s'en sert. Le binding `machine` mitige partiellement.
- **L'usage offline indéfini est limité par `GracePeriod`** au-delà duquel le binaire refuse. Sans heartbeat ni revocation, l'expiration repose uniquement sur `NotAfter`.
- **Le `Payload` non scellé est lisible** par quiconque détient la licence. Pour du contenu confidentiel, utilise `SealedPayload` (chiffré pour un destinataire X25519 précis).
- **`hostid.Local()` est falsifiable** par un attaquant qui contrôle entièrement la machine.

## Anatomie de `Verify`

`Verify` exécute les vérifications dans l'ordre suivant. La première qui échoue retourne `ErrLicenseInvalid` (cause précise loguée mais absente du message d'erreur).

1. Format PEM + taille (< 16 KiB).
2. Résolution du `KeyID` dans `Trusted.Keys`.
3. Signature Ed25519 sur `domain_tag || canonical(License)`.
4. Lecture du state file (HMAC vérifié). Si `time.Now() < trusted_floor` → refus.
5. `NotBefore` ≤ now + skew ≤ `NotAfter`.
6. `WithAudience` ∈ `License.Audience` et `WithIssuer` = `License.Issuer`.
7. Tous les `Bindings` matchent les évidences fournies.
8. Si `WithBinaryPinning()` : `BinarySHA256` et/ou `IdentitySHA256` matchent.
9. Si `WithRevocation` : la licence n'est pas révoquée et la liste est fraîche.
10. Si `WithHeartbeat` : le serveur confirme `ok: true` (skip si une réponse OK a moins de `interval`).
11. Si `WithNTPCheck` : la dérive d'horloge est tolérable.
12. Écriture atomique du state file mis à jour.

Tout est ordonné `cheap → expensive` : les checks crypto sont avant les checks réseau.

## Première lecture recommandée

| Si tu veux… | Va à |
|---|---|
| Émettre et vérifier ta première licence | [Cookbook Recette 1](./workflow.md#recette-1) |
| Comprendre tous les champs du JSON | [Référence des champs](../techniques/license-framing.md#référence-des-champs) |
| Distribuer un binaire avec licence à plusieurs personnes | [Recette 2](./workflow.md#recette-2) et [Recette 11](./workflow.md#recette-11) |
| Limiter une licence dans le temps + à une machine | [Recette 3](./workflow.md#recette-3) |
| Embarquer des données applicatives signées | [Recette 7-bis](./workflow.md#recette-7-bis--payload-applicatif-typé) |
| Mettre en place la révocation | [Recette 5](./workflow.md#recette-5) |
| Comprendre les menaces couvertes et celles qui ne le sont pas | [Threat model](./threat-model.md) |
| Trouver une réponse rapide à un cas concret | [FAQ](./faq.md) |

## Voir aussi

- [Cookbook (recettes copier-coller)](./workflow.md)
- [Référence complète des champs](../techniques/license-framing.md)
- [Threat model](./threat-model.md)
- [FAQ](./faq.md)
