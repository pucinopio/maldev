// Root App — central model, key router, screen switcher, overlay host

const { useState, useEffect, useCallback, useRef } = React;

function App() {
  // --- central state ----------------------------------------------------
  const [session, setSession] = useState("ready"); // "ready" | "locked" | "onboarding"
  const [active, setActive]   = useState("dashboard");
  const [overlay, setOverlay] = useState(null);     // single overlay (modal/drawer)
  const [overlayStack, setOverlayStack] = useState([]); // for nested (rare)
  const [lastKey, setLastKey] = useState(null);
  const [msg, setMsg]         = useState(null);

  // per-screen sub-state
  const [licSel, setLicSel] = useState(0);
  const [licSearch, setLicSearch] = useState("");
  const [licFilters, setLicFilters] = useState({ status: null });
  const [licDetailOpen, setLicDetailOpen] = useState(true);
  const [licDetailTab, setLicDetailTab] = useState("ident");
  const [issSel, setIssSel] = useState(0);
  const [recSel, setRecSel] = useState(0);
  const [idSel, setIdSel]   = useState(0);
  const [revSel, setRevSel] = useState(0);
  const [audSel, setAudSel] = useState(0);
  const [audFilters, setAudFilters] = useState({ kind: null });
  const [srvSub, setSrvSub] = useState("revoc");
  const [probeView, setProbeView] = useState("tokens");

  // wizard state
  const [wiz, setWiz] = useState({ step: 0, done: {} });

  const openOverlay = useCallback((o) => setOverlay(o), []);
  const closeOverlay = useCallback(() => setOverlay(null), []);

  // expose pour scripts de screenshot externes
  React.useEffect(() => {
    window.__jump = (slug) => {
      if (slug.startsWith("tab:")) setActive(slug.slice(4));
      else if (slug === "onboarding") setSession("onboarding");
      else if (slug === "locked")     setSession("locked");
      else if (slug === "ready")      setSession("ready");
      else if (slug.startsWith("wizard:")) {
        const n = parseInt(slug.split(":")[1], 10);
        setActive("licenses"); setWiz({ step: n, done: {} }); setOverlay({ kind: "wizard" });
      }
      else if (slug.startsWith("servers:")) {
        const parts = slug.split(":"); // servers:sub[:probeView]
        setActive("servers"); setSrvSub(parts[1]);
        if (parts[1] === "probe" && parts[2]) setProbeView(parts[2]);
      }
      else if (slug.startsWith("licenses:")) {
        const [, idx, tab] = slug.split(":");
        setActive("licenses");
        setLicSel(parseInt(idx, 10) || 0);
        setLicDetailOpen(true);
        if (tab) setLicDetailTab(tab);
      }
      else if (slug.startsWith("overlay:")) {
        const parts = slug.split(":");
        const kind = parts[1];
        if (kind === "qr") setOverlay({ kind: "qr" });
        else if (kind === "revoke") setOverlay({ kind: "revoke" });
        else if (kind === "quit") setOverlay({ kind: "quit" });
        else if (kind === "help") setOverlay({ kind: "help" });
        else if (kind === "filepicker") setOverlay({ kind: "filepicker" });
        else if (kind === "probe_drawer") setOverlay({ kind: "probe_drawer" });
        else if (kind === "probe_drawer_received") setOverlay({ kind: "probe_drawer", phase: "received" });
        else if (kind === "probe_regen") setOverlay({ kind: "probe_regen" });
        else if (kind === "probe_keep") setOverlay({ kind: "probe_keep" });
        else if (kind === "rekey") setOverlay({ kind: "rekey" });
        else if (kind === "identity_blocked") setOverlay({ kind: "identity_blocked", name: "rshell-windows-amd64.bin", refs: 22 });
        else if (kind === "reissue_blocked") setOverlay({ kind: "reissue_blocked", successor: "lic-alice-9f3a" });
        else if (kind === "ok") setOverlay({ kind: "ok", title: "Licence émise", body: "lic:9f3a-b21c-… ajoutée à la base et signée par k2026-04." });
        else if (kind === "error_port") setOverlay({ kind: "error", title: "Port :8443 occupé", body: "Impossible de démarrer le serveur de révocation : address already in use (errno 98).", details: "listen tcp 0.0.0.0:8443: bind: address already in use", recover: "essayer :8543" });
      }
      else if (slug === "close") setOverlay(null);
    };
    return () => { delete window.__jump; };
  });

  // tour helper — used by the right-side demo panel
  const tourJump = useCallback((slug) => {
    if (slug.startsWith("tab:")) setActive(slug.slice(4));
    else if (slug === "onboarding") setSession("onboarding");
    else if (slug === "locked")     setSession("locked");
    else if (slug === "ready")      setSession("ready");
    else if (slug === "wizard:0")   { setActive("licenses"); setWiz({ step: 0, done: {} }); setOverlay({ kind: "wizard" }); }
    else if (slug.startsWith("wizard:")) {
      const n = parseInt(slug.split(":")[1], 10);
      setActive("licenses"); setWiz({ step: n, done: {} }); setOverlay({ kind: "wizard" });
    }
    else if (slug === "probe") { setActive("licenses"); setWiz({ step: 2, done: {} }); setOverlay({ kind: "wizard" }); setTimeout(() => setOverlay({ kind: "wizard", chained: "probe_drawer" }), 0); }
    else if (slug === "probe-drawer") openOverlay({ kind: "probe_drawer" });
    else if (slug === "qr")             openOverlay({ kind: "qr" });
    else if (slug === "revoke")         openOverlay({ kind: "revoke" });
    else if (slug === "quit")           openOverlay({ kind: "quit" });
    else if (slug === "filepicker")     openOverlay({ kind: "filepicker" });
    else if (slug === "help")           openOverlay({ kind: "help" });
    else if (slug === "error-port")     openOverlay({ kind: "error", title: "Port :8443 occupé", body: "Impossible de démarrer le serveur de révocation : address already in use (errno 98).", details: "listen tcp 0.0.0.0:8443: bind: address already in use", recover: "essayer :8543" });
    else if (slug === "error-pass")     openOverlay({ kind: "error", title: "Passphrase incorrecte", body: "Impossible de déverrouiller la base. Réessaye ou récupère la passphrase depuis ton gestionnaire.", recover: "réessayer" });
    else if (slug === "error-token")    openOverlay({ kind: "error", title: "Token expiré", body: "Le token /probe/tk_… a expiré (1 min). Génère-en un nouveau depuis le wizard ou l'onglet Servers." });
    else if (slug === "error-locked")   openOverlay({ kind: "error", title: "Base SQLite verrouillée", body: "Une autre instance de license-manager semble tourner et a verrouillé db.sqlite. Ferme-la ou attends.", recover: "réessayer" });
    else if (slug === "ok-issued")      openOverlay({ kind: "ok", title: "Licence émise", body: "lic:9f3a-b21c-… a été ajoutée à la base et signée par k2026-04. Onglet Licences pour la voir." });
  }, [openOverlay]);

  // --- key router -------------------------------------------------------
  useEffect(() => {
    function onKey(e) {
      // skip modifier-only keystrokes
      if (e.key === "Shift" || e.key === "Control" || e.key === "Alt" || e.key === "Meta") return;
      let label = e.key;
      if (e.key === " ") label = "Space";
      if (e.shiftKey && e.key !== "Tab") label = "⇧" + label;
      setLastKey(label);

      // overlay first — esc closes
      if (overlay) {
        if (e.key === "Escape") { setOverlay(null); e.preventDefault(); return; }
        // wizard owns its own keys, including 1-8 for step jump
        if (overlay.kind === "wizard") {
          if (e.key === "Tab" && !e.shiftKey) { setWiz(w => ({ ...w, step: Math.min(w.step + 1, WIZARD_STEPS.length - 1), done: { ...w.done, [WIZARD_STEPS[w.step].key]: true } })); e.preventDefault(); return; }
          if (e.shiftKey && e.key === "Tab") { setWiz(w => ({ ...w, step: Math.max(w.step - 1, 0) })); e.preventDefault(); return; }
          if (/^[1-8]$/.test(e.key))          { setWiz(w => ({ ...w, step: parseInt(e.key, 10) - 1 })); e.preventDefault(); return; }
          if (e.key === "Enter")              { setWiz(w => ({ ...w, step: Math.min(w.step + 1, WIZARD_STEPS.length - 1), done: { ...w.done, [WIZARD_STEPS[w.step].key]: true } })); e.preventDefault(); return; }
          // 9 falls through — but we swallow it so it doesn't switch tabs
          if (e.key === "9") { e.preventDefault(); return; }
          return; // wizard absorbs everything else (form typing, etc — mocked)
        }
        // other overlays: Enter dismisses (mock)
        if (e.key === "Enter") { setOverlay(null); e.preventDefault(); return; }
        return; // overlays absorb keys
      }

      // session gates
      if (session !== "ready") return;

      // global tab switch 1-9
      if (/^[1-9]$/.test(e.key)) {
        const t = TABS.find(x => x.k === e.key);
        if (t) { setActive(t.id); e.preventDefault(); return; }
      }

      // helps + quit + search
      if (e.key === "?") { openOverlay({ kind: "help" }); e.preventDefault(); return; }
      if (e.key === "q") { openOverlay({ kind: "quit" }); e.preventDefault(); return; }

      // per-screen
      if (active === "licenses") {
        if (e.key === "n") { setActive("licenses"); setWiz({ step: 0, done: {} }); setOverlay({ kind: "wizard" }); e.preventDefault(); return; }
        if (e.key === "x") { openOverlay({ kind: "revoke", lic: window.DATA.licenses[licSel]?.subj }); e.preventDefault(); return; }
        if (e.key === "d") { setLicDetailOpen(o => !o); e.preventDefault(); return; }
        if (licDetailOpen) {
          if (e.key === "I" || e.key === "i") { setLicDetailTab("ident"); e.preventDefault(); return; }
          if (e.key === "B" || e.key === "b") { setLicDetailTab("bind");  e.preventDefault(); return; }
          if (e.key === "P") { setLicDetailTab("pem");   e.preventDefault(); return; }
          if (e.key === "A") { setLicDetailTab("audit"); e.preventDefault(); return; }
          if (e.key === "C" || e.key === "c") { setLicDetailTab("chain"); e.preventDefault(); return; }
        }
        if (e.key === "ArrowDown") { setLicSel(s => Math.min(s + 1, window.DATA.licenses.length - 1)); e.preventDefault(); }
        if (e.key === "ArrowUp")   { setLicSel(s => Math.max(s - 1, 0)); e.preventDefault(); }
      }
      if (active === "issuers") {
        if (e.key === "ArrowDown") { setIssSel(s => Math.min(s + 1, window.DATA.issuer_keys.length - 1)); e.preventDefault(); }
        if (e.key === "ArrowUp")   { setIssSel(s => Math.max(s - 1, 0)); e.preventDefault(); }
      }
      if (active === "recipients") {
        if (e.key === "ArrowDown") setRecSel(s => Math.min(s + 1, window.DATA.recipient_keys.length - 1));
        if (e.key === "ArrowUp")   setRecSel(s => Math.max(s - 1, 0));
      }
      if (active === "identities") {
        if (e.key === "x") {
          const id = window.DATA.identities[idSel];
          if (id && id.refs > 0) {
            openOverlay({ kind: "identity_blocked", name: id.name, refs: id.refs });
          } else {
            openOverlay({ kind: "confirm", title: "Supprimer cette identity ?", body: `${id?.name} sera retirée de la DB.`, kind: "danger", confirmLabel: "Supprimer" });
          }
          e.preventDefault(); return;
        }
        if (e.key === "ArrowDown") setIdSel(s => Math.min(s + 1, window.DATA.identities.length - 1));
        if (e.key === "ArrowUp")   setIdSel(s => Math.max(s - 1, 0));
      }
      if (active === "audit") {
        if (e.key === "ArrowDown") setAudSel(s => Math.min(s + 1, window.DATA.audit_long.length - 1));
        if (e.key === "ArrowUp")   setAudSel(s => Math.max(s - 1, 0));
      }
      if (active === "revocation") {
        if (e.key === "ArrowDown") setRevSel(s => Math.min(s + 1, window.DATA.revocations.length - 1));
        if (e.key === "ArrowUp")   setRevSel(s => Math.max(s - 1, 0));
      }
      if (active === "servers") {
        if (e.key === "s") { /* mock: toggle start/stop — no-op */ e.preventDefault(); return; }
        if (e.key === "g") { openOverlay({ kind: "probe_regen" }); e.preventDefault(); return; }
        if (srvSub === "probe") {
          if (e.key === "t" || e.key === "T") { setProbeView("tokens");  e.preventDefault(); return; }
          if (e.key === "H")                  { setProbeView("history"); e.preventDefault(); return; }
          if (e.key === "l" || e.key === "L") { setProbeView("log");     e.preventDefault(); return; }
          if (e.key === "r" || e.key === "R") { setSrvSub("revoc");     e.preventDefault(); return; }
          // pour passer à Heartbeat depuis Probe : Shift+H réservé à history, donc Shift+B/H sont pris ; use mouse or [7]
        } else {
          if (e.key === "r" || e.key === "R") { setSrvSub("revoc");     e.preventDefault(); return; }
          if (e.key === "h" || e.key === "H") { setSrvSub("heartbeat"); e.preventDefault(); return; }
          if (e.key === "p" || e.key === "P") { setSrvSub("probe");     e.preventDefault(); return; }
        }
      }
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [active, session, overlay, licSel, openOverlay]);

  // hints per screen
  const hintsFor = (view) => {
    switch (view) {
      case "dashboard":  return [{ k: "1-9", t: "onglets" }, { k: "n", t: "nouvelle licence" }, { k: "/", t: "rechercher" }, { k: "k", t: "clés actives" }, { k: "?", t: "aide" }, { k: "q", t: "quitter", imp: true }];
      case "licenses":   return [{ k: "↑↓", t: "naviguer" }, { k: "d", t: "détail" }, { k: "n", t: "nouvelle", imp: true }, { k: "/", t: "rechercher" }, { k: "f", t: "filtre" }, { k: "x", t: "révoquer" }, { k: "e", t: "re-émettre" }];
      case "issuers":    return [{ k: "↑↓", t: "naviguer" }, { k: "n", t: "générer", imp: true }, { k: "i", t: "importer" }, { k: "a", t: "désigner active" }, { k: "E", t: "export .pub" }, { k: "K", t: "export .key" }, { k: "x", t: "retirer" }];
      case "recipients": return [{ k: "↑↓", t: "naviguer" }, { k: "n", t: "générer" }, { k: "i", t: "importer" }, { k: "E", t: "export .pub" }, { k: "x", t: "retirer" }];
      case "identities": return [{ k: "↑↓", t: "naviguer" }, { k: "n", t: "créer" }, { k: "E", t: "export .bin" }, { k: "R", t: "régénérer ⚠" }, { k: "x", t: "supprimer" }];
      case "revocation": return [{ k: "↑↓", t: "naviguer" }, { k: "n", t: "ajouter" }, { k: "x", t: "retirer" }, { k: "E", t: "exporter CRL signée" }];
      case "servers":    return [{ k: "R/H/P", t: "sous-onglets" }, { k: "s", t: "start/stop", imp: true }, { k: "e", t: "config" }, { k: "g", t: "régénérer token" }, { k: "c", t: "clear log" }];
      case "audit":      return [{ k: "↑↓", t: "naviguer" }, { k: "d", t: "détail" }, { k: "f", t: "filtre kind" }, { k: "/", t: "rechercher target" }, { k: "E", t: "export CSV" }, { k: "J", t: "export JSON" }];
      case "settings":   return [{ k: "↑↓", t: "naviguer" }, { k: "Space", t: "toggle" }, { k: "P", t: "rekey passphrase" }];
      default: return [];
    }
  };

  // --- render -----------------------------------------------------------
  if (session === "locked") {
    return (
      <div style={{ position: "relative", height: "100vh", width: "100vw", overflow: "hidden", display: "flex", flexDirection: "column" }}>
        <OnboardingScreen mode="passphrase" setSession={setSession} openOverlay={openOverlay} />
        <OverlayHost overlay={overlay} closeOverlay={closeOverlay} openOverlay={openOverlay} />
        <TourPanel session={session} tourJump={tourJump} active={active} />
      </div>
    );
  }
  if (session === "onboarding") {
    return (
      <div style={{ position: "relative", height: "100vh", width: "100vw", overflow: "hidden", display: "flex", flexDirection: "column" }}>
        <OnboardingScreen mode="first-run" setSession={setSession} />
        <TourPanel session={session} tourJump={tourJump} active={active} />
      </div>
    );
  }

  let body = null;
  switch (active) {
    case "dashboard":  body = <DashboardScreen goto={setActive} />; break;
    case "licenses":   body = <LicensesScreen sel={licSel} setSel={setLicSel} search={licSearch} setSearch={setLicSearch} filters={licFilters} setFilters={setLicFilters} detailOpen={licDetailOpen} setDetailOpen={setLicDetailOpen} detailTab={licDetailTab} setDetailTab={setLicDetailTab} openOverlay={openOverlay} openWizard={() => { setWiz({ step: 0, done: {} }); setOverlay({ kind: "wizard" }); }} />; break;
    case "issuers":    body = <IssuersScreen sel={issSel} setSel={setIssSel} />; break;
    case "recipients": body = <RecipientsScreen sel={recSel} />; break;
    case "identities": body = <IdentitiesScreen sel={idSel} openOverlay={openOverlay} />; break;
    case "revocation": body = <RevocationScreen sel={revSel} />; break;
    case "servers":    body = <ServersScreen sub={srvSub} setSub={setSrvSub} probeView={probeView} setProbeView={setProbeView} openOverlay={openOverlay} />; break;
    case "audit":      body = <AuditScreen sel={audSel} setSel={setAudSel} filters={audFilters} setFilters={setAudFilters} />; break;
    case "settings":   body = <SettingsScreen openOverlay={openOverlay} />; break;
  }

  const breadcrumb = (() => {
    switch (active) {
      case "dashboard":  return ["dashboard"];
      case "licenses":   return ["licences", `liste (${window.DATA.licenses.length})`, window.DATA.licenses[licSel]?.subj];
      case "issuers":    return ["clés d'émission", "Ed25519", window.DATA.issuer_keys[issSel]?.keyid];
      case "recipients": return ["recipients", "X25519", window.DATA.recipient_keys[recSel]?.keyid];
      case "identities": return ["identities", window.DATA.identities[idSel]?.name];
      case "revocation": return ["révocation", `CRL (${window.DATA.revocations.length})`];
      case "servers":    return ["serveurs HTTP", SERVERS_SUBS.find(s => s.id === srvSub)?.label];
      case "audit":      return ["audit", `entries (${window.DATA.audit_long.length})`];
      case "settings":   return ["settings"];
      default: return [active];
    }
  })();

  const serverOnCount = window.DATA.servers.filter(s => s.on).length;

  return (
    <div style={{ position: "relative", height: "100vh", width: "100vw", overflow: "hidden", display: "flex", flexDirection: "column" }}>
      <Titlebar db="db.sqlite" server_on_count={serverOnCount} online />
      <TabStrip active={active} onSwitch={setActive} />
      <Crumb items={breadcrumb} />

      <div style={{ flex: 1, minHeight: 0, position: "relative", overflow: "hidden" }}>
        {body}
        {overlay?.kind === "wizard" && (
          <div style={{ position: "absolute", inset: 0, background: "var(--bg)" }}>
            <WizardScreen wiz={wiz} setWiz={setWiz} openOverlay={openOverlay} closeOverlay={closeOverlay} />
          </div>
        )}
        <OverlayHost overlay={overlay && overlay.kind !== "wizard" ? overlay : null} closeOverlay={closeOverlay} openOverlay={openOverlay} />
      </div>

      <StatusBar hints={hintsFor(active)} lastKey={lastKey} message={msg} />

      <TourPanel session={session} tourJump={tourJump} active={active} />
    </div>
  );
}

// --- TOUR PANEL ---------------------------------------------------------
// Hidden by default; toggle with `~`. Lets the reviewer jump to every state
// without having to chase the keyboard.

function TourPanel({ session, tourJump, active }) {
  const [open, setOpen] = useState(false);
  useEffect(() => {
    function onKey(e) { if (e.key === "~" || e.key === "²") { setOpen(o => !o); e.preventDefault(); } }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  if (!open) {
    return (
      <button onClick={() => setOpen(true)} style={{
        position: "fixed", bottom: 12, right: 12,
        background: "var(--bg-1)", border: "1px solid var(--magenta)",
        color: "var(--magenta)", padding: "4px 10px", fontFamily: "inherit",
        fontSize: 12, cursor: "pointer", zIndex: 100,
      }}>~ tour</button>
    );
  }

  const sections = [
    { t: "session", items: [
      ["onboarding",  "1er lancement (DB vide)"],
      ["locked",      "DB existante (passphrase)"],
      ["ready",       "app prête"],
    ]},
    { t: "tabs", items: TABS.map(t => [`tab:${t.id}`, `[${t.k}] ${t.label}`])},
    { t: "wizard licence", items: [
      ["wizard:0", "ét.1 — identité"],
      ["wizard:1", "ét.2 — validité"],
      ["wizard:2", "ét.3 — bindings"],
      ["wizard:3", "ét.4 — features"],
      ["wizard:4", "ét.5 — pinning"],
      ["wizard:5", "ét.6 — payload"],
      ["wizard:6", "ét.7 — sealed"],
      ["wizard:7", "ét.8 — récap"],
    ]},
    { t: "overlays", items: [
      ["probe-drawer", "fingerprint probe"],
      ["qr",           "QR TOTP"],
      ["filepicker",   "filepicker (binary SHA)"],
      ["revoke",       "modal révoquer"],
      ["quit",         "quitter (serveurs ON)"],
      ["help",         "aide ?"],
      ["ok-issued",    "ok : licence émise"],
    ]},
    { t: "erreurs", items: [
      ["error-port",   "port :8443 occupé"],
      ["error-pass",   "passphrase incorrecte"],
      ["error-token",  "token probe expiré"],
      ["error-locked", "DB locked (autre instance)"],
    ]},
  ];

  return (
    <div style={{
      position: "fixed", top: 12, right: 12, bottom: 12,
      width: 220, background: "var(--bg-1)",
      border: "1px solid var(--magenta)",
      display: "flex", flexDirection: "column",
      zIndex: 100, overflow: "hidden",
    }}>
      <div style={{ padding: "6px 10px", borderBottom: "1px solid var(--magenta)", display: "flex", alignItems: "center", gap: 6 }}>
        <span className="glow-magenta" style={{ fontWeight: 700, fontSize: 12 }}>◆ TOUR</span>
        <span className="dim" style={{ fontSize: 11 }}>(~ pour masquer)</span>
        <span style={{ flex: 1 }} />
        <button onClick={() => setOpen(false)} style={{
          background: "transparent", border: "none", color: "var(--fg-dim)",
          cursor: "pointer", fontSize: 14, padding: 0,
        }}>×</button>
      </div>
      <div style={{ flex: 1, overflow: "auto", padding: "4px 0" }}>
        {sections.map((sec, i) => (
          <div key={i} style={{ marginBottom: 6 }}>
            <div className="dim" style={{ padding: "4px 10px", fontSize: 10, textTransform: "uppercase", letterSpacing: 0.06, color: "var(--cyan)" }}>{sec.t}</div>
            {sec.items.map(([slug, label]) => (
              <div key={slug} onClick={() => tourJump(slug)} style={{
                padding: "3px 10px",
                cursor: "pointer",
                fontSize: 12,
                color: (slug === `tab:${active}`) ? "var(--magenta)" : "var(--fg-dim)",
              }}
              onMouseEnter={e => { e.currentTarget.style.background = "rgba(255,54,212,0.08)"; }}
              onMouseLeave={e => { e.currentTarget.style.background = "transparent"; }}
              >→ {label}</div>
            ))}
          </div>
        ))}
      </div>
      <div style={{ borderTop: "1px solid var(--border)", padding: "6px 10px", fontSize: 11, color: "var(--fg-mute)" }}>
        Le tour pilote l'IA. Le clavier reste actif partout.
      </div>
    </div>
  );
}

ReactDOM.createRoot(document.getElementById("root")).render(<App />);
