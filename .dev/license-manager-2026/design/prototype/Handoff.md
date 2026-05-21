# `license-manager` — TUI design handoff

> Document auto-suffisant pour implémenter la TUI en Go. Tu ne connais pas la
> conversation de design qui l'a produit ; tout ce qu'il te faut est ici.
> Lecture ~15 min, implémentation ~2-3 jours.

---

## 1. Synthèse du périmètre

- **Outil** : `license-manager`, TUI Go locale pour issuer une lib offensive-security.
- **Persona** : un seul opérateur, niveau technique élevé, keyboard-first.
- **Backend** : un struct unique **`*service.Services`** injecté au boot. Bundle les sous-services `Issuer`, `License`, `Revoke`, `Identity`, `Recipient`, `TOTP`, `Probe`, `Audit`, `Settings`. La TUI **n'accède jamais aux stores ni au réseau directement** ; toutes les opérations métier passent par ce bundle (cf §10 pour la signature complète).
- **httpsrv** : un **`*httpsrv.Bundle`** singleton (Revocation, Heartbeat, Probe). Expose `Start/Stop/Status/Events()` par serveur + un fan-in `MergedEvents() <-chan Event` consommé en streaming par la TUI pour les live logs.
- **Passphrase cascade au boot** (résolution dans cet ordre, premier non-vide gagne) :
  1. `--passphrase-file <path>` (CLI)
  2. env `MALDEV_MGR_PASSPHRASE_FILE`
  3. env `MALDEV_MGR_PASSPHRASE`
  4. fallback : prompt TUI interactif (cf §3.12, 3 tentatives max)
  Si la cascade résout silencieusement, l'écran passphrase **n'est pas affiché** — l'app va directement au Dashboard (ou au wizard "première utilisation" si DB inexistante).
- **Stack TUI** : Go ≥ 1.22, `charmbracelet/bubbletea` + `lipgloss` + `bubbles`. `huh` autorisé pour les wizards si utile.
- **Local-first** : DB SQLite chiffrée sur disque, offline-capable. Les serveurs HTTP sont **OFF au lancement** (sauf si `Settings.auto_start_servers`), démarrés à la demande. À la fermeture, modal de confirmation si serveur ON ET `Settings.confirm_quit_with_servers`.
- **Zéro friction** : 1-2 touches pour les actions fréquentes. Wizards préférés aux forms plats. Défauts hérités de Settings. Tout artefact produit (PEM, QR, secrets, tokens) présenté avec actions copier/sauver/exporter explicites.
- **Surface fonctionnelle** : 9 onglets globaux (Dashboard, Licenses, Issuer keys, Recipients, Identities, Revocation, Servers, Audit, Settings) + wizard 8-étapes pour nouvelle licence + drawer fingerprint probe + onboarding première-utilisation + prompt passphrase (conditionnel).
- **Status licences** : `active`, `expiring`, `expired`, `revoked`, **`superseded`** (re-émise plus tard, non-utilisable ni re-émissible).
- **Style visuel** : palette néon (fond profond, bold + couleur saturée — pas de glow réel, cf §11), monospace, densité dense-mais-aérée façon `lazygit`. Constantes lipgloss en §9.

---

## 2. Structure de fichiers

```
cmd/license-manager/
└── main.go                 # entrypoint : flags, ouvre la DB ou route vers onboarding, lance tea.Program

internal/manager/tui/
├── app.go                  # root model, router, msg dispatcher, lifecycle
├── theme.go                # constantes lipgloss (couleurs, styles)
├── keys.go                 # keybindings globaux (bubbles/key.Binding)
├── chrome.go               # titlebar + tab strip + status bar + breadcrumb (vue partagée)
├── overlay.go              # type Overlay (interface), stack, dispatcher
│
├── screen_dashboard.go     # §3.1
├── screen_licenses.go      # §3.2 — list + detail (expand row)
├── wizard_license.go       # §3.3 — 8 étapes (StepIdentity, StepValidity, ...)
├── screen_issuers.go       # §3.4
├── screen_recipients.go    # §3.5
├── screen_identities.go    # §3.6
├── screen_revocation.go    # §3.7
├── screen_servers.go       # §3.8 — 3 sous-onglets (Revoc, Heartbeat, Probe)
├── screen_audit.go         # §3.9
├── screen_settings.go      # §3.10
├── screen_onboarding.go    # §3.11 — first-run wizard 4 étapes
├── screen_passphrase.go    # §3.12 — prompt passphrase (DB existante)
│
├── overlay_confirm.go      # confirmations + quit
├── overlay_revoke.go       # révocation d'une licence
├── overlay_qr.go           # QR ASCII + export PNG/SVG
├── overlay_probe.go        # drawer fingerprint probe (utilisé depuis wizard ou Servers)
├── overlay_filepicker.go   # wrapper bubbles/filepicker
├── overlay_error.go        # modal d'erreur avec recover
├── overlay_help.go         # ? aide globale
│
└── cmds/                   # tea.Cmd wrappers thin sur *service.Services et *httpsrv.Bundle
    ├── licenses.go         # ListLicensesCmd, IssueLicenseCmd, ReissueLicenseCmd, RevokeLicenseCmd, ...
    ├── keys.go             # ListIssuerKeysCmd, GenerateIssuerKeyCmd, ImportPEMCmd, ExportPubCmd, ...
    ├── identities.go       # ListIdentitiesCmd, CreateIdentityCmd, ExportBinCmd, RegenIdentityCmd, ...
    ├── audit.go            # ListAuditCmd, ListAuditForTargetCmd, ExportCSVCmd, ExportJSONCmd
    ├── crl.go              # ListCRLCmd, AddRevocationCmd, RemoveRevocationCmd, PublishSignedListCmd
    ├── probe.go            # NewProbeTokenCmd, SubscribeProbeCmd, ProbeHistoryCmd, RevokeProbeCmd
    ├── hostid.go           # hostid.Local() / hostid.Composite() (in-process, pas via Services)
    └── settings.go         # GetSettingsCmd, UpdateSettingsCmd, ChangePassphraseCmd

# Tous les fichiers cmds/* prennent *service.Services en paramètre constructor :
#
#   func ListLicensesCmd(svc *service.Services, f LicenseFilter) tea.Cmd {
#       return func() tea.Msg {
#           rows, err := svc.License.List(context.TODO(), f)
#           if err != nil { return BackendErrorMsg{err} }
#           return ListLicensesMsg{rows}
#       }
#   }
#
# *service.Services est instancié dans main.go après résolution de la cascade passphrase,
# puis injecté dans rootModel.backend (cf §4).
```

> Convention : un écran = un fichier `screen_*.go`, un overlay = un `overlay_*.go`.
> Tout l'IO backend transite par `cmds/*` qui renvoie une `tea.Cmd` ; jamais d'appel synchrone dans `Update`.

---

## 3. Écrans

Chaque sous-section suit la même structure :
**Rôle · Mock ASCII · Bubbles · Model fields · Msg · Cmd · Touches**.

### 3.1 Dashboard — point d'entrée

**Rôle** : compteurs, clé active, état serveurs, 5 dernières actions, raccourcis vers chaque domaine.

```
┌────────────────────────────────────────────────────────────────────────────────────────────┐
│ Actives [a]      Révoquées [r]     Expirées [e]      Expirent < 7j [w]                     │
│ 47 (good)        6 (danger)        12 (mute)         4 (warn)                              │
│ signées avec     présentes dans    NotAfter dépassé  à renouveler / prolonger              │
│ la clé active    la CRL                                                                    │
├──────────────────────────────────────────┬─────────────────────────────────────────────────┤
│ CLÉ D'ÉMISSION ACTIVE              [k]   │ 5 DERNIÈRES ACTIONS                [8] tout l'audit│
│ k2026-04                  [ACTIVE]       │ 13:42:18  license.issue   lic:9f3a… (alice@…)   │
│ nom rshell-prod-2026Q2                   │ 13:38:02  key.activate    k2026-04              │
│ fpr ed25519:a4f2…91bc                    │ 13:22:55  license.revoke  lic:71bd… reason:…    │
│                                          │ 11:09:11  server.start    revocation :8443      │
│ SERVEURS HTTP                  [7] [s]   │ 10:58:00  identity.new    rshell-windows-amd64  │
│ ● revocation :8443  ON  url… 1289 req    │                                                 │
│ ● heartbeat :8444   ON  url… 5132 req    ├─────────────────────────────────────────────────┤
│ ○ probe :8445       OFF arrêté · [7]     │ RACCOURCIS                                      │
│                                          │ [n] nouvelle  [/] rechercher  [x] révoquer      │
│                                          │ [k] clés      [i] identity    [?] aide          │
└──────────────────────────────────────────┴─────────────────────────────────────────────────┘
 1-9 onglets · n nouvelle licence · / rechercher · k clés actives · ? aide · q quitter
```

**Bubbles** : aucun (tout est lipgloss statique). Optionnellement `progress.Model` pour le tile "expirent < 7j" si vous voulez un mini-bar.

**Model**
```go
type dashboardModel struct {
    counters     CountersDTO  // ListCountersMsg
    activeKey    KeyDTO       // ListActiveKeyMsg
    servers      [3]ServerStatus
    recentAudit  []AuditEntry // ListRecentAuditMsg (limit=5)
    refreshing   bool
}
```

**Msg**
| Msg | Émetteur | Effet |
|---|---|---|
| `RefreshDashboardMsg` | `tea.Tick(5s)` + `r` | déclenche les 4 Cmd ci-dessous |
| `CountersMsg{c CountersDTO}` | `services.License.Counters()` | màj counters |
| `ActiveKeyMsg{k KeyDTO}` | `services.Issuer.Active()` | màj activeKey |
| `ServersStatusMsg{s [3]ServerStatus}` | `httpsrv.Bundle.Revocation.Status()` etc. (sync) | màj servers |
| `RecentAuditMsg{a []AuditEntry}` | `services.Audit.List({limit:5})` | màj audit |

**Cmd** : `services.License.Counters`, `services.Issuer.Active`, `services.Audit.List({limit:5})`, `httpsrv.Bundle.*.Status` (sync — lu en place à chaque refresh).

**Touches**
| Touche | Action |
|---|---|
| `1`-`9` | onglet global |
| `a` `r` `e` `w` | jump tile → onglet Licenses filtré sur status correspondant |
| `k` | onglet Issuer keys |
| `i` | onglet Identities |
| `n` | ouvre wizard licence |
| `/` | onglet Licenses, focus recherche |
| `x` | onglet Licenses, prompt révoquer la 1ère ligne (rare ; voir use-case 5) |
| `r` | `RefreshDashboardMsg` |
| `?` | help modal |
| `q` | quit modal (cf §5) |

---

### 3.2 Licenses — liste + détail split-pane (le cœur)

**Rôle** : table filtrable/cherchable. Touche `d` ouvre un panneau de détail **sous** la table (split-pane vertical — cf §11.2), avec 5 onglets internes pour la ligne sélectionnée.

```
┌────────────────────────────────────────────────────────────────────────────────────────────┐
│  / rechercher dans subject…           [█]                       7/12                       │
│  [f] all  active  expiring  expired  revoked  superseded                                   │
├────────────────────────────────────────────────────────────────────────────────────────────┤
│ LICENCES (7)                          [↑↓] nav · [d] détail · [n] nouvelle · [x] révoquer  │
│ STATUS      SUBJECT          ISSUER           AUDIENCE   KEYID    EXPIRES    FEATURES      │
│ ACTIVE      alice@research   research@offsec  rshell     k2026-04 2026-08-14 scan report   │
│▌ACTIVE      bob@research↪    research@offsec  rshell     k2026-04 2026-07-02 scan report  ▐│ ← sel
│ EXPIRING    carol@research   research@offsec  rshell-edu k2026-04 2026-05-24 scan          │
│ SUPERSEDED  alice@research↪  research@offsec  rshell     k2026-01 2026-05-15 scan report   │
│ …                                                                                          │
├────────────────────────────────────────────────────────────────────────────────────────────┤
│ Détail · lic:9f3a-… · bob@research            [d] replier · [I]dent [B]ind [P]EM [A]udit  │
│                                                            [C]haîne   ⇒ actions [c o e x] │
│ ↩ Re-émise depuis lic-rshell-aabc (rshell-demo-v3, expirée 2026-04-30)                     │
│ ─────────────────────────────────────────────────────────────────────────────────────────  │
│ ( contenu de l'onglet courant — Identité / Bindings / PEM viewport / Audit / Chaîne ASCII )│
└────────────────────────────────────────────────────────────────────────────────────────────┘
 ↑↓ nav · d détail · n nouvelle · e re-émettre · / rechercher · f filtre · x révoquer
```

Les flèches `↩` et `↪` dans la colonne SUBJECT signalent la position dans une chaîne de re-émission : `↩` = a un parent, `↪` = a au moins un successeur (navigables dans le détail onglet Chaîne).

**Onglets du détail (touche dédiée)**

| Touche | Onglet | Contenu |
|---|---|---|
| `I` | Identité  | lic_id, subject, issuer, audience, keyid, validité, features, status |
| `B` | Bindings  | bindings décodés (machine OR, password preset argon2id, TOTP secret masqué + alg, k/v) + Pinning (identity sha + binary sha) + Sealed payload (recipient) |
| `P` | PEM       | `viewport` scrollable du PEM signé |
| `A` | Audit     | `services.Audit.ListForTarget(license.ID)` — events filtrés sur cette licence |
| `C` | Chaîne    | rendu ASCII parent → (cette licence) → successeurs. Si status=`superseded`, banner d'avertissement "re-émission refusée — utiliser le successeur" |

**Bubbles** :
- `bubbles/table` pour la grille principale.
- `bubbles/textinput` pour la recherche (top bar, focus via `/`).
- `bubbles/viewport` pour les onglets PEM + Audit scrollables.
- `bubbles/help` pour la dernière ligne.

**Model**
```go
type LicenseStatus string
const (
    StatusActive     LicenseStatus = "active"
    StatusExpiring   LicenseStatus = "expiring"
    StatusExpired    LicenseStatus = "expired"
    StatusRevoked    LicenseStatus = "revoked"
    StatusSuperseded LicenseStatus = "superseded"
)

type LicenseDTO struct {
    ID, Subject, Issuer, Audience, KeyID string
    Status       LicenseStatus
    NotAfter     time.Time
    Features     []string
    ParentID     string   // "" si racine
    SuccessorIDs []string // re-émissions ultérieures (multi)
}

type DetailTab string
const (
    TabIdent DetailTab = "ident"
    TabBind  DetailTab = "bind"
    TabPEM   DetailTab = "pem"
    TabAudit DetailTab = "audit"
    TabChain DetailTab = "chain"
)

type licensesModel struct {
    rows       []LicenseDTO
    filtered   []int
    cursor     int
    detail     bool            // split-pane ouvert
    detailTab  DetailTab
    detailFull *LicenseDetail  // chargé à la demande (Get(id))
    search     textinput.Model
    filter     LicenseFilter
    table      table.Model
    pemView    viewport.Model
    auditView  viewport.Model
    err        error
}

type LicenseFilter struct {
    Status         LicenseStatus  // "" pour all
    Audience, KeyID string
    Feature        string
    ExpiringWithin time.Duration
}
```

**Msg**

| Msg | Effet |
|---|---|
| `ListLicensesMsg{rows []LicenseDTO}` | depuis `services.License.List(filter)` |
| `SearchInputMsg` (interne) | re-filtre |
| `ApplyFilterMsg{f LicenseFilter}` | re-filtre |
| `ToggleDetailMsg` | `d` |
| `SwitchDetailTabMsg{tab DetailTab}` | `I/B/P/A/C` |
| `LicenseDetailLoadedMsg{d LicenseDetail}` | enrichit avec bindings décodés + chain |
| `OpenWizardMsg{prefill *LicenseDTO}` | `n` ou `e` |
| `OpenRevokeMsg{id LicenseID}` | `x` |
| `RevokeDoneMsg{id LicenseID}` | recharge la liste |
| `CopyPEMMsg{id LicenseID}` | clipboard |
| `SavePEMMsg{id LicenseID, path string}` | filepicker → save |
| `ReissueBlockedMsg{id, successor LicenseID}` | tentative `e` sur licence `superseded` |

**Cmd** : `services.License.List(filter)`, `services.License.Get(id) → LicenseDetail{Row, DecodedBindings, DecodedPayload, TOTPs, Revocation?, Parent?, Successors[]}`, `services.License.ExportPEM(id, path)`, `services.Audit.ListForTarget(id)`, `clipboard.Write(s)`.

**Touches**

| Touche | Action |
|---|---|
| `↑↓` | nav table |
| `d` ou `↵` | toggle split-pane détail |
| `I B P A C` | (détail ouvert) switch onglet interne |
| `/` | focus search |
| `f` | cycle status filter (all→active→expiring→expired→revoked→superseded) |
| `n` | wizard licence (vide) |
| `e` | wizard licence pré-rempli avec valeurs originales **sauf NotAfter** qui est re-demandé ; refusé si `superseded` → `reissue_blocked` overlay |
| `x` | overlay révoquer |
| `c` | copier PEM sélectionnée |
| `o` | sauver PEM → filepicker save |
| `q` | (onglet B avec TOTP présent) overlay QR |
| `Esc` | sort de recherche / ferme split-pane |

---

### 3.3 Wizard `nouvelle licence` — 8 étapes

**Rôle** : workflow guidé pour émettre une licence. Sidebar à gauche, étape à droite, barre de progression `n/8` en haut.

```
┌─ progress ─────────────────────────────────────────────────────────────────────────────────┐
│ ◆ NOUVELLE LICENCE   étape 3/8 · Bindings        Tab suivant · ⇧Tab précédent · 1-8 jump · esc annuler │
│ ████████████████████████████████░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░ │
├─ sidebar ──────────────┬─ step content ────────────────────────────────────────────────────┤
│ ✓ 1  Identité          │ Bindings déclarés (1)                                             │
│ ✓ 2  Validité          │  [m] +machine  [p] +password  [t] +TOTP  [k] +k/v                 │
│ ▌3  Bindings          │  ┌─ ◆ MACHINE  #1 ───────────────── [e] éditer · [x] retirer ─┐  │
│   4  Features          │  │ IDs acceptés (OR) :                                        │  │
│   5  Pinning           │  │   0c8a91…f4d2 (local)                                      │  │
│   6  Payload           │  │   7c91aa…1208 (composite)                                  │  │
│   7  Sealed payload    │  │ [l] ma machine actuelle  [r] récupérer machine distante…   │  │
│   8  Récap & émettre   │  └────────────────────────────────────────────────────────────┘  │
│                        │  ┌─ + ajouter un binding (dashed empty card) ─────────────────┐  │
│                        │  └────────────────────────────────────────────────────────────┘  │
└────────────────────────┴───────────────────────────────────────────────────────────────────┘
```

Les 8 étapes en bref :
| # | Step | Contenu |
|---|---|---|
| 1 | Identité       | subject (textinput + autocomplete history), issuer (default Settings), audience (multi), KeyID (default = active, dropdown) |
| 2 | Validité       | NotBefore (default now), NotAfter (textinput intelligent : "30d", "6mo", "1y", ISO date) + chips 30d/90d/6mo/1y/5y |
| 3 | Bindings       | table éditable de bindings empilables ; sub-types machine / password / TOTP / k/v ; **bouton `r` → ouvre `overlay_probe` en drawer** sans quitter le wizard |
| 4 | Features       | textinput multi + chips autocomplete (depuis DB) |
| 5 | Pinning        | left = IdentitySHA256 (dropdown des identities + `+` créer) ; right = BinarySHA256 (filepicker → hash auto avec progress.Model + paste manuel) |
| 6 | Payload        | chips `[1] vide`, `[2] JSON inline (textarea)`, `[3] importer fichier…` |
| 7 | Sealed payload | recipient key (dropdown) + secret en clair masqué (textinput password) ; sautable |
| 8 | Récap          | KV décodé complet + bouton **Émettre** (primary, `↵`) + boutons résultat (Copier / Sauver / QR si TOTP) |

**Bubbles**
- `bubbles/textinput` (un par champ).
- `bubbles/list` ou `huh.Form` pour le multi-select audience/features.
- `bubbles/progress` pour le hash binary SHA256 (étape 5).
- `bubbles/filepicker` (étape 5 binary, étape 6 import payload).
- `bubbles/viewport` (étape 8, PEM preview scrollable).
- `bubbles/textarea` (étape 6 si "JSON inline").

**Model**
```go
type wizardModel struct {
    step     int     // 0..7
    done     [8]bool // affichage check vert sidebar
    draft    LicenseDraft
    inputs   wizardInputs    // un *bubbles/textinput par champ pour rester contrôlé
    binding  bindingEditor   // sous-éditeur pour l'étape 3 (cf below)
    pinner   pinningEditor   // filepicker + progress
    payload  payloadEditor
    drawer   *probeDrawer    // nil si overlay probe pas ouvert ; cf §5
    err      error
}

type LicenseDraft struct {
    Subject, Issuer string
    Audience []string
    KeyID    string
    NotBefore, NotAfter time.Time
    Bindings []Binding
    Features []string
    IdentitySHA, BinarySHA [32]byte
    Payload  []byte           // raw JSON
    Sealed   *SealedSpec      // nil si none
}
type Binding interface { kind() string } // machine, password, totp, kv
type SealedSpec struct { RecipientKeyID string; SecretPlain []byte }
```

**Msg**
| Msg | Effet |
|---|---|
| `WizardNextMsg` / `WizardPrevMsg` / `WizardGotoMsg{n int}` | nav |
| `WizardCancelMsg` | esc → confirmation si dirty, sinon close |
| `MachineLocalIDsMsg{local, composite []byte}` | bouton `l` → `hostid.Local()` + `hostid.Composite()` (in-process) |
| `ProbeOpenMsg` | bouton `r` → ouvre drawer |
| `ProbeResultMsg{local, composite []byte, hostname, os, arch, cpu string}` | drawer → wizard |
| `HashStartedMsg{path string}` / `HashProgressMsg{done, total int64}` / `HashDoneMsg{sha [32]byte}` | étape 5 binary |
| `GenerateTOTPSecretMsg` / `TOTPSecretMsg{secret []byte}` | étape 3 binding TOTP |
| `WizardSubmitMsg` | étape 8 → `services.License.Issue(draft)` |
| `LicenseIssuedMsg{id LicenseID, pem []byte}` | succès → modal OK + retour Licences |
| `WizardErrorMsg{err error}` | échec → modal erreur, garde le draft |

**Cmd** (tous via `*service.Services`) : `hostid.Local()` / `hostid.Composite()` (in-process), `services.License.Issue(IssueRequest)` → `IssuedLicense{Row, PEM, TOTPs[]}`, `services.License.HashFile(path, progress func(done, total int64))` (renvoie un stream `HashProgressMsg` via channel + un final `HashDoneMsg`), `services.TOTP.Get(licenseID)` (post-issue) → secret + QRImageASCII + QRImagePNG, `services.Identity.List()`, `services.Recipient.List()`, `services.Issuer.Active()` / `services.Issuer.List()`, `services.License.List({SubjectLike: prefix})` pour l'autocomplete subjects.

**Touches**
| Touche | Action |
|---|---|
| `Tab` / `⇧Tab` | étape suivante/précédente |
| `1`-`8` | jump étape |
| `↵` | valider l'étape (next) |
| `Esc` | confirmer annulation (modal) |
| `Ctrl+C` | annulation immédiate sans confirmation |
| `m` `p` `t` `k` | (étape 3) ajouter binding du type |
| `l` `r` | (étape 3 binding machine) local / probe distant |
| `1`-`3` | (étape 6) variant payload |

---

### 3.4 Issuer keys (Ed25519)

**Rôle** : liste des clés de signature, marquer active, exporter, retirer.

```
┌────────────────────────────────────────────────────────────────────────────────────────────┐
│ Clés d'émission Ed25519 (4)   [n] générer · [i] importer · [a] désigner active · [E] .pub …│
│ KEYID      NOM                  STATUS   CRÉÉE        #SIGNÉES   FINGERPRINT               │
│▌k2026-04   rshell-prod-2026Q2   ACTIVE   2026-04-01        47    ed25519:a4f2…91bc        ▐│
│ ┌── détail (Métadonnées · Actions · Licences signées par cette clé) ────────────────────┐  │
│ │ keyid k2026-04 · name rshell-prod-2026Q2 · status ACTIVE · created … · signed 47       │  │
│ │ [a] déjà active  [E] export .pub  [K] export .key (confirm+passphrase)  [x] retirer    │  │
│ │ Licences signées (8 affichées) ── alice@research · bob@research · …                    │  │
│ └────────────────────────────────────────────────────────────────────────────────────────┘  │
│  k2026-01   rshell-prod-2026Q1  RETIRED  2026-01-04       138    ed25519:6c81…ab02         │
│  …                                                                                          │
└────────────────────────────────────────────────────────────────────────────────────────────┘
```

**Bubbles** : `table` + `viewport` (pour la liste des licences signées dans l'expand).

**Model** : `keysModel { rows []IssuerKeyDTO; cursor int; expanded bool; err error }`

**Msg** : `ListIssuerKeysMsg`, `GenerateIssuerKeyMsg{name string}`, `IssuerKeyGeneratedMsg{key KeyDTO}`, `ImportIssuerKeyMsg{path string, passphrase []byte}`, `SetActiveKeyMsg{keyid string}`, `ExportPubMsg{keyid, path string}`, `ExportPrivMsg{keyid, path string, passphrase []byte}`, `RetireKeyMsg{keyid string}`.

**Cmd** : `services.Issuer.List()`, `services.Issuer.Generate(name)` (renvoie aussi le KeyID auto-suggéré "k2026-MM"), `services.Issuer.ImportPEM(path, passphrase)`, `services.Issuer.SetActive(keyid)`, `services.Issuer.ExportPub(keyid, path)`, `services.Issuer.ExportPriv(keyid, path, passphrase)` (filepicker save).

**Touches** : `↑↓` `d` `n` `i` `a` `E` `K` `x` (cf header).

---

### 3.5 Recipient keys (X25519)

Même structure que §3.4. Moins central. Sert uniquement à sealer un payload.

```
┌────────────────────────────────────────────────────────────────────────────────────────────┐
│ Les recipient keys servent à sceller un payload (NaCl box).                                │
│ Recipient keys X25519 (2)  [n] générer · [i] importer · [E] export .pub · [x] retirer      │
│ KEYID      NOM                    CRÉÉE        #SEALED   FINGERPRINT                       │
│▌r2026-01   default-recipient      2026-04-01        12    x25519:7a90…003c                ▐│
│  r-emerg   emergency-recovery     2025-11-12         1    x25519:0044…bbf2                 │
└────────────────────────────────────────────────────────────────────────────────────────────┘
```

**Model** : `recipientsModel { rows []RecipientKeyDTO; cursor int; ... }`
**Msg / Cmd** : analogues à §3.4 (`ListRecipientKeysMsg`, `GenerateRecipientKeyMsg`, `ExportRecipientPubMsg`, `ExportRecipientPrivMsg`, `RetireRecipientKeyMsg`).

---

### 3.6 Identities binaires (identity.bin)

**Rôle** : 32 bytes random embarqués via `//go:embed` ; chaque identity a un sha256 référençable par une licence.

```
┌────────────────────────────────────────────────────────────────────────────────────────────┐
│ Identities (4)  [n] créer · [E] export .bin · [R] régénérer ⚠ · [x] supprimer              │
│ NOM                          SHA256              #REFS   CRÉÉE                             │
│▌rshell-windows-amd64.bin     8b3c91ad…2e1            22  2026-04-12                       ▐│
│ ┌── détail ────────────────────────────────────────────────────────────────────────────┐  │
│ │ Actions :  [E] export .bin  [R] régénérer (casse 22 licences) ⚠  [x] supprimer        │  │
│ │ ⚠ Régénérer change le sha256. Toute licence pinnée sur l'ancien sha cessera de valid. │  │
│ └──────────────────────────────────────────────────────────────────────────────────────┘  │
│ …                                                                                          │
└────────────────────────────────────────────────────────────────────────────────────────────┘
```

**Model** : `identitiesModel { rows []IdentityDTO; cursor int; ... }`
**Msg** : `ListIdentitiesMsg`, `CreateIdentityMsg{name string}`, `ExportIdentityMsg{name, path string}`, `RegenIdentityMsg{name string}` (overlay confirm danger forcée si refs > 0), `DeleteIdentityMsg{name string}` (impossible si refs > 0).

---

### 3.7 Revocation (CRL)

**Rôle** : afficher la CRL, ajouter/retirer une entry, exporter signée pour distribution offline.

```
┌────────────────────────────────────────────────────────────────────────────────────────────┐
│ Entries CRL: 4 (danger)   Pushed via :8443: oui (good)   Dernier export: 13:22             │
├────────────────────────────────────────────────────────────────────────────────────────────┤
│ Liste révocations (4)   [n] ajouter · [x] retirer · [E] exporter PEM signé                 │
│ LICENCE                            KEYID      AT               REASON                      │
│▌lic:71bd-… (bob@research)          k2026-04   2026-05-20 13:22 key_compromised            ▐│
│  lic:0033-… (ex-intern)            k2026-01   2026-05-04 09:11 offboarding                 │
│  …                                                                                          │
└────────────────────────────────────────────────────────────────────────────────────────────┘
```

**Model** : `crlModel { rows []CRLEntry; cursor int; pushedLive bool; lastExportAt time.Time; ... }`
**Msg** : `ListCRLMsg`, `AddRevocationMsg{licID, reason string}` (entrée via overlay revoke depuis Licenses), `RemoveRevocationMsg{licID string}`, `ExportSignedCRLMsg{path string}`.
**Cmd** : `services.Revoke.ListActive()`, `services.Revoke.Add(licID, reason)`, `services.Revoke.Remove(licID)`, `services.Revoke.ExportSigned(path)`.

---

### 3.8 Servers — 3 sous-onglets (Revocation / Heartbeat / Probe)

**Rôle** : status + config + live log des 3 services HTTP exposés par `*httpsrv.Bundle`. Sous-onglets cyclés par `r` / `h` / `p` (lowercase). La vue Probe a en plus une 3-vue interne (Tokens / History / Live log) commutable par `t` / `H` / `l` (cf §11.7).

```
┌── [r] Revocation ● ─── [h] Heartbeat ● ─── [p] Fingerprint probe ○ ─────────────────────────┐
│                                                                          fan-in via MergedEvents()│
├──────────────────────────────────────────┬──────────────────────────────────────────────────┤
│ STATUS                          [s] stop │ Live log — viewport autoscroll · [c] clear · [a] │
│  ● ON   port :8443                       │ 13:42:18  POST /revoke    200  18ms  1.2.3.4     │
│   url   https://manager.local:8443       │ 13:42:14  GET  /crl       200   3ms  1.2.3.4     │
│   uptime 2h 41m                          │ 13:42:08  GET  /crl       200   3ms  10.0.4.21   │
│   req tot 1 289      req/s 0.21          │ 13:41:42  POST /revoke    401  12ms  78.92.1.8   │
│                                          │ …                                                 │
│ CONFIGURATION                  [e] edit  ├──────────────────────────────────────────────────┤
│  port           8443                     │ Endpoints                                        │
│  TLS cert       /etc/…/tls.crt    📁     │ GET  /crl              renvoie CRL signée        │
│  TLS key        /etc/…/tls.key    📁     │ GET  /crl?keyid=…      filtré par KeyID          │
│  admin token    tk_•••••••••  [g] regen  │ POST /revoke           admin token requis        │
│                                          │ GET  /healthz · /metrics                         │
└──────────────────────────────────────────┴──────────────────────────────────────────────────┘
 r/h/p sous-onglets · s start/stop · e config · g régénérer token (stop+regen+restart) · c clear log
```

**Sous-onglet Probe — vue interne** (l'écran principal change le panneau de droite par 3 vues que cycle `t/H/l`) :

```
┌── [r] Revocation ─── [h] Heartbeat ─── [p] Fingerprint probe ● ─────────────────────────────┐
├──────────────────────────────────────────┬──────────────────────────────────────────────────┤
│ STATUS · CONFIG (idem)                   │ [t] Tokens actifs (3)  [H] History (5)  [l] Log │
│  ● ON :8445  url https://…/probe         ├──────────────────────────────────────────────────┤
│  token TTL par défaut    60s             │ TOKEN              LABEL         ISSUED   TTL   STATE│
│  max tokens actifs       8               │ tk_aB3xZ9mLqP21vR  alice-laptop  13:42    00:47s waiting│
│                                          │ tk_pQ7nT3wXyZ80kE  carol-vm-prod 13:39    02:22s waiting│
│                                          │ tk_xL9mN5sB2vC44d  lab-rig-02    13:11    30:20s waiting│
│ Astuce : sert surtout depuis le wizard   │                                                  │
│ licence (bindings machine → [r]). Mais   ├──── History view : ──────────────────────────────┤
│ tu peux le démarrer ici pour générer un  │ RECEIVED            LABEL   HOSTNAME  OS   LOCAL  USED IN│
│ batch de tokens (cas d'usage §10).       │ 2026-05-20 13:09    alice…  laptop-…  lin… 0c8a91…lic-…│
│                                          │ 2026-05-19 17:28    bob-mac MBP-bob   dar… 5e21fa…lic-…│
│                                          │ 2026-05-19 11:02    rig-03  rig-03    lin… aa18cc… —   │
└──────────────────────────────────────────┴──────────────────────────────────────────────────┘
 r/h/p sous-onglets · t/H/l vues internes · n nouveau token · q QR token · ↵ créer licence depuis fingerprint
```

**Bubbles** : `viewport` pour les live logs (autoscroll via `GotoBottom`). `textinput` pour port/TLS/token TTL. `filepicker` pour TLS cert/key. `table` pour Tokens actifs et History. `spinner` pour la phase "starting…".

**Model**
```go
type ServerID string
const (
    ServerRevoc     ServerID = "revoc"
    ServerHeartbeat ServerID = "heartbeat"
    ServerProbe     ServerID = "probe"
)

type ProbeView string
const (
    ProbeViewTokens  ProbeView = "tokens"
    ProbeViewHistory ProbeView = "history"
    ProbeViewLog     ProbeView = "log"
)

type serversModel struct {
    sub        ServerID
    probeView  ProbeView                          // utilisé uniquement quand sub == probe
    cfg        map[ServerID]ServerConfig
    status     map[ServerID]ServerStatus          // copie locale, maj sur ServerStatusMsg
    logs       map[ServerID]*viewport.Model       // ringbuf de N lignes (50 par défaut)
    tokens     []ProbeToken                       // services.Probe.ListActive()
    history    []ProbeResult                      // services.Probe.History(limit=100)
    inputs     serversInputs
    err        error
}
```

**Msg**

| Msg | Effet |
|---|---|
| `SwitchServerSubMsg{id ServerID}` | `r` / `h` / `p` |
| `SwitchProbeViewMsg{v ProbeView}` | `t` / `H` / `l` (uniquement si sub=probe) |
| `StartServerMsg{id ServerID, cfg ServerConfig}` | `s` |
| `StopServerMsg{id ServerID}` | `s` |
| `ServerStatusMsg{id ServerID, status ServerStatus}` | depuis `Bundle.Events()` |
| `ServerLogLineMsg{id ServerID, line LogLine}` | depuis `Bundle.MergedEvents()` (filtre côté TUI par `line.Server`) |
| `RegenAdminTokenMsg{id ServerID}` | `g` → push overlay `probe_regen` (confirme stop+regen+restart) |
| `RegenConfirmedMsg{id ServerID}` | overlay confirmé → `tea.Sequence(StopServer, RegenToken, StartServer)` |
| `AdminTokenMsg{id ServerID, token string}` | renvoyé par RegenToken, affiché **une seule fois** dans l'écran |
| `NewProbeTokenMsg{label string, ttl time.Duration}` | `n` sur vue Tokens |
| `ProbeTokenCreatedMsg{tok *ProbeToken}` | ajoute à `tokens`, refresh affichage |
| `ProbeResultReceivedMsg{r ProbeResult}` | un fingerprint vient d'arriver côté serveur — recharge tokens/history |
| `ProbeUseMsg{r ProbeResult}` | `↵` sur ligne history → `OpenWizardMsg` pré-rempli avec binding machine |
| `ClearLogMsg{id ServerID}` | `c` |
| `ServerErrorMsg{id ServerID, err error}` | port occupé, TLS bad → overlay `error` |

**Cmd** (tous via le bundle httpsrv ou `services.Probe`) :
- `bundle.Revocation.Start(cfg)` / `.Stop()` / `.Status()`
- `bundle.Heartbeat.Start(cfg)` / `.Stop()` / `.Status()`
- `bundle.Probe.Start(cfg)` / `.Stop()` / `.Status()`
- subscribe long-running : `for ev := range bundle.MergedEvents() { emit ServerLogLineMsg{ev.Server, ev.Line} }` — un `tea.Cmd` qui re-arm comme dans §6
- `services.Probe.NewToken(label, ttl)` → renvoie token + `<-chan *ProbeToken` (consommé par un autre tea.Cmd, émet `ProbeResultReceivedMsg`)
- `services.Probe.History(limit)` → liste pour view History
- `services.Probe.Revoke(token)` → `x` sur token actif

**Touches**

| Touche | Action |
|---|---|
| `r` / `h` / `p` | switch sub-tab Revoc / Heartbeat / Probe |
| `s` | start / stop serveur courant |
| `e` | éditer config (focus 1er textinput) |
| `g` | régénérer admin token → overlay `probe_regen` (confirm stop+regen+restart) |
| `c` | clear log |
| `a` | toggle autoscroll log |
| **Probe uniquement** | |
| `t` | vue Tokens actifs |
| `H` (Shift+h) | vue History |
| `l` | vue Live log |
| `n` | (vue Tokens) générer un nouveau token (push form label + TTL) |
| `q` | (vue Tokens) afficher QR du token sélectionné |
| `x` | (vue Tokens) révoquer le token sélectionné |
| `↵` | (vue History) créer une licence à partir du fingerprint → `OpenWizardMsg` pré-rempli binding machine |

#### Sous-onglet Heartbeat — spécifique
Vue "Licences live" : table des license_ids actifs avec toggle individuel `Space` qui ajoute / retire la licence de la blacklist heartbeat (considérée révoquée côté reply).

#### Sous-onglet Probe — flow de régénération admin token
Demande explicite, destructive en 3 étapes : (1) stop server, (2) regen token, (3) restart sur le même port. Un overlay `probe_regen` confirme avant exécution. Pendant ~2-3s le serveur est indisponible.

---

### 3.9 Audit

```
┌────────────────────────────────────────────────────────────────────────────────────────────┐
│ filtres : [f] all kinds  license.issue  license.revoke  key.activate  key.generate  …      │
│                                                            [E] export CSV   [J] export JSON │
├────────────────────────────────────────────────────────────────────────────────────────────┤
│ Audit (24)                          [d] détail · [r] refresh · [pgup/pgdn] page            │
│ TIMESTAMP             KIND              ACTOR     TARGET                  NOTE             │
│ 2026-05-20 13:42:18   license.issue     operator  lic:9f3a-…              k2026-04         │
│▌2026-05-20 13:35:35   license.revoke    operator  lic:9fbd-…              —               ▐│
│ ┌── détail ────────────────────────────────────────────────────────────────────────────┐  │
│ │ Entry id 1001 · t … · kind … · actor … · target … · note …                            │  │
│ │ Payload JSON :  { "kind": "license.revoke", "actor": "operator", "ip": "127.0.0.1", …}│  │
│ └──────────────────────────────────────────────────────────────────────────────────────┘  │
│  …                                                                                         │
└────────────────────────────────────────────────────────────────────────────────────────────┘
```

**Bubbles** : `table` + `viewport` (pour JSON pretty-printed). Pagination native ou `pgup/pgdn` géré à la main.

**Model** : `auditModel { rows []AuditEntry; cursor int; filter AuditFilter; page int; pageSize int; expanded bool }`
**Msg** : `ListAuditMsg{rows []AuditEntry, page int}`, `AuditFilterMsg{f AuditFilter}`, `ExportCSVMsg{path string}`, `ExportJSONMsg{path string}`.

---

### 3.10 Settings

```
┌─────────────────────────────────────────────┬──────────────────────────────────────────────┐
│ Defaults licence (wizard)                   │ default_argon_preset                         │
│  default_issuer_name  research@offsec.local │  [1] fast    [2] default ●  [3] paranoid     │
│  default_audience     rshell, rshell-edu    │  Coût à la vérification côté binaire         │
│  default_ttl_seconds  7776000 (90 j)        │                                              │
│  default_keyid        active                │                                              │
├─────────────────────────────────────────────┼──────────────────────────────────────────────┤
│ Identité opérateur                          │ Base de données                              │
│  operator_name    operator                  │  chemin  ~/.config/license-manager/db.sqlite │
│  → audit "actor"                            │  passphrase  résolue via MALDEV_MGR_PASSPHRASE_FILE│
│                                             │  [P] changer passphrase (rekey transactionnel)│
│                                             │  [V] vacuum + analyse · [B] backup chiffré…  │
├─────────────────────────────────────────────┼──────────────────────────────────────────────┤
│ Cycle de vie HTTP                           │ Cascade passphrase au boot (read-only)       │
│  À la fermeture                             │  1. --passphrase-file <path>                 │
│   [✓] confirm_quit_with_servers             │  2. env MALDEV_MGR_PASSPHRASE_FILE           │
│   [✓] arrêter tous les serveurs avant sortie│  3. env MALDEV_MGR_PASSPHRASE                │
│  Au démarrage                               │  4. fallback prompt TUI (cf §3.12)           │
│   [ ] auto_start_servers                    │  Cette session : MALDEV_MGR_PASSPHRASE_FILE ✓│
│   [✓] ouvrir directement Dashboard          │                                              │
└─────────────────────────────────────────────┴──────────────────────────────────────────────┘
```

**Bubbles** : `textinput` par champ. `huh.Form` recommandé ici (form plate, sections groupées). Toggle switches custom (cf §11).

**Model**
```go
type settingsModel struct {
    s      Settings   // copie locale, dirty si modifié
    inputs settingsInputs
    dirty  bool
    err    error
}

type Settings struct {
    // Defaults licence
    DefaultIssuerName  string
    DefaultAudience    []string
    DefaultTTLSeconds  int64                // services.Settings expose en secondes
    DefaultArgonPreset string                // "fast"|"default"|"paranoid"
    DefaultKeyID       string                // "active" ou keyid explicit

    // Identité opérateur (audit actor)
    OperatorName string

    // Cycle de vie serveurs
    ConfirmQuitWithServers bool
    StopServersOnQuit      bool
    AutoStartServers       bool

    // DB (lecture seule après init)
    DBPath                string
    PassphraseResolvedVia string                  // "CLI" | "ENV_FILE" | "ENV" | "PROMPT"

    // Apparence
    Theme           string                          // "neon"|"mono"|"nord-soft"
    DensityConfort  bool
    LocalTimestamps bool

    // Telemetry
    AuditAll           bool
    LogServerLifecycle bool
}
```

**Msg** : `LoadSettingsMsg`, `SettingsLoadedMsg{s Settings}`, `SaveSettingsMsg{s Settings}` (debounced 500ms), `OpenRekeyMsg` (push overlay `rekey`), `RekeyDBMsg{old, new []byte}`, `RekeyDoneMsg`, `RekeyFailedMsg{err error}`, `VacuumDBMsg`, `BackupDBMsg{path string}`.

**Cmd** : `services.Settings.Get()`, `services.Settings.Update(s)`, `services.Settings.ChangePassphrase(old, new)` (rekey en transaction unique).

---

### 3.11 Onboarding (first-run, DB inexistante)

**Rôle** : 4 étapes pour amener à un état où Dashboard est viable.

```
◆ PREMIÈRE UTILISATION    étape 2/4 · Passphrase DB     Tab continuer
█████████████████████████░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░

  2/4 — Passphrase de la base
  passphrase       ••••••••••••••   (≥ 12 caractères)
  confirmation     ••••••••••••••
  force : forte · entropie ≈ 92 bits · zxcvbn score 4/4

  [↵ Suivant]  [⇧Tab Précédent]
```

Étapes : (1) bienvenue & ce que va faire ce wizard, (2) passphrase, (3) issuer + 1ère paire Ed25519 (KeyID auto k2026-MM), (4) première licence pour soi-même avec binding machine local auto-détecté.

**Model** : `onboardingModel { step int; passphrase, passphraseConfirm []byte; issuer, keyName, keyid string; firstLicenseDraft LicenseDraft; ... }`

**Msg** : `OnboardingNextMsg`, `OnboardingPrevMsg`, `OnboardingPassphraseMsg`, `OnboardingIssuerMsg`, `OnboardingFirstLicenseMsg`, `OnboardingDoneMsg` → bascule `state.session = SessionReady`.

---

### 3.12 Passphrase prompt (DB existante, cascade non-résolue)

**Affichage conditionnel** : cet écran n'est rendu que si la cascade passphrase (cf §1) **n'a pas résolu** silencieusement. Dans le cas `MALDEV_MGR_PASSPHRASE_FILE` ou `MALDEV_MGR_PASSPHRASE`, on saute directement au Dashboard.

```
                                 ◆ license-manager
                            local-first · offline-capable

           Base existante à ~/.config/license-manager/db.sqlite

                       ┌──────────────────────────────┐
                       │ passphrase  ••••••••••••••   │
                       │ [↵ Déverrouiller]  [esc Quit]│
                       └──────────────────────────────┘
        3 essais restants · backoff exponentiel après échec · cascade non-résolue
```

**Boucle 3 tentatives** : chaque échec affiche le nombre d'essais restants et applique un backoff exponentiel (1s → 2s → 4s pendant lesquels l'input est désactivé). À la 4ème tentative refusée, l'app exit avec code 1.

**Model** :
```go
type passphraseModel struct {
    input       textinput.Model
    attempts    int           // décompte 3 → 2 → 1 → 0
    lockUntil   time.Time     // pendant le backoff
    err         error
}
```

**Msg** : `TryUnlockMsg{passphrase []byte}`, `UnlockOKMsg` (→ `SessionReady`), `UnlockFailedMsg{remaining int, backoff time.Duration}`, `UnlockExhaustedMsg` (→ `tea.Quit` avec exit code 1).

**Cmd** : un unique `services.Settings.VerifyPassphrase(p) error` côté backend (n'est PAS une API publique du bundle ; appelé par `main.go` au boot avant d'instancier la Services). Le prompt est dans un `tea.Program` léger spécifique, séparé du root program (deux niveaux : prompt → succès → bootstrap root).

---

## 4. Model global

**Décision** : *single central model* avec `activeView` enum + sub-models par écran (state co-localisé), **plus** une `overlayStack` pour les transients (wizards, confirmations, drawer probe). C'est l'idiome bubbletea le plus solide pour une app à onglets stables.

```go
type SessionState int
const (
    SessionLocked      SessionState = iota // DB existante, passphrase pas encore validée
    SessionOnboarding                      // DB inexistante, wizard first-run
    SessionReady                           // app principale
)

type ViewID string
const (
    ViewDashboard  ViewID = "dashboard"
    ViewLicenses   ViewID = "licenses"
    ViewIssuers    ViewID = "issuers"
    ViewRecipients ViewID = "recipients"
    ViewIdentities ViewID = "identities"
    ViewRevocation ViewID = "revocation"
    ViewServers    ViewID = "servers"
    ViewAudit      ViewID = "audit"
    ViewSettings   ViewID = "settings"
)

type rootModel struct {
    session    SessionState
    active     ViewID
    overlay    []Overlay         // stack ; len > 0 → render le dernier par-dessus la vue
    width, hgt int               // tea.WindowSizeMsg
    lastKey    string            // affichage status bar
    msg        *FlashMsg         // toast (cyan/green/red) avec TTL
    keys       KeyMap            // bubbles/key
    help       help.Model

    // sessions
    passphrase passphraseModel
    onboarding onboardingModel

    // screens (alloués une fois, persistent)
    dashboard  dashboardModel
    licenses   licensesModel
    issuers    keysModel
    recipients recipientsModel
    identities identitiesModel
    revocation crlModel
    servers    serversModel
    audit      auditModel
    settings   settingsModel

    // wizard licence : on alloue à l'ouverture, on garbage à la fermeture
    wizard     *wizardModel

    // services injectés à la construction (depuis main.go après la cascade passphrase)
    services *service.Services      // bundle métier : Issuer, License, Revoke, Identity, Recipient, TOTP, Probe, Audit, Settings
    httpsrv  *httpsrv.Bundle        // bundle des 3 serveurs HTTP : Revocation, Heartbeat, Probe
                                    // expose Start/Stop/Status par serveur + MergedEvents() <-chan Event (fan-in)
}

func (m rootModel) Init() tea.Cmd { ... }
func (m rootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { ... }
func (m rootModel) View() string { ... }
```

**Routing**. `rootModel.Update` :
1. WindowSizeMsg → propage à chaque sub-model.
2. Si `overlay` non vide → l'overlay du dessus consomme `msg`. S'il renvoie un `OverlayCloseMsg{result}` on pop ; sinon on absorbe ou on laisse couler selon le type d'overlay (les modaux capturent tout sauf `WindowSize` ; le drawer laisse passer les `tea.Tick`).
3. Si `session != Ready` → on délègue à `passphrase` ou `onboarding`.
4. Sinon → key router global (tabs, `?`, `q`, `/`) ; si non consommé → `m.active.Update(msg)`.

**Persistance**. Chaque sub-model est gardé en mémoire — basculer onglet ne perd ni le scroll, ni la sélection, ni le filtre. Le wizard est l'exception : à `WizardCancelMsg`/`LicenseIssuedMsg` on `m.wizard = nil` et on libère.

**Pourquoi pas une pile d'écrans ?** Le persona switche entre des domaines stables (9 onglets), pas entre des écrans push/pop. Un `activeView` enum est plus simple et plus performant qu'un router. La pile sert uniquement aux **overlays transients**.

---

## 5. Overlays

**Interface** :
```go
type Overlay interface {
    tea.Model            // Update + View
    Kind() OverlayKind   // pour router des Msg-overlays-spécifiques
    ConsumesGlobalKeys() bool   // true: les 1-9, q, ? sont absorbés ; false: pass-through
    // L'overlay émet un OverlayCloseMsg{ Kind, Result any } pour se fermer.
    // rootModel pop l'overlay et dispatch Result au sub-model approprié.
}
```

**Liste**

| Kind | Type visuel | Provient de | Émet en sortie |
|---|---|---|---|
| `confirm`           | modal centré        | n'importe où                                      | `ConfirmedMsg{ok bool, ctx any}` |
| `quit`              | modal danger        | `q` global                                       | `QuitConfirmedMsg{stopServers bool}` |
| `revoke`            | modal danger        | `x` sur licence                                  | `RevokeConfirmedMsg{licID, reason string}` |
| `error`             | modal danger        | `*ErrorMsg`                                      | `OverlayCloseMsg` |
| `ok`                | modal succès        | post-action                                       | `OverlayCloseMsg` |
| `qr`                | modal info          | détail licence / probe / TOTP step                | `OverlayCloseMsg` |
| `filepicker`        | modal large         | partout où un chemin est requis                   | `FilepickerResultMsg{path string, ctx any}` |
| `help`              | modal info          | `?`                                              | `OverlayCloseMsg` |
| `rekey`             | modal               | Settings `P`                                     | `RekeyConfirmedMsg{old, new []byte}` |
| `identity_blocked`  | modal avertissement | tentative de delete d'une identity avec refs > 0  | `OverlayCloseMsg` (info-only) |
| `reissue_blocked`   | modal avertissement | tentative `e` sur licence `superseded`          | `OverlayCloseMsg` ou `OpenLicenseMsg{successorID}` |
| `probe_drawer`      | **drawer droit 62%**| wizard étape 3 / Servers/Probe                    | `ProbeResultMsg{local, composite []byte, hostname, os, arch, cpu string}` ou `ProbeCancelMsg` |
| `probe_regen`       | modal avertissement | `g` sur écran Servers (n'importe quel sous-onglet)| `RegenConfirmedMsg{id ServerID}` → `tea.Sequence(StopServer, RegenToken, StartServer)` |
| `probe_keep`        | modal               | après `ProbeResultMsg` consommé                  | `KeepProbeMsg{keepRunning bool}` |

**Pourquoi un drawer pour probe ?** L'opérateur doit lire l'URL/token tout en restant dans le wizard et voir la machine cliente apparaître. Couvrir le wizard avec une modale rend le contexte invisible. Le drawer occupe 62% à droite, le wizard reste lisible derrière le dim léger à gauche.

**Renvoi de résultat**. L'overlay est dispatché par le root mais son résultat est destiné au sub-model qui l'a ouvert. Pattern :
```go
type OverlayContext struct {
    OriginView ViewID
    Payload    any        // ex: licID pour revoke, "wizard.step3.binding[0]" pour probe
}
```
Quand l'overlay émet son résultat, le root re-dispatch au sub-model d'origine en attachant le `ctx.Payload`.

**Modal stacking**. L'overlay est une pile : on peut empiler `qr` sur un wizard sur un overlay de licence détail. En pratique on évite > 2 niveaux. Esc pop le top.

---

## 6. Cycle de vie des serveurs HTTP

**Principe** : les goroutines HTTP ne vivent **pas** dans la TUI. Elles vivent dans un **`*httpsrv.Bundle` fourni par le backend** (PAS à implémenter — déjà disponible). `main.go` l'instancie après la cascade passphrase et l'injecte dans `rootModel.httpsrv`. La TUI consomme cette API exclusivement via des `tea.Cmd`.

### 6.1 Surface du `*httpsrv.Bundle` (rappel)

```go
type Bundle struct {
    Revocation *Server
    Heartbeat  *Server
    Probe      *Server
}

type Server interface {
    Start(cfg Config) error                // bind + listen, renvoie immédiatement après "Up"
    Stop() error                           // graceful shutdown
    Status() Status                        // {State, URL, StartedAt, RequestCount, ...}
    Events() <-chan Event                  // events propres à ce serveur
}

func (b *Bundle) MergedEvents() <-chan Event   // fan-in des 3 Events() en un seul channel
```

### 6.2 Côté TUI : subscribe long-running

La TUI s'abonne à `MergedEvents()` via un `tea.Cmd` "longue durée" qui boucle et émet des messages au root model :

```go
func subscribeHTTPEvents(b *httpsrv.Bundle) tea.Cmd {
    return func() tea.Msg {
        ev, ok := <-b.MergedEvents()
        if !ok { return nil } // bundle fermé
        switch e := ev.(type) {
        case httpsrv.StatusEvent:  return ServerStatusMsg{e.Server, e.Status}
        case httpsrv.LogEvent:     return ServerLogLineMsg{e.Server, e.Line}
        case httpsrv.ErrorEvent:   return ServerErrorMsg{e.Server, e.Err}
        }
        return nil
    }
}
```

On lance `subscribeHTTPEvents` dans `Init()`, puis on le **re-arme** après chaque `ServerStatusMsg` / `ServerLogLineMsg` / `ServerErrorMsg` (en retournant à nouveau le même Cmd depuis `Update`). Idiome bubbletea standard pour un canal long-lived.

### 6.3 Wrappers Cmd unitaires (`cmds/servers.go`)

```go
func StartServerCmd(b *httpsrv.Bundle, id ServerID, cfg httpsrv.Config) tea.Cmd {
    return func() tea.Msg {
        srv := pickServer(b, id)
        if err := srv.Start(cfg); err != nil {
            return ServerErrorMsg{id, err}
        }
        return ServerStartedMsg{id, srv.Status()}
    }
}
// StopServerCmd, StopAllCmd analogues.
```

### 6.4 Fermeture sûre

`q` ouvre l'overlay `quit`. Si des serveurs sont ON ET `Settings.confirm_quit_with_servers`, l'overlay liste les actifs et demande confirmation. À confirmation :

```go
return tea.Sequence(
    StopAllServersCmd(httpsrv),
    CloseServicesCmd(services),    // ferme la DB proprement
    tea.Quit,
)
```

`Ctrl+C` (SIGINT) → `tea.Quit` direct, mais `main.go` doit avoir un `defer` qui appelle `httpsrv.Bundle.StopAll()` + `services.Close()` pour ne laisser ni port ouvert ni DB lockée.

### 6.5 Probe one-shot : pattern

Le probe a deux particularités : (a) tokens à TTL, (b) résultats arrivent sur un channel dédié plutôt que via les events HTTP standard.

```go
// services.Probe.NewToken renvoie un token + un channel one-shot
tok, ch, err := services.Probe.NewToken(label, ttl)

// La TUI consomme :
func waitProbe(tok string, ch <-chan *probe.Result, ttl time.Duration) tea.Cmd {
    return func() tea.Msg {
        select {
        case r, ok := <-ch:
            if !ok { return ProbeCancelledMsg{tok} }
            return ProbeResultMsg{tok, r}
        case <-time.After(ttl):
            return ProbeTimeoutMsg{tok}
        }
    }
}
```

Le drawer `probe_drawer` lance ce Cmd à l'ouverture. Si l'opérateur ferme avant réception, on annule le token via `services.Probe.Revoke(tok)` et le channel est fermé.

### 6.6 Regen admin token : séquence atomique

`g` sur l'écran Servers ouvre l'overlay `probe_regen` (modal jaune). Sur confirmation :

```go
return tea.Sequence(
    StopServerCmd(httpsrv, id),
    RegenAdminTokenCmd(services, id),    // expose le nouveau token via AdminTokenMsg
    StartServerCmd(httpsrv, id, cfg),
)
```

Pendant l'opération (~2-3s), l'écran montre `STARTING` puis `ON` à nouveau. Si le `Start` final échoue (port pris entre-temps), overlay `error` + `Réessayer démarrage` (cf §8).

---

## 7. Catalogue de Msg/Cmd transversaux

### Clavier global
| Touche | Msg | Cmd |
|---|---|---|
| `1`-`9` | `SwitchViewMsg{ViewID}` | aucun |
| `?` | `OpenHelpMsg` | aucun (push overlay) |
| `q` | `QuitRequestMsg` | aucun (push `quit` overlay) |
| `Esc` | `EscMsg` | dépend du contexte |
| `Ctrl+C` | `tea.QuitMsg` | `StopAllServersCmd` + `tea.Quit` |
| `/` | `FocusSearchMsg` | aucun |
| `r` | `RefreshCurrentMsg` | re-déclenche la `ListXMsg` du sub-model |

### Système
| Msg | Source | Effet |
|---|---|---|
| `tea.WindowSizeMsg` | term resize | propage à tous les sub-models |
| `TickMsg` (5s) | `tea.Tick` | refresh dashboard counters + uptime serveurs |
| `ClockTickMsg` (1s) | `tea.Tick` | refresh affichage horloge titlebar |
| `FlashMsg{text, kind, ttl}` | actions post-success/error | toast en status bar, auto-clear |
| `BackendErrorMsg{err error}` | n'importe quelle Cmd | push overlay error si non gérable inline |

### Wizard partagé
`WizardNextMsg`, `WizardPrevMsg`, `WizardGotoMsg`, `WizardCancelMsg`, `WizardSubmitMsg` — réutilisables par d'autres wizards (onboarding, future passe).

---

## 8. Cas d'erreur

| Cas | Rendu | Recovery |
|---|---|---|
| Passphrase fausse | overlay `error` : "Passphrase incorrecte. N essais restants." | bouton `réessayer` → focus textinput, backoff 1s/2s/4s |
| DB locked (autre instance) | overlay `error` : "Base SQLite verrouillée. Une autre instance tourne ?" | bouton `réessayer` (retry tous les 2s, max 5) |
| DB corruption | overlay `error` rouge non-dismissable + log path/erreur + bouton `quit` | aucun — pas de recovery automatique |
| Port serveur occupé | overlay `error` : "Port :8443 occupé (errno 98)" | bouton `essayer :8543` (incrémente, retry) ; rester sur l'écran Servers |
| TLS cert/key invalide | overlay `error` avec détails du parser | bouton `ouvrir filepicker` |
| Token probe expiré | inline dans le drawer probe : "Token expiré ✗" + bouton `générer un nouveau` ; ne pas fermer le drawer | regénère + re-arm waiter |
| Filepicker no read perm | overlay `error` : "Permission refusée sur ce répertoire" | bouton `ok`, retour filepicker |
| Hashing fichier échoue | inline étape 5 wizard : barre rouge + message | bouton `réessayer` ou `coller hash manuel` |
| Émission licence échoue | overlay `error` après étape 8 ; le draft est préservé | bouton `revenir` (retour étape 8) |
| **Re-issue d'une licence superseded** | overlay `reissue_blocked` au moment du `e` ; titre violet "Re-émission refusée — déjà superseded par lic-xyz" | bouton `Ouvrir le successeur` qui sélectionne la dernière licence de la chaîne dans la liste |
| **Delete d'un Issuer avec licences signées > 0** | modal `error` rouge "Suppression refusée — N licences signées par cette clé" | bouton `Voir les licences` (Licences filtré sur keyid) ; ou `Retirer` à la place de `Delete` (status retired préserve les signatures) |
| **Delete d'une identity avec UsageCount > 0** | modal `identity_blocked` jaune "X licences l'utilisent" | bouton `Voir les licences` (Licences filtré sur identity sha256) ; `Fermer` |
| **HashFile sur fichier inaccessible** | inline étape 5 wizard : message rouge "permission denied" + le chemin tenté ; le filepicker reste ouvrable | bouton `Réessayer` ou `Coller hash manuellement` |
| **Passphrase fausse (cascade non-résolue)** | écran §3.12, message inline rouge "Passphrase incorrecte. N essais restants." + backoff visible | bouton `Réessayer` (auto-actif après expiration du backoff). 4ème échec → `UnlockExhaustedMsg` → exit code 1 |
| **Regen admin token interrompu** | si `Stop` réussit mais `Start` échoue (port occupé entre-temps), overlay `error` "Serveur arrêté + token regen OK + restart KO" + nouveau token affiché | bouton `Réessayer démarrage` (avec port modifié) ; sinon le serveur reste OFF |
| Génération clé Ed25519 échoue | overlay `error`, retour onglet Issuer keys | bouton `réessayer` |
| Réseau HTTP indisponible (offline) | tile dashboard "serveurs" affiche `OFFLINE` ; pas d'erreur bloquante (local-first) | aucun |
| Export CSV/JSON, disque plein | overlay `error` avec espace libre | bouton `changer chemin` |
| Crash backend (panic en Cmd) | `BackendErrorMsg{err}` → overlay error + audit `system.panic` | bouton `redémarrer` (re-init le sub-model concerné) |

**Format** : toujours **titre court + corps lisible + détails techniques en monospace + bouton recover quand pertinent**. Jamais de stacktrace brute imposée.

---

## 9. Conventions de style (lipgloss)

```go
// internal/manager/tui/theme.go
package tui

import "github.com/charmbracelet/lipgloss"

// Palette néon ----------------------------------------------------------
var Palette = struct {
    Bg, Bg1, Bg2, Bg3 lipgloss.Color
    Border, BorderBright lipgloss.Color
    Fg, FgDim, FgMute lipgloss.Color
    Cyan, Magenta, Green, Yellow, Orange, Red, Violet lipgloss.Color
}{
    Bg: "#05050d", Bg1: "#0a0a18", Bg2: "#10102a", Bg3: "#16163a",
    Border: "#2a2a52", BorderBright: "#4a4aa0",
    Fg: "#e6e6ff", FgDim: "#7a7ab8", FgMute: "#4a4a78",
    Cyan: "#00f0ff", Magenta: "#ff36d4", Green: "#39ff88",
    Yellow: "#ffce39", Orange: "#ff8a3c", Red: "#ff3c5f", Violet: "#a070ff",
}

// Styles primaires ------------------------------------------------------
var (
    Base       = lipgloss.NewStyle().Foreground(Palette.Fg).Background(Palette.Bg)
    Dim        = lipgloss.NewStyle().Foreground(Palette.FgDim)
    Mute       = lipgloss.NewStyle().Foreground(Palette.FgMute)
    GlowCyan   = lipgloss.NewStyle().Foreground(Palette.Cyan).Bold(true)
    GlowMagent = lipgloss.NewStyle().Foreground(Palette.Magenta).Bold(true)
    GlowGreen  = lipgloss.NewStyle().Foreground(Palette.Green).Bold(true)
    GlowRed    = lipgloss.NewStyle().Foreground(Palette.Red).Bold(true)

    BoxStyle = lipgloss.NewStyle().
        Border(lipgloss.NormalBorder()).
        BorderForeground(Palette.Border)

    BoxFocused = BoxStyle.Copy().BorderForeground(Palette.Magenta)

    BoxTitle = lipgloss.NewStyle().
        Foreground(Palette.Cyan).
        Bold(true).
        Padding(0, 1).
        Border(lipgloss.NormalBorder(), false, false, true, false).
        BorderForeground(Palette.Border)

    BoxTitleFocused = BoxTitle.Copy().
        Foreground(Palette.Magenta).
        BorderForeground(Palette.Magenta)

    // Tabs
    TabActive   = lipgloss.NewStyle().Foreground(Palette.Fg).Bold(true).
                   Padding(0, 2).Background(lipgloss.Color("#26063b")).
                   Border(lipgloss.NormalBorder(), false, false, true, false).
                   BorderForeground(Palette.Magenta)
    TabInactive = lipgloss.NewStyle().Foreground(Palette.FgDim).Padding(0, 2)
    TabNumActive   = lipgloss.NewStyle().Foreground(Palette.Magenta).Bold(true)
    TabNumInactive = lipgloss.NewStyle().Foreground(Palette.FgMute).Bold(true)

    // Status bar hint
    HintKey  = lipgloss.NewStyle().Foreground(Palette.Magenta).Bold(true).Padding(0, 1)
    HintText = lipgloss.NewStyle().Foreground(Palette.FgDim)

    // Pills
    pillBase = lipgloss.NewStyle().Padding(0, 1).Bold(true)
    PillActive   = pillBase.Copy().Foreground(Palette.Green).Border(lipgloss.NormalBorder()).BorderForeground(Palette.Green)
    PillExpiring = pillBase.Copy().Foreground(Palette.Yellow).Border(lipgloss.NormalBorder()).BorderForeground(Palette.Yellow)
    PillExpired  = pillBase.Copy().Foreground(Palette.FgMute).Border(lipgloss.NormalBorder()).BorderForeground(Palette.FgMute)
    PillRevoked  = pillBase.Copy().Foreground(Palette.Red).Border(lipgloss.NormalBorder()).BorderForeground(Palette.Red)
    PillOn       = PillActive
    PillOff      = PillExpired

    // Inputs
    InputLabel    = Dim
    InputLabelFocused = lipgloss.NewStyle().Foreground(Palette.Magenta).Bold(true)
    InputValue    = lipgloss.NewStyle().Foreground(Palette.Fg)
    InputUnderline = lipgloss.NewStyle().Border(lipgloss.NormalBorder(), false, false, true, false).BorderForeground(Palette.Border)
    InputUnderlineFocused = InputUnderline.Copy().BorderForeground(Palette.Magenta)
    Caret = lipgloss.NewStyle().Background(Palette.Magenta).Foreground(Palette.Bg).Blink(true)

    // Table
    TableHeader    = lipgloss.NewStyle().Foreground(Palette.Cyan).Bold(true).
                      Border(lipgloss.NormalBorder(), false, false, true, false).
                      BorderForeground(Palette.Border)
    TableRow       = lipgloss.NewStyle()
    TableRowSel    = lipgloss.NewStyle().
                      Background(lipgloss.Color("#1c0a25")).
                      Border(lipgloss.NormalBorder(), false, true, false, true).
                      BorderForeground(Palette.Magenta)

    // Modal
    Modal       = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(Palette.Magenta).Padding(1, 2)
    ModalDanger = Modal.Copy().BorderForeground(Palette.Red)
    ModalOK     = Modal.Copy().BorderForeground(Palette.Green)
    Scrim       = lipgloss.NewStyle().Background(lipgloss.Color("#000000")).Faint(true)

    // Progress bar (custom — bubbles/progress accepte gradients)
    ProgressFill = lipgloss.NewStyle().Foreground(Palette.Magenta)
    ProgressBg   = lipgloss.NewStyle().Foreground(Palette.Bg3)

    // Glow simulation
    // lipgloss n'a pas de text-shadow ; on simule par bold + couleur saturée.
    // Pour les terminaux qui le supportent, on peut activer le rendu RGB.
)

// Densité ---------------------------------------------------------------
// Dense-mais-aéré façon lazygit.
const (
    PadX = 1   // padding horizontal interne d'une box
    PadY = 0   // padding vertical interne d'une box (0 = dense ; 1 = confort, suivre Settings.DensityConfort)
    GutterX = 2
    GutterY = 1
)
```

**Conventions** :
- `BoxFocused` n'est utilisé qu'au plus **une fois par écran** : indique la zone qui reçoit le clavier.
- `GlowMagenta` pour les valeurs critiques (KeyID actif, IDs probe), `GlowCyan` pour les titres et fingerprints, `GlowGreen` pour les statuts ON / succès, `GlowRed` pour révoqué / erreurs.
- Jamais de couleur saturée sur du texte de paragraphe — toujours `Fg` ou `FgDim`.
- Densité par défaut : `lazygit`. Bascule `Settings.DensityConfort` ajoute +1 à `PadY` globalement.

---

## 10. Dépendances Go

À ajouter à `go.mod` :

```go
require (
    github.com/charmbracelet/bubbletea v0.27.0      // root runtime
    github.com/charmbracelet/lipgloss  v0.13.0      // styles
    github.com/charmbracelet/bubbles   v0.20.0      // table, textinput, viewport, filepicker, progress, help, spinner, key, cursor
    github.com/charmbracelet/huh       v0.5.0       // facultatif — Settings + onboarding step 3 si plus rapide
    github.com/atotto/clipboard        v0.1.4       // copier PEM / secret dans le clipboard OS
    github.com/skip2/go-qrcode         v0.0.0-…     // export PNG/SVG du QR (l'ASCII est rendu maison)
    github.com/mdp/qrterminal/v3       v3.2.0       // rendu QR ASCII inline (alternative : faire à la main)
)
```

### 10.1 Backend — APIs consommées par la TUI

Le backend fournit deux singletons injectés au boot par `main.go` :

```go
// 1) Bundle métier — toutes les opérations passent par là.
type Services struct {
    Issuer    IssuerService     // Ed25519 keys
    License   LicenseService    // CRUD + Issue + Reissue + List + Get + ExportPEM + HashFile
    Revoke    RevokeService     // CRL : Add, Remove, ListActive, ExportSigned
    Identity  IdentityService   // identity.bin : List, Create, ExportBin, Regenerate, Delete (refus si UsageCount>0)
    Recipient RecipientService  // X25519 sealed payload
    TOTP      TOTPService       // Get(licenseID) → secret + QR ASCII + QR PNG
    Probe     ProbeService      // NewToken(label, ttl), Subscribe(token), History(limit), Revoke(token)
    Audit     AuditService      // Append, List(filter), ListForTarget(id), ExportCSV/JSON
    Settings  SettingsService   // Get, Update, ChangePassphrase, VerifyPassphrase
}

// 2) Bundle HTTP — 3 serveurs gérés en propre.
type Bundle struct {
    Revocation *Server
    Heartbeat  *Server
    Probe      *Server
}
// Chaque *Server expose : Start(cfg), Stop(), Status(), Events() <-chan Event
// Bundle expose en plus : MergedEvents() <-chan Event  (fan-in)
```

**À retenir** :
- La TUI **ne touche jamais** aux stores, à la DB, ni au réseau directement.
- Tous les wrappers `cmds/*.go` sont des fonctions courtes qui prennent `*service.Services` (et/ou `*httpsrv.Bundle`) en paramètre et émettent un `tea.Msg` (cf §2).
- Aucun appel synchrone dans `Update` : tout passe par un `tea.Cmd`.
- Les channels long-lived (`MergedEvents`, `Probe.Subscribe`) sont consommés par un `tea.Cmd` qui se **re-arme** lui-même après chaque message (idiome bubbletea standard, cf §6.2).

> Le backend (DB SQLite chiffrée, hostid, NaCl box, argon2id, ed25519/x25519, *httpsrv*) est déjà fourni. La TUI ne dépend de rien d'autre côté réseau.

---

## 11. Compatibilité bubbletea/lipgloss — ce qui ne se transpose pas littéralement

Les mocks ASCII et le prototype HTML utilisent des effets CSS (glow, gradients, blur, animations, tailles de police variables) qui **n'existent pas dans un terminal**. Cette section liste explicitement les libertés prises par les mocks et leur équivalent terminal. **À lire avant de coder.**

### 11.1 Effets visuels purement décoratifs (à ignorer)

| Effet du mock / prototype | Existe en TUI ? | Substitut |
|---|---|---|
| `text-shadow: 0 0 8px …` (glow néon) | **non** | `lipgloss.NewStyle().Bold(true).Foreground(palette)` — l'éclat se traduit par couleur saturée + bold |
| `box-shadow` / glow autour des modals | **non** | bordure colorée vive (rouge pour danger, magenta pour primary) suffit |
| `backdrop-filter: blur(2px)` sur le scrim | **non** | `lipgloss.NewStyle().Faint(true)` appliqué au rendu du fond avant de composer le modal par-dessus |
| `linear-gradient(180deg, …)` (tabs actives, dashboard) | **non** | une seule couleur de fond solide + une bordure colorée d'1 ligne (cf §11.4) |
| `@keyframes blink/pulse/spin` (autre que spinner) | **non** | bubbletea n'a pas d'animation hors `bubbles/spinner` et `bubbles/cursor`. Tout effet "fade", "slide", "pulse" du HTML disparaît — apparition/disparition instantanée |
| Tailles de police variables (`font-size: 32px` sur les compteurs dashboard) | **non** | tout fait la même taille de cellule. Pour donner du poids, `Bold(true)` + couleur saturée. Optionnel : utiliser une ASCII-art figlet (cf §11.6) pour les compteurs centraux |
| `border-radius` arrondis | **non** | lipgloss propose `RoundedBorder()` (caractères `╭╮╰╯`) ou `NormalBorder()` (`┌┐└┘`). Pas de vrai radius |
| Caret rose qui clignote | **partiel** | `bubbles/cursor` gère le blink ; la couleur du curseur est paramétrable mais pas son glow |
| Scanlines / vignette / dégradés radiaux sur le fond | **non** | terminal = couleur unie. Le fond néon du proto disparaît |

### 11.2 Layouts qui demandent une adaptation

| Pattern du mock | Adaptation TUI |
|---|---|
| **Expand-row inline dans `bubbles/table`** | `bubbles/table` ne supporte pas de ligne déployable in-place. Implémenter en **split-pane vertical** : `lipgloss.JoinVertical(Top, tableView, detailPanel)` où `detailPanel` ne s'affiche que si `expanded` est vrai et occupe ~45% de la hauteur disponible sous la table. Visuellement c'est équivalent : la ligne sélectionnée est visible en haut, le détail rempli en bas. La touche `d` toggle `expanded`. |
| **Drawer slide-over droit (probe)** | `lipgloss.JoinHorizontal(Top, leftView, drawerView)` où `leftView` est la vue du wizard rendue avec `Faint(true)` et `drawerView` occupe 62% de la largeur, séparée par une bordure gauche colorée cyan. Pas d'animation de slide — apparition instantanée |
| **Modal centré avec scrim dimmé** | (1) rendre la vue de fond, (2) appliquer `Faint(true)` ligne par ligne au string résultat, (3) calculer la box modale (lipgloss.NewStyle.Border…Padding…Width…), (4) la centrer avec `lipgloss.Place(termW, termH, lipgloss.Center, lipgloss.Center, modalString, lipgloss.WithWhitespaceChars(" "))`. Pas de transparence réelle |
| **Stack d'overlays** | une `[]Overlay` dans le root model ; `View()` itère et compose chaque overlay par-dessus le précédent. Esc pop le top |
| **Chips à wrap multi-ligne** | lipgloss ne wrap pas en flex. Mesurer la largeur de chaque chip rendu, accumuler horizontalement avec `lipgloss.JoinHorizontal`, et déclencher un saut de ligne soi-même quand le cumul dépasse `availableWidth - margin`. Helper utilitaire `chipFlow(chips []string, w int) string` à écrire (50 lignes) |
| **Grid CSS 4 colonnes responsive (tiles dashboard)** | `tea.WindowSizeMsg` → `tileW := (w - gutters) / 4` → chaque tile rendue à largeur fixe → `lipgloss.JoinHorizontal` |
| **`overflow: auto` sur un panneau** | `bubbles/viewport` (PEM preview, live log, audit JSON, détail row) |
| **Sticky header de table** | natif dans `bubbles/table` ✓ |

### 11.3 Composants `bubbles` réellement utilisés (récap)

| Composant | Où | Notes |
|---|---|---|
| `bubbles/table` | Licenses, Issuers, Recipients, Identities, Revocation, Audit | tri natif, sélection, header. **Pas de ligne expand** (cf §11.2) |
| `bubbles/textinput` | tous les forms, recherche `/` | gère curseur via `bubbles/cursor` |
| `bubbles/textarea` | wizard étape 6 (JSON inline) | multi-ligne |
| `bubbles/viewport` | live log, PEM preview, audit JSON detail | autoscroll via `GotoBottom()` |
| `bubbles/filepicker` | wizard étape 5 (binary), Servers (TLS cert/key), exports | fournit sa propre vue ; vous l'enveloppez dans un `overlay_filepicker.go` qui rend en modal |
| `bubbles/progress` | hashing binary SHA, progress wizard, optionnel sur counters | supporte les gradients via `progress.WithGradient(cyan, magenta)` ✓ |
| `bubbles/spinner` | "en attente de la machine cliente" (drawer probe), opérations longues | frames `Meter`, `Dot`, `Line` — choisir `spinner.Dot` ou `spinner.Line` |
| `bubbles/help` | status bar | mode short par défaut, long via `?` (ou utiliser notre overlay help) |
| `bubbles/key` | tous les écrans | déclarer `KeyMap` global + per-screen, exposer `ShortHelp()` et `FullHelp()` pour `bubbles/help` |
| `bubbles/cursor` | piloté par textinput | n/a directement |
| `bubbles/list` | facultatif (pourrait remplacer `table` si on préfère cards multi-lignes) | non retenu par défaut |
| `huh` | Settings + onboarding étape 3 si form plat plus rapide | facultatif |

### 11.4 Idiomes lipgloss à connaître

```go
// Bordure haut/bas seulement (pour un séparateur de header de box) :
lipgloss.NewStyle().
    Border(lipgloss.NormalBorder(), false /*top*/, false /*right*/, true /*bottom*/, false /*left*/).
    BorderForeground(Palette.Border)

// "Border-left magenta" sur la ligne sélectionnée (équiv du box-shadow inset) :
lipgloss.NewStyle().
    Border(lipgloss.ThickBorder(), false, false, false, true). // ┃ uniquement à gauche
    BorderForeground(Palette.Magenta)

// Centrer une modale sur le terminal :
lipgloss.Place(termW, termH, lipgloss.Center, lipgloss.Center, modalView,
    lipgloss.WithWhitespaceChars(" "))

// Composer overlay sur fond :
bg := m.renderActiveView()           // string
bg = lipgloss.NewStyle().Faint(true).Render(bg)
overlay := m.topOverlay.View()
return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, overlay,
    lipgloss.WithWhitespaceChars(string([]rune(bg))))  // ou un helper de composition ligne-à-ligne
// NB : la vraie composition pixel-par-cell se fait via un utilitaire maison qui split chaque
// string en lignes et superpose, parce que lipgloss.Place "écrit par-dessus" en remplaçant.
// Recommandation pratique : ne pas chercher la transparence ligne par ligne ; rendre le scrim
// comme un grand rectangle Faint, et la modale dessus avec ses propres bordures.
```

### 11.5 Couleurs et terminaux

- Spécifier les couleurs en **hex** (`lipgloss.Color("#ff36d4")`) — lipgloss détecte le TrueColor / 256 / ANSI 16 et dégrade automatiquement.
- Vérifier le rendu sur un terminal en **256-color** : les couleurs `#ff36d4` ↦ ~magenta, `#00f0ff` ↦ ~cyan. La palette néon reste lisible.
- Vérifier **fond noir** : `lipgloss.Color("#05050d")` ↦ ANSI black sur 256. OK.
- Vérifier `lipgloss.HasDarkBackground()` au démarrage et basculer le thème "mono" si l'utilisateur a un fond clair, ou refuser de rendre en clair (option Settings).

### 11.6 Compteurs Dashboard — donner du poids sans `font-size`

Trois options classées par effort :

1. **Bold + couleur saturée** (zéro effort) : `47` rendu en magenta bold. Suffisant si la densité est respectée.
2. **Padding et bordure proéminente** : entourer chaque tile d'une bordure colorée 1 ligne, padding 2 lignes, header séparé.
3. **ASCII-art figlet** (`github.com/common-nighthawk/go-figure` ou `github.com/mbndr/figlet4go`) : rendre `47` en font `small` ou `digital` sur 3-4 lignes. Effet "compteur géant" comme dans `htop`. Coût : 1 dépendance + ~30 lignes d'intégration. Recommandé si l'opérateur passe beaucoup de temps sur le Dashboard.

### 11.7 Choses du proto à NE PAS chercher à reproduire

- Le panneau "Tour" (bouton bas-droite + slide-out) est un outil de revue de design, pas une feature de la TUI.
- Les animations de focus (pulse, blink autre que cursor).
- Les coins arrondis avec un radius précis — utiliser `RoundedBorder()` (un seul style) ou rester sur `NormalBorder()`.
- Les transitions entre écrans (fade, slide). Les changements d'onglet sont instantanés.
- Le scintillement / scanlines sur le fond.
- Les "shadow" et "glow" : tous les effets de lumière se traduisent en **bold + couleur saturée** uniquement.

### 11.8 Checklist d'implémentation pas-à-pas

1. Squelette : `main.go` ouvre la DB ou bascule onboarding ; `tea.NewProgram(rootModel{}, tea.WithAltScreen(), tea.WithMouseCellMotion()).Run()`.
2. `theme.go` : copier le bloc lipgloss du §9 verbatim. Tester avec une vue Dashboard statique pour valider la palette sur ton terminal cible.
3. `chrome.go` : titlebar + tab strip + status bar + breadcrumb. Tout statique, gardé en haut et bas, le centre est laissé au sub-model actif.
4. Dashboard d'abord (le plus simple, valide le layout 4-colonnes + couleurs).
5. Supervisor des serveurs (cf §6) **avant** l'écran Servers — sinon tu n'as rien à brancher.
6. Servers écran avec live log → valide `viewport` et le pattern d'abonnement bus.
7. Licenses table → valide `bubbles/table` + split-pane détail.
8. Wizard licence → le plus gros morceau ; commencer par les étapes 1, 2, 4 (forms simples), puis 5 (filepicker + progress), puis 3 (binding editor + drawer probe), puis 6, 7, 8.
9. Overlays restants (revoke, qr, error, ok, help, quit, filepicker wrapper).
10. Audit + Settings + Identities/Recipients (faciles, copient les patterns).
11. Onboarding + Passphrase prompt.
12. Polish : `bubbles/help`, raccourcis manquants, gestion `tea.WindowSizeMsg` partout.

---

## Annexe — quelques use-cases tracés

**(2) Licence distante zéro-friction**.
`Licenses → n → wizard étape 3 → m (+ machine) → r (récupérer machine distante) → drawer probe s'ouvre → opérateur copie token/URL → Alice run le curl one-liner → drawer affiche fingerprint → opérateur clique "ajouter les 2 (OR)" → drawer ferme + probe server stoppe auto → wizard continue étape 4 → … → étape 8 → Émettre → modal OK → retour Licences avec la nouvelle ligne en tête.`

**(5) Révocation rapide**.
`Dashboard → / → tape "alice" → Enter → expand row (d) → x → modal revoke → tape "key_compromised" → Enter → CRL maj → si server révocation ON, push immédiat ; toast vert "révoquée + publiée".`

**(8) Fermeture sûre**.
`q → quit overlay liste les serveurs ON → Enter → tea.Sequence(StopAll, CloseDB, Quit) → terminal rendu propre.`

---

*Fin du handoff. Toute ambiguïté restante après lecture indique un trou de design ; remonte au designer plutôt que d'improviser silencieusement.*
