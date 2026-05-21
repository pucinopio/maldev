// gallery.jsx — renders every screen + key overlays in stacked 1440x900 frames.
// Designed to be printed: Cmd/Ctrl+P → Save as PDF.

const { useState } = React;
const D = window.DATA;

// Frame chrome (replays the same titlebar/tabs/crumb/status bar as index.html)
function Frame({ id, title, sub, active, crumb, hints, drawer, modal, children, wizardFull }) {
  return (
    <section className="frame">
      <h2 className="frame-title">
        <span className="id">{id}</span>
        <span>{title}</span>
        {sub && <span className="sub">— {sub}</span>}
      </h2>
      <div className="frame-viewport">
        <div style={{ display: "flex", flexDirection: "column", height: "100%", width: "100%", overflow: "hidden" }}>
          <Titlebar db="db.sqlite" server_on_count={2} online />
          <TabStrip active={active} onSwitch={() => {}} />
          {!wizardFull && <Crumb items={crumb || [active]} />}
          <div style={{ flex: 1, minHeight: 0, position: "relative", overflow: "hidden" }}>
            {children}
            {drawer}
            {modal}
          </div>
          <StatusBar hints={hints || []} />
        </div>
      </div>
    </section>
  );
}

// Full-screen frame (no chrome) — for onboarding + passphrase prompt.
function PlainFrame({ id, title, sub, children }) {
  return (
    <section className="frame">
      <h2 className="frame-title">
        <span className="id">{id}</span>
        <span>{title}</span>
        {sub && <span className="sub">— {sub}</span>}
      </h2>
      <div className="frame-viewport">
        <div style={{ position: "relative", height: "100%", width: "100%", overflow: "hidden", background: "var(--bg)" }}>
          {children}
        </div>
      </div>
    </section>
  );
}

// --- helpers for per-screen state stubs --------------------------------
const noop = () => {};

function DashboardFrame()  { return <DashboardScreen goto={noop} />; }

function LicensesFrame({ sel = 1, tab = "ident" }) {
  return (
    <LicensesScreen
      sel={sel} setSel={noop}
      search="" setSearch={noop}
      filters={{ status: null }} setFilters={noop}
      detailOpen detailTab={tab} setDetailTab={noop}
      openOverlay={noop} openWizard={noop}
    />
  );
}

function WizardFrame({ step }) {
  const done = { identity: step > 0, validity: step > 1, bindings: step > 2, features: step > 3, pinning: step > 4, payload: step > 5, sealed: step > 6 };
  const [wiz, setWiz] = useState({ step, done });
  return <WizardScreen wiz={wiz} setWiz={setWiz} openOverlay={noop} closeOverlay={noop} />;
}

function IssuersFrame()    { return <IssuersScreen    sel={0} setSel={noop} />; }
function RecipientsFrame() { return <RecipientsScreen sel={0} />; }
function IdentitiesFrame() { return <IdentitiesScreen sel={0} openOverlay={noop} />; }
function RevocationFrame() { return <RevocationScreen sel={0} />; }
function ServersFrame({ sub, view = "tokens" }) {
  return <ServersScreen sub={sub} setSub={noop} probeView={view} setProbeView={noop} openOverlay={noop} />;
}
function AuditFrame()      { return <AuditScreen     sel={1} setSel={noop} filters={{ kind: null }} setFilters={noop} />; }
function SettingsFrame()   { return <SettingsScreen  openOverlay={noop} />; }

// Hints lists per tab (reused from app.jsx)
const HINTS = {
  dashboard:  [{ k: "1-9", t: "onglets" }, { k: "n", t: "nouvelle licence" }, { k: "/", t: "rechercher" }, { k: "k", t: "clés actives" }, { k: "?", t: "aide" }, { k: "q", t: "quitter" }],
  licenses:   [{ k: "↑↓", t: "naviguer" }, { k: "d", t: "détail" }, { k: "n", t: "nouvelle" }, { k: "/", t: "rechercher" }, { k: "f", t: "filtre" }, { k: "x", t: "révoquer" }, { k: "e", t: "re-émettre" }],
  issuers:    [{ k: "↑↓", t: "naviguer" }, { k: "n", t: "générer" }, { k: "i", t: "importer" }, { k: "a", t: "désigner active" }, { k: "E", t: "export .pub" }, { k: "K", t: "export .key" }, { k: "x", t: "retirer" }],
  recipients: [{ k: "↑↓", t: "naviguer" }, { k: "n", t: "générer" }, { k: "i", t: "importer" }, { k: "E", t: "export .pub" }, { k: "x", t: "retirer" }],
  identities: [{ k: "↑↓", t: "naviguer" }, { k: "n", t: "créer" }, { k: "E", t: "export .bin" }, { k: "R", t: "régénérer ⚠" }, { k: "x", t: "supprimer" }],
  revocation: [{ k: "↑↓", t: "naviguer" }, { k: "n", t: "ajouter" }, { k: "x", t: "retirer" }, { k: "E", t: "exporter CRL signée" }],
  servers:    [{ k: "r/h/p", t: "sous-onglets" }, { k: "s", t: "start/stop" }, { k: "e", t: "config" }, { k: "g", t: "régénérer token" }, { k: "c", t: "clear log" }],
  audit:      [{ k: "↑↓", t: "naviguer" }, { k: "d", t: "détail" }, { k: "f", t: "filtre kind" }, { k: "/", t: "rechercher target" }, { k: "E", t: "export CSV" }, { k: "J", t: "export JSON" }],
  settings:   [{ k: "↑↓", t: "naviguer" }, { k: "Space", t: "toggle" }, { k: "P", t: "rekey passphrase" }],
};

// --- crumbs per state ---
const CRUMBS = {
  dashboard:    ["dashboard"],
  licenses_ident: ["licences", "liste (12)", "alice@research", "Identité"],
  licenses_bind:  ["licences", "liste (12)", "alice@research", "Bindings"],
  licenses_pem:   ["licences", "liste (12)", "alice@research", "PEM"],
  licenses_chain: ["licences", "liste (12)", "alice@research (superseded)", "Chaîne"],
  issuers:      ["clés d'émission", "Ed25519", "k2026-04"],
  recipients:   ["recipients", "X25519", "r2026-01"],
  identities:   ["identities", "rshell-windows-amd64.bin"],
  revocation:   ["révocation", "CRL (4)"],
  servers_revoc:["serveurs HTTP", "Revocation"],
  servers_hb:   ["serveurs HTTP", "Heartbeat"],
  servers_probe:["serveurs HTTP", "Fingerprint probe"],
  audit:        ["audit", "entries (28)"],
  settings:     ["settings"],
};

// Overlay wrappers — re-use OverlayHost with a forced overlay object
function OverlayOn({ kind, extra = {} }) {
  return <OverlayHost overlay={{ kind, ...extra }} closeOverlay={noop} openOverlay={noop} />;
}

// --- the gallery itself ---
function Gallery() {
  const items = [
    // SESSION / BOOT
    { id: "00", title: "Démarrage — DB existante, passphrase à entrer", sub: "§3.12 · cascade non-résolue", group: "Session",
      el: <PlainFrame id="00" title="Passphrase prompt" sub="§3.12">
            <OnboardingScreen mode="passphrase" setSession={noop} openOverlay={noop} />
          </PlainFrame> },
    { id: "01", title: "Onboarding — Bienvenue", sub: "§3.11 · première utilisation, étape 1/4", group: "Session",
      el: <PlainFrame id="01" title="First-run wizard"><OnboardingScreen mode="first-run" setSession={noop} /></PlainFrame> },

    // CORE 9 TABS
    { id: "02", title: "Dashboard", sub: "§3.1 · compteurs + clé active + serveurs + audit récent", group: "Onglets",
      el: <Frame id="02" title="Dashboard" active="dashboard" crumb={CRUMBS.dashboard} hints={HINTS.dashboard}><DashboardFrame /></Frame> },
    { id: "03", title: "Licenses · détail Identité", sub: "§3.2 · split-pane sous la table", group: "Onglets",
      el: <Frame id="03" title="Licenses" sub="onglet I (Identité)" active="licenses" crumb={CRUMBS.licenses_ident} hints={HINTS.licenses}><LicensesFrame sel={1} tab="ident" /></Frame> },
    { id: "04", title: "Licenses · détail Bindings", sub: "§3.2 · machine OR + TOTP + pinning", group: "Onglets",
      el: <Frame id="04" title="Licenses" sub="onglet B (Bindings)" active="licenses" crumb={CRUMBS.licenses_bind} hints={HINTS.licenses}><LicensesFrame sel={0} tab="bind" /></Frame> },
    { id: "05", title: "Licenses · détail PEM", sub: "§3.2 · viewport scrollable", group: "Onglets",
      el: <Frame id="05" title="Licenses" sub="onglet P (PEM)" active="licenses" crumb={CRUMBS.licenses_pem} hints={HINTS.licenses}><LicensesFrame sel={0} tab="pem" /></Frame> },
    { id: "06", title: "Licenses · détail Chaîne (superseded)", sub: "§3.2 · parent → cette licence → successeurs", group: "Onglets",
      el: <Frame id="06" title="Licenses" sub="onglet C (Chaîne) — licence superseded" active="licenses" crumb={CRUMBS.licenses_chain} hints={HINTS.licenses}><LicensesFrame sel={11} tab="chain" /></Frame> },
    { id: "07", title: "Issuer keys (Ed25519)", sub: "§3.4", group: "Onglets",
      el: <Frame id="07" title="Issuer keys" active="issuers" crumb={CRUMBS.issuers} hints={HINTS.issuers}><IssuersFrame /></Frame> },
    { id: "08", title: "Recipient keys (X25519)", sub: "§3.5", group: "Onglets",
      el: <Frame id="08" title="Recipient keys" active="recipients" crumb={CRUMBS.recipients} hints={HINTS.recipients}><RecipientsFrame /></Frame> },
    { id: "09", title: "Identities binaires (identity.bin)", sub: "§3.6", group: "Onglets",
      el: <Frame id="09" title="Identities" active="identities" crumb={CRUMBS.identities} hints={HINTS.identities}><IdentitiesFrame /></Frame> },
    { id: "10", title: "Revocation (CRL)", sub: "§3.7", group: "Onglets",
      el: <Frame id="10" title="Revocation" active="revocation" crumb={CRUMBS.revocation} hints={HINTS.revocation}><RevocationFrame /></Frame> },
    { id: "11", title: "Servers · Revocation", sub: "§3.8 · status + config + live log", group: "Onglets",
      el: <Frame id="11" title="Servers — Revocation" active="servers" crumb={CRUMBS.servers_revoc} hints={HINTS.servers}><ServersFrame sub="revoc" /></Frame> },
    { id: "12", title: "Servers · Heartbeat", sub: "§3.8 · live log + toggle licences", group: "Onglets",
      el: <Frame id="12" title="Servers — Heartbeat" active="servers" crumb={CRUMBS.servers_hb} hints={HINTS.servers}><ServersFrame sub="heartbeat" /></Frame> },
    { id: "13", title: "Servers · Probe — Tokens actifs", sub: "§3.8 · table avec TTL countdown", group: "Onglets",
      el: <Frame id="13" title="Servers — Probe" sub="vue T (Tokens)" active="servers" crumb={CRUMBS.servers_probe} hints={HINTS.servers}><ServersFrame sub="probe" view="tokens" /></Frame> },
    { id: "14", title: "Servers · Probe — History", sub: "§3.8 · fingerprints reçus, lien vers licence", group: "Onglets",
      el: <Frame id="14" title="Servers — Probe" sub="vue H (History)" active="servers" crumb={CRUMBS.servers_probe} hints={HINTS.servers}><ServersFrame sub="probe" view="history" /></Frame> },
    { id: "15", title: "Audit", sub: "§3.9 · liste paginée + filtres", group: "Onglets",
      el: <Frame id="15" title="Audit" active="audit" crumb={CRUMBS.audit} hints={HINTS.audit}><AuditFrame /></Frame> },
    { id: "16", title: "Settings", sub: "§3.10 · defaults + cycle de vie + cascade passphrase", group: "Onglets",
      el: <Frame id="16" title="Settings" active="settings" crumb={CRUMBS.settings} hints={HINTS.settings}><SettingsFrame /></Frame> },

    // WIZARD 8 ÉTAPES (full-screen)
    { id: "17", title: "Wizard licence · étape 1 — Identité", sub: "§3.3", group: "Wizard nouvelle licence",
      el: <Frame id="17" title="Wizard" sub="étape 1/8 — Identité" wizardFull active="licenses" hints={[{k:"Tab",t:"suivant"},{k:"⇧Tab",t:"précédent"},{k:"1-8",t:"aller à"},{k:"esc",t:"annuler"}]}><WizardFrame step={0} /></Frame> },
    { id: "18", title: "Wizard licence · étape 2 — Validité", sub: "§3.3", group: "Wizard nouvelle licence",
      el: <Frame id="18" title="Wizard" sub="étape 2/8 — Validité" wizardFull active="licenses" hints={[{k:"Tab",t:"suivant"},{k:"⇧Tab",t:"précédent"},{k:"1-8",t:"aller à"},{k:"esc",t:"annuler"}]}><WizardFrame step={1} /></Frame> },
    { id: "19", title: "Wizard licence · étape 3 — Bindings", sub: "§3.3 · machine / password / TOTP / k/v empilables", group: "Wizard nouvelle licence",
      el: <Frame id="19" title="Wizard" sub="étape 3/8 — Bindings" wizardFull active="licenses" hints={[{k:"m",t:"+machine"},{k:"p",t:"+password"},{k:"t",t:"+TOTP"},{k:"k",t:"+k/v"},{k:"r",t:"probe distant"}]}><WizardFrame step={2} /></Frame> },
    { id: "20", title: "Wizard licence · étape 4 — Features", sub: "§3.3 · chips multi-select avec autocomplete", group: "Wizard nouvelle licence",
      el: <Frame id="20" title="Wizard" sub="étape 4/8 — Features" wizardFull active="licenses" hints={[{k:"Tab",t:"suivant"},{k:"⇧Tab",t:"précédent"}]}><WizardFrame step={3} /></Frame> },
    { id: "21", title: "Wizard licence · étape 5 — Pinning", sub: "§3.3 · IdentitySHA + BinarySHA (filepicker + progress)", group: "Wizard nouvelle licence",
      el: <Frame id="21" title="Wizard" sub="étape 5/8 — Pinning" wizardFull active="licenses" hints={[{k:"↵",t:"filepicker"},{k:"v",t:"coller hash"}]}><WizardFrame step={4} /></Frame> },
    { id: "22", title: "Wizard licence · étape 6 — Payload", sub: "§3.3 · vide / JSON inline / import fichier", group: "Wizard nouvelle licence",
      el: <Frame id="22" title="Wizard" sub="étape 6/8 — Payload" wizardFull active="licenses" hints={[{k:"1",t:"vide"},{k:"2",t:"JSON"},{k:"3",t:"importer"}]}><WizardFrame step={5} /></Frame> },
    { id: "23", title: "Wizard licence · étape 7 — Sealed payload", sub: "§3.3 · NaCl box pour recipient", group: "Wizard nouvelle licence",
      el: <Frame id="23" title="Wizard" sub="étape 7/8 — Sealed (optionnel)" wizardFull active="licenses" hints={[{k:"Tab",t:"sauter"},{k:"↵",t:"valider"}]}><WizardFrame step={6} /></Frame> },
    { id: "24", title: "Wizard licence · étape 8 — Récap & émettre", sub: "§3.3 · preview PEM + actions sortie", group: "Wizard nouvelle licence",
      el: <Frame id="24" title="Wizard" sub="étape 8/8 — Récap" wizardFull active="licenses" hints={[{k:"↵",t:"émettre"},{k:"⇧Tab",t:"précédent"},{k:"esc",t:"annuler"}]}><WizardFrame step={7} /></Frame> },

    // OVERLAYS — sur fond Licenses (le plus pertinent)
    { id: "25", title: "Overlay · Révoquer", sub: "§5 · modal danger sur fond Licenses", group: "Overlays",
      el: <Frame id="25" title="Overlay" sub="révoquer" active="licenses" crumb={CRUMBS.licenses_ident} hints={HINTS.licenses} modal={<OverlayOn kind="revoke" extra={{ lic: "lic-bob-71bd", keyid: "k2026-04" }} />}><LicensesFrame sel={1} tab="ident" /></Frame> },
    { id: "26", title: "Overlay · Quitter (serveurs ON)", sub: "§5 · modal danger globale", group: "Overlays",
      el: <Frame id="26" title="Overlay" sub="quit" active="dashboard" crumb={CRUMBS.dashboard} hints={HINTS.dashboard} modal={<OverlayOn kind="quit" />}><DashboardFrame /></Frame> },
    { id: "27", title: "Overlay · QR TOTP", sub: "§5 · QR ASCII + secret + export PNG", group: "Overlays",
      el: <Frame id="27" title="Overlay" sub="qr (TOTP alice)" active="licenses" crumb={CRUMBS.licenses_bind} hints={HINTS.licenses} modal={<OverlayOn kind="qr" />}><LicensesFrame sel={0} tab="bind" /></Frame> },
    { id: "28", title: "Overlay · Filepicker", sub: "§5 · bubbles/filepicker enveloppé en modal", group: "Overlays",
      el: <Frame id="28" title="Overlay" sub="filepicker (binary SHA wizard)" wizardFull active="licenses" hints={[{k:"↑↓",t:"nav"},{k:"↵",t:"choisir"},{k:"esc",t:"annuler"}]} modal={<OverlayOn kind="filepicker" />}><WizardFrame step={4} /></Frame> },
    { id: "29", title: "Overlay · Aide globale", sub: "§5 · touche ?", group: "Overlays",
      el: <Frame id="29" title="Overlay" sub="help" active="dashboard" crumb={CRUMBS.dashboard} hints={HINTS.dashboard} modal={<OverlayOn kind="help" />}><DashboardFrame /></Frame> },
    { id: "30", title: "Overlay · OK — licence émise", sub: "§5 · modal succès post-wizard", group: "Overlays",
      el: <Frame id="30" title="Overlay" sub="ok" active="licenses" crumb={CRUMBS.licenses_ident} hints={HINTS.licenses} modal={<OverlayOn kind="ok" extra={{ title: "Licence émise", body: "lic:9f3a-b21c-… ajoutée à la base et signée par k2026-04. Onglet Licences pour la voir." }} />}><LicensesFrame sel={1} tab="ident" /></Frame> },
    { id: "31", title: "Overlay · Erreur (port :8443 occupé)", sub: "§8 · erreur serveur avec recover", group: "Overlays",
      el: <Frame id="31" title="Overlay" sub="error_port" active="servers" crumb={CRUMBS.servers_revoc} hints={HINTS.servers} modal={<OverlayOn kind="error" extra={{ title: "Port :8443 occupé", body: "Impossible de démarrer le serveur de révocation : address already in use (errno 98).", details: "listen tcp 0.0.0.0:8443: bind: address already in use", recover: "essayer :8543" }} />}><ServersFrame sub="revoc" /></Frame> },
    { id: "32", title: "Overlay · Reissue refusée (superseded)", sub: "§8 · violet sur fond Licenses", group: "Overlays",
      el: <Frame id="32" title="Overlay" sub="reissue_blocked" active="licenses" crumb={CRUMBS.licenses_chain} hints={HINTS.licenses} modal={<OverlayOn kind="reissue_blocked" extra={{ successor: "lic-alice-9f3a" }} />}><LicensesFrame sel={11} tab="chain" /></Frame> },
    { id: "33", title: "Overlay · Identity blocked (refs > 0)", sub: "§8 · jaune", group: "Overlays",
      el: <Frame id="33" title="Overlay" sub="identity_blocked" active="identities" crumb={CRUMBS.identities} hints={HINTS.identities} modal={<OverlayOn kind="identity_blocked" extra={{ name: "rshell-windows-amd64.bin", refs: 22 }} />}><IdentitiesFrame /></Frame> },
    { id: "34", title: "Overlay · Rekey passphrase DB", sub: "§3.10 · transactionnel", group: "Overlays",
      el: <Frame id="34" title="Overlay" sub="rekey" active="settings" crumb={CRUMBS.settings} hints={HINTS.settings} modal={<OverlayOn kind="rekey" />}><SettingsFrame /></Frame> },
    { id: "35", title: "Overlay · Probe regen admin token", sub: "§6.6 · stop+regen+restart", group: "Overlays",
      el: <Frame id="35" title="Overlay" sub="probe_regen" active="servers" crumb={CRUMBS.servers_revoc} hints={HINTS.servers} modal={<OverlayOn kind="probe_regen" />}><ServersFrame sub="revoc" /></Frame> },
    { id: "36", title: "Overlay · Garder ProbeServer ON ?", sub: "§5 · après consommation de token", group: "Overlays",
      el: <Frame id="36" title="Overlay" sub="probe_keep" active="servers" crumb={CRUMBS.servers_probe} hints={HINTS.servers} modal={<OverlayOn kind="probe_keep" />}><ServersFrame sub="probe" /></Frame> },

    // DRAWER (62% right) — fingerprint probe
    { id: "37", title: "Drawer · Fingerprint probe — en attente (bash)", sub: "§3.8 / §6.5 · token affiché + snippet bash + spinner", group: "Drawer probe",
      el: <Frame id="37" title="Drawer" sub="probe_drawer (waiting, tab bash)" wizardFull active="licenses" hints={[{k:"esc",t:"fermer"}]} drawer={<OverlayOn kind="probe_drawer" extra={{ phase: "waiting" }} />}><WizardFrame step={2} /></Frame> },
    { id: "38", title: "Drawer · Fingerprint probe — fingerprint reçu", sub: "§3.8 / §6.5 · 3 boutons sortie", group: "Drawer probe",
      el: <Frame id="38" title="Drawer" sub="probe_drawer (received)" wizardFull active="licenses" hints={[{k:"↵",t:"ajouter les 2"},{k:"L",t:"Local only"},{k:"C",t:"Composite only"},{k:"esc",t:"ignorer"}]} drawer={<OverlayOn kind="probe_drawer" extra={{ phase: "received" }} />}><WizardFrame step={2} /></Frame> },
  ];

  // ToC
  React.useEffect(() => {
    const toc = document.getElementById("toc");
    if (!toc || toc.querySelector("a")) return;
    let lastGroup = "";
    items.forEach(it => {
      if (it.group !== lastGroup) {
        const h = document.createElement("div");
        h.className = "toc-title";
        h.textContent = it.group;
        h.style.marginTop = "8px";
        toc.appendChild(h);
        lastGroup = it.group;
      }
      const a = document.createElement("a");
      a.href = "#" + it.id;
      a.textContent = it.id + " — " + it.title;
      toc.appendChild(a);
    });
  }, []);

  return (
    <div>
      {items.map(it => (
        <div key={it.id} id={it.id}>
          {it.el}
        </div>
      ))}
    </div>
  );
}

ReactDOM.createRoot(document.getElementById("root")).render(<Gallery />);
