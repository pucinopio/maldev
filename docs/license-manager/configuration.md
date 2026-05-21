---
package: github.com/oioio-space/maldev/cmd/license-manager
---

# License Manager — Configuration

Référence des flags CLI, variables d'environnement, valeurs par défaut des serveurs, et préréglages Argon2id.

## Flags CLI

| Flag | Type | Défaut | Description |
|------|------|--------|-------------|
| `--db` | string | `./manager.db` | Chemin vers la base SQLite. Créée au premier lancement. |
| `--passphrase-file` | string | — | Fichier contenant la passphrase (lue + trimée). Priorité maximale dans la cascade. |
| `--no-tui` | bool | false | Désactive la TUI bubbletea. Utile pour les flux scriptés ou le débogage. |

Exemples :

```bash
# Lancement standard
license-manager --db /var/lib/maldev/manager.db

# Passphrase depuis fichier (recommandé pour les flux CI/scripts)
license-manager --db manager.db --passphrase-file /run/secrets/mgr_pass

# Mode script sans TUI
license-manager --db manager.db --no-tui
```

## Variables d'environnement

| Variable | Description | Priorité dans la cascade |
|----------|-------------|--------------------------|
| `MALDEV_MGR_PASSPHRASE_FILE` | Chemin vers un fichier passphrase. Équivalent à `--passphrase-file` mais sans flag. | 2 (après `--passphrase-file`) |
| `MALDEV_MGR_PASSPHRASE` | Passphrase directement en valeur. À éviter si `ps` expose les variables d'environnement. | 3 |

La cascade complète de résolution de passphrase :

```
1. flag --passphrase-file          (priorité maximale)
2. env  MALDEV_MGR_PASSPHRASE_FILE
3. env  MALDEV_MGR_PASSPHRASE
4. (v2) OS keystore (DPAPI / Keychain / libsecret)
5. prompt interactif TUI           (fallback)
```

Dès qu'une source produit une valeur non vide, les suivantes sont ignorées. Quand la passphrase est résolue silencieusement (étapes 1–3), aucun prompt n'apparaît.

## Valeurs par défaut des serveurs

Configurables via `SettingsService.UpdateServerConfig`. Toutes les adresses d'écoute sont vides par défaut — un serveur avec adresse vide ne peut pas être démarré.

| Champ `ServerConfig` | Valeur par défaut | Description |
|----------------------|-------------------|-------------|
| `revocation_listen` | `""` | Adresse d'écoute du serveur Revocation (ex. `":8443"`) |
| `revocation_path` | `"/revoked.pem"` | Chemin de la CRL |
| `revocation_tls_cert` | `""` | Chemin PEM du certificat TLS (vide = HTTP) |
| `revocation_tls_key` | `""` | Chemin PEM de la clé TLS |
| `heartbeat_listen` | `""` | Adresse d'écoute du serveur Heartbeat (ex. `":8444"`) |
| `heartbeat_path` | `"/heartbeat"` | Chemin du endpoint heartbeat |
| `heartbeat_tls_cert` | `""` | |
| `heartbeat_tls_key` | `""` | |
| `probe_listen` | `""` | Adresse d'écoute du serveur Probe (ex. `":8445"`) |
| `probe_tls_cert` | `""` | |
| `probe_tls_key` | `""` | |
| `probe_default_ttl_seconds` | `86400` | TTL par défaut des tokens probe (24h) |

Exemple de configuration minimale via `UpdateServerConfig` :

```go
svc.Settings.UpdateServerConfig(ctx, func(u *ent.ServerConfigUpdate) {
    u.SetRevocationListen(":8443").
        SetHeartbeatListen(":8444").
        SetProbeListen(":8445")
})
```

## Préréglages Argon2id

Le champ `Setting.default_argon_preset` contrôle les paramètres utilisés pour hacher les bindings `password` lors de l'émission. Les préréglages s'appliquent aussi à la dérivation KEK (`crypto.DeriveFromPassphrase`).

| Préréglage | `time` | `memory` | `threads` | Latence approx. |
|------------|--------|----------|-----------|-----------------|
| `fast` | 1 | 32 MiB | 2 | ~30 ms |
| `default` | 3 | 64 MiB | 4 | ~100 ms |
| `paranoid` | 8 | 256 MiB | 8 | ~800 ms |

- **`fast`** : acceptable pour des tests et des environnements à faible mémoire.
- **`default`** : recommandé pour la production. Résiste aux attaques GPU modernes.
- **`paranoid`** : pour des secrets à très haute valeur (clés de root CA, etc.). Prévoir ~1 s à chaque démarrage.

Modifier le préréglage via `SettingsService.Update` :

```go
svc.Settings.Update(ctx, func(u *ent.SettingUpdate) {
    u.SetDefaultArgonPreset(setting.DefaultArgonPresetParanoid)
})
```

## Permissions recommandées

| Ressource | Permission | Raison |
|-----------|------------|--------|
| `manager.db` | `600` (owner RW seulement) | Contient les clés privées chiffrées |
| Fichier passphrase | `400` (owner R seulement) | Lecture par le processus uniquement |
| Binaires probe (`agents/*`) | `755` | Exécution sur machines distantes |
| Répertoire `probe/agents/gen/` | Accès build uniquement | Source du binaire agent — ne pas distribuer |

## Paramètres opérateur (`Setting`)

| Champ | Défaut | Description |
|-------|--------|-------------|
| `default_issuer_name` | `""` | Valeur pré-remplie pour le champ `iss` dans les nouvelles licences |
| `default_audience` | `[]` | Audiences pré-sélectionnées dans le formulaire d'émission |
| `default_ttl_seconds` | `2592000` (30j) | TTL par défaut des nouvelles licences |
| `default_argon_preset` | `default` | Voir tableau Argon2id ci-dessus |
| `operator_name` | `""` | Nom affiché dans le champ `actor` de l'audit trail |
| `auto_start_servers` | `false` | Démarrer les serveurs configurés au boot |
| `confirm_quit_with_servers` | `true` | Confirmer avant de quitter si des serveurs tournent |

## Voir aussi

- [Concepts](./concepts.md)
- [Cookbook](./workflow.md)
