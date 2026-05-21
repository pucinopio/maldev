// Overlays : modals (centrés, dimmés via Faint), drawer droit, error/ok modals.
// Fidèles à lipgloss : pas de text-shadow, box-shadow, blur, gradient.

function OverlayHost({ overlay, closeOverlay, openOverlay }) {
  if (!overlay) return null;
  switch (overlay.kind) {
    case "confirm":         return <ConfirmModal o={overlay} close={closeOverlay} />;
    case "quit":            return <QuitModal o={overlay} close={closeOverlay} />;
    case "revoke":          return <RevokeModal o={overlay} close={closeOverlay} />;
    case "error":           return <ErrorModal o={overlay} close={closeOverlay} />;
    case "qr":              return <QRModal o={overlay} close={closeOverlay} />;
    case "probe_drawer":    return <ProbeDrawer o={overlay} close={closeOverlay} />;
    case "probe_regen":     return <ProbeRegenModal o={overlay} close={closeOverlay} />;
    case "probe_keep":      return <ProbeKeepModal o={overlay} close={closeOverlay} />;
    case "filepicker":      return <FilepickerModal o={overlay} close={closeOverlay} />;
    case "help":            return <HelpModal o={overlay} close={closeOverlay} />;
    case "ok":              return <OkModal o={overlay} close={closeOverlay} />;
    case "rekey":           return <RekeyModal o={overlay} close={closeOverlay} />;
    case "identity_blocked":return <IdentityBlockedModal o={overlay} close={closeOverlay} />;
    case "reissue_blocked": return <ReissueBlockedModal o={overlay} close={closeOverlay} />;
    default: return null;
  }
}

function Scrim({ children }) {
  return <div className="scrim">{children}</div>;
}

// --- CONFIRM / QUIT / REVOKE / ERROR / OK -------------------------------

function ConfirmModal({ o, close }) {
  return (
    <Scrim>
      <div className="modal" style={{ padding: 16 }}>
        <div className="b-magenta" style={{ marginBottom: 6 }}>{o.title || "Confirmer ?"}</div>
        <div className="dim" style={{ marginBottom: 14 }}>{o.body}</div>
        <div style={{ display: "flex", gap: 8, justifyContent: "flex-end" }}>
          <Btn label="Annuler" hot="esc" onClick={close} />
          <Btn label={o.confirmLabel || "OK"} hot="↵" kind={o.kind || "primary"} focused onClick={close} />
        </div>
      </div>
    </Scrim>
  );
}

function QuitModal({ close }) {
  const active = window.DATA.servers.filter(s => s.on);
  const confirmEnabled = window.DATA.settings.confirm_quit_with_servers;
  return (
    <Scrim>
      <div className="modal danger" style={{ padding: 16, width: 580 }}>
        <div className="b-red" style={{ marginBottom: 6 }}>Quitter license-manager ?</div>
        <div className="dim" style={{ marginBottom: 8 }}>
          {active.length > 0 ? `${active.length} serveur(s) HTTP actif(s) :` : "Aucun serveur HTTP actif."}
        </div>
        {active.map((s, i) => (
          <div key={i} className="row" style={{ padding: "3px 0", gap: 8 }}>
            <Dot kind="green" />
            <span>{s.name}</span>
            <span className="dim">:{s.port}</span>
            <span style={{ flex: 1 }} />
            <span className="dim">{s.reqs.toLocaleString("fr-FR")} req · up {s.uptime}</span>
          </div>
        ))}
        <Rule />
        <div className="dim" style={{ marginBottom: 12 }}>
          Quitter va arrêter {active.length} serveur(s) et fermer la base proprement.
          {!confirmEnabled && <span className="mute"> (cette confirmation peut être désactivée dans Settings)</span>}
        </div>
        <div style={{ display: "flex", gap: 8, justifyContent: "flex-end" }}>
          <Btn label="Annuler" hot="esc" onClick={close} />
          <Btn label="Arrêter & quitter" hot="↵" kind="danger" focused onClick={close} />
        </div>
      </div>
    </Scrim>
  );
}

function RevokeModal({ o, close }) {
  return (
    <Scrim>
      <div className="modal danger" style={{ padding: 16, width: 620 }}>
        <div className="b-red" style={{ marginBottom: 6 }}>Révoquer la licence ?</div>
        <KV k="lic_id"  v={<span className="c-magenta">{o.lic || "lic:9f3a-… (alice@research)"}</span>} />
        <KV k="keyid"   v={<span className="c-cyan">{o.keyid || "k2026-04"}</span>} />
        <Rule />
        <Input label="raison" value={o.reason || "key_compromised"} hint="freeform — visible dans la CRL" focused />
        <div className="dim" style={{ marginBottom: 12 }}>
          Suggestions : key_compromised, offboarding, leak, decommissioned, abuse
        </div>
        <div className="dim" style={{ marginBottom: 12 }}>
          {window.DATA.servers.find(s => s.id === "revoc")?.on
            ? <>Serveur révocation <span className="c-green">actif</span> — la CRL sera publiée au prochain GET /crl.</>
            : <>Serveur révocation <span className="mute">arrêté</span> — CRL offline jusqu'au démarrage.</>}
        </div>
        <div style={{ display: "flex", gap: 8, justifyContent: "flex-end" }}>
          <Btn label="Annuler" hot="esc" onClick={close} />
          <Btn label="Révoquer" hot="↵" kind="danger" focused onClick={close} />
        </div>
      </div>
    </Scrim>
  );
}

function ErrorModal({ o, close }) {
  return (
    <Scrim>
      <div className="modal danger" style={{ padding: 16, width: 620 }}>
        <div className="b-red" style={{ marginBottom: 6 }}>✗ {o.title || "Erreur"}</div>
        <div style={{ marginBottom: 10 }}>{o.body}</div>
        {o.details && (
          <div className="ascii" style={{ background: "var(--bg)", padding: 8, border: "1px solid var(--border)", color: "var(--red)", marginBottom: 10 }}>
            {o.details}
          </div>
        )}
        <div style={{ display: "flex", gap: 8, justifyContent: "flex-end" }}>
          {o.recover && <Btn label={o.recover} hot="r" kind="primary" onClick={close} />}
          <Btn label="Fermer" hot="↵" focused onClick={close} />
        </div>
      </div>
    </Scrim>
  );
}

function OkModal({ o, close }) {
  return (
    <Scrim>
      <div className="modal ok" style={{ padding: 16, width: 580 }}>
        <div className="b-green" style={{ marginBottom: 6 }}>✓ {o.title || "Succès"}</div>
        <div style={{ marginBottom: 12 }}>{o.body}</div>
        <div style={{ display: "flex", gap: 8, justifyContent: "flex-end" }}>
          <Btn label="OK" hot="↵" focused onClick={close} />
        </div>
      </div>
    </Scrim>
  );
}

// --- IDENTITY/LICENSE blocking modals -----------------------------------

function IdentityBlockedModal({ o, close }) {
  return (
    <Scrim>
      <div className="modal" style={{ padding: 16, width: 600, borderColor: "var(--yellow)" }}>
        <div className="b-yellow" style={{ marginBottom: 6 }}>⚠ Suppression refusée</div>
        <div style={{ marginBottom: 10 }}>
          <span className="b-cyan">{o.name || "rshell-windows-amd64.bin"}</span> est utilisée par{" "}
          <span className="b-magenta">{o.refs || 22}</span> licence(s).
        </div>
        <div className="dim" style={{ marginBottom: 14 }}>
          Pour la supprimer, révoque ou ignore d'abord les licences qui la référencent.
          Tu peux ouvrir la liste filtrée des licences pinnées sur cette identité.
        </div>
        <div style={{ display: "flex", gap: 8, justifyContent: "flex-end" }}>
          <Btn label="Voir les licences" hot="l" onClick={close} />
          <Btn label="Fermer" hot="↵" focused onClick={close} />
        </div>
      </div>
    </Scrim>
  );
}

function ReissueBlockedModal({ o, close }) {
  return (
    <Scrim>
      <div className="modal" style={{ padding: 16, width: 620, borderColor: "var(--violet)" }}>
        <div className="b-violet" style={{ marginBottom: 6 }}>⚠ Re-émission refusée</div>
        <div style={{ marginBottom: 10 }}>
          Cette licence est déjà <span className="b-violet">SUPERSEDED</span> par{" "}
          <span className="c-magenta">{o.successor || "lic-xyz"}</span>.
        </div>
        <div className="dim" style={{ marginBottom: 14 }}>
          Une licence superseded n'est pas re-émissible. Re-émets le successeur le plus récent à la place.
        </div>
        <div style={{ display: "flex", gap: 8, justifyContent: "flex-end" }}>
          <Btn label="Ouvrir le successeur" hot="↵" kind="primary" focused onClick={close} />
          <Btn label="Fermer" hot="esc" onClick={close} />
        </div>
      </div>
    </Scrim>
  );
}

// --- QR -----------------------------------------------------------------

function QRModal({ o, close }) {
  return (
    <Scrim>
      <div className="modal" style={{ padding: 16, width: 620 }}>
        <div className="b-magenta" style={{ marginBottom: 4 }}>{o.title || "QR — TOTP"}</div>
        <div className="dim" style={{ marginBottom: 12 }}>{o.subtitle || "otpauth://totp/rshell:alice?secret=JBSWY3DPEHPK3PXP&issuer=offsec"}</div>
        <div style={{ display: "grid", gridTemplateColumns: "auto 1fr", gap: 18, alignItems: "start" }}>
          <div style={{ background: "var(--bg)", padding: 8, border: "1px solid var(--border)" }}>
            <AsciiQR size={25} />
          </div>
          <div>
            <SecHead>Secret</SecHead>
            <div style={{ marginBottom: 10 }}><span className="c-cyan">JBSWY3DPEHPK3PXP</span></div>
            <SecHead>Paramètres</SecHead>
            <KV k="alg"    v="SHA1" />
            <KV k="digits" v="6" />
            <KV k="period" v="30 s" />
            <KV k="issuer" v="offsec" />
            <KV k="label"  v="rshell:alice" />
            <Rule />
            <div style={{ display: "flex", gap: 6, flexWrap: "wrap" }}>
              <Btn label="Copier secret" hot="c" />
              <Btn label="Export PNG…" hot="P" kind="primary" />
              <Btn label="Print" hot="p" />
              <Btn label="Fermer" hot="esc" onClick={close} />
            </div>
          </div>
        </div>
      </div>
    </Scrim>
  );
}

// --- FILEPICKER ---------------------------------------------------------

function FilepickerModal({ o, close }) {
  const cwd = o.cwd || "/work/build";
  const items = [
    { n: "..",          k: "dir" },
    { n: "rshell.exe",  k: "exe", sz: "7.0 MB", mt: "2026-05-19 18:21" },
    { n: "rshell.elf",  k: "exe", sz: "6.8 MB", mt: "2026-05-19 18:21" },
    { n: "rshell.dmg",  k: "exe", sz: "9.4 MB", mt: "2026-05-19 18:22" },
    { n: "checksums.txt", k: "txt", sz: "412 B", mt: "2026-05-19 18:22" },
    { n: "old/",        k: "dir" },
    { n: ".gitignore",  k: "hidden", sz: "211 B", mt: "2026-05-12 11:09" },
  ];
  return (
    <Scrim>
      <div className="modal" style={{ width: 760, height: 480, display: "flex", flexDirection: "column" }}>
        <div style={{ padding: "8px 12px", borderBottom: "1px solid var(--border)", display: "flex", gap: 14, alignItems: "center" }}>
          <span className="c-cyan">filepicker</span>
          <span className="dim">cwd</span><span style={{ color: "var(--fg)" }}>{cwd}</span>
          <span style={{ flex: 1 }} />
          <HK k="↑↓">nav</HK><HK k="↵">choisir</HK><HK k="h">cachés</HK><HK k="esc">annuler</HK>
        </div>
        <div style={{ flex: 1, overflow: "auto" }}>
          {items.map((it, i) => (
            <div key={i} className={"row" + (i === 1 ? " selected" : "")} style={{ padding: "3px 12px", gap: 14 }}>
              <span style={{ width: "3ch", color: it.k === "dir" ? "var(--cyan)" : it.k === "exe" ? "var(--magenta)" : "var(--fg-dim)", fontWeight: 700 }}>
                {it.k === "dir" ? "▸" : it.k === "exe" ? "●" : "·"}
              </span>
              <span style={{ flex: 1 }}>{it.n}</span>
              <span className="dim" style={{ width: "10ch", textAlign: "right" }}>{it.sz || ""}</span>
              <span className="dim" style={{ width: "20ch", textAlign: "right" }}>{it.mt || ""}</span>
            </div>
          ))}
        </div>
        <div style={{ padding: "6px 12px", borderTop: "1px solid var(--border)", display: "flex", alignItems: "center", gap: 14 }}>
          <span className="dim">sélection :</span><span className="c-magenta">/work/build/rshell.exe</span>
          <span style={{ flex: 1 }} />
          <Btn label="Annuler" hot="esc" onClick={close} />
          <Btn label="Choisir" hot="↵" kind="primary" focused onClick={close} />
        </div>
      </div>
    </Scrim>
  );
}

// --- REKEY (change passphrase) ------------------------------------------

function RekeyModal({ o, close }) {
  return (
    <Scrim>
      <div className="modal" style={{ padding: 16, width: 620 }}>
        <div className="b-magenta" style={{ marginBottom: 6 }}>Changer la passphrase de la DB</div>
        <div className="dim" style={{ marginBottom: 12 }}>
          Rekey complet en transaction : ré-encrypte tous les blobs chiffrés. La nouvelle passphrase remplace l'actuelle.
        </div>
        <Input label="passphrase actuelle" value="••••••••••••••" masked focused />
        <Input label="nouvelle passphrase" value="••••••••••••••••••••" masked />
        <Input label="confirmation"        value="••••••••••••••••••••" masked />
        <div className="dim" style={{ marginBottom: 12 }}>
          force : <span className="c-green">forte</span> · entropie ≈ 104 bits
        </div>
        <div style={{ display: "flex", gap: 8, justifyContent: "flex-end" }}>
          <Btn label="Annuler" hot="esc" onClick={close} />
          <Btn label="Rekey" hot="↵" kind="primary" focused onClick={close} />
        </div>
      </div>
    </Scrim>
  );
}

// --- PROBE : drawer, regen token, keep server ---------------------------

function ProbeDrawer({ o, close }) {
  const [phase, setPhase] = React.useState(o.phase || "waiting");
  const [snipTab, setSnipTab] = React.useState("bash");
  React.useEffect(() => {
    if (phase === "waiting") {
      const t = setTimeout(() => setPhase("received"), 8000);
      return () => clearTimeout(t);
    }
  }, [phase]);

  return (
    <div className="drawer-wrap">
      <div className="scrim-mute" onClick={close} />
      <div className="drawer" style={{ display: "flex", flexDirection: "column" }}>
        <div style={{ padding: "8px 14px", borderBottom: "1px solid var(--cyan)", display: "flex", alignItems: "baseline", gap: 12 }}>
          <span className="b-cyan">◆ FINGERPRINT PROBE</span>
          <Dot kind={phase === "received" ? "green" : "yellow"} />
          <span className="dim">{phase === "waiting" ? "subscribe channel · en attente" : "fingerprint reçu"}</span>
          <span style={{ flex: 1 }} />
          <span className="dim"><HK k="esc">fermer</HK></span>
        </div>

        <div style={{ padding: 16, overflow: "auto", flex: 1 }}>
          {phase === "waiting" && <ProbeWaiting snipTab={snipTab} setSnipTab={setSnipTab} onSimulate={() => setPhase("received")} />}
          {phase === "received" && <ProbeReceived close={close} />}
        </div>
      </div>
    </div>
  );
}

function ProbeWaiting({ snipTab, setSnipTab, onSimulate }) {
  return (
    <div>
      <Box title="URL à donner au client distant" right={<span className="dim">[c] copier</span>}>
        <div style={{ padding: 12 }}>
          <div className="c-cyan" style={{ wordBreak: "break-all" }}>
            https://manager.local:8445/probe/tk_aB3xZ9mLqP21vR
          </div>
          <Rule />
          <div className="dim" style={{ marginBottom: 4 }}>token one-shot (expire dans 47s · label "alice-laptop")</div>
          <div className="b-magenta">tk_aB3xZ9mLqP21vR</div>
        </div>
      </Box>

      <div style={{ marginTop: 12 }}>
        <Box title="Snippets copy-paste" right={
          <span className="dim">
            <HK k="1">bash</HK><HK k="2">PowerShell</HK><HK k="3">QR ASCII</HK>
          </span>
        }>
          {/* sub-tabs */}
          <div style={{ display: "flex", borderBottom: "1px solid var(--border)" }}>
            {[
              { id: "bash", k: "1", l: "Linux / macOS (bash)" },
              { id: "ps",   k: "2", l: "Windows (PowerShell)" },
              { id: "qr",   k: "3", l: "QR ASCII (téléphone)" },
            ].map(t => (
              <div key={t.id} onClick={() => setSnipTab(t.id)} style={{
                padding: "4px 12px",
                cursor: "pointer",
                color: snipTab === t.id ? "var(--fg)" : "var(--fg-dim)",
                background: snipTab === t.id ? "var(--bg-2)" : "transparent",
                boxShadow: snipTab === t.id ? "inset 0 -2px 0 var(--cyan)" : "none",
                display: "flex", gap: 6,
              }}>
                <span style={{ color: snipTab === t.id ? "var(--cyan)" : "var(--fg-mute)", fontWeight: 700 }}>[{t.k}]</span>
                <span>{t.l}</span>
              </div>
            ))}
          </div>
          <div style={{ padding: 12 }}>
            {snipTab === "bash" && (
              <pre style={{ margin: 0, fontFamily: "inherit", color: "var(--green)", whiteSpace: "pre-wrap" }}>
{`curl -fsSL https://manager.local:8445/probe/tk_aB3xZ9mLqP21vR/agent | sh`}
              </pre>
            )}
            {snipTab === "ps" && (
              <pre style={{ margin: 0, fontFamily: "inherit", color: "var(--green)", whiteSpace: "pre-wrap" }}>
{`iwr https://manager.local:8445/probe/tk_aB3xZ9mLqP21vR/agent.exe -OutFile $env:TEMP\\a.exe
& "$env:TEMP\\a.exe"`}
              </pre>
            )}
            {snipTab === "qr" && (
              <div style={{ display: "flex", gap: 16, alignItems: "center" }}>
                <div style={{ background: "var(--bg)", padding: 6, border: "1px solid var(--border)" }}><AsciiQR size={19} /></div>
                <div className="dim" style={{ flex: 1 }}>
                  QR du curl bash entier (URL + token). Scan depuis un téléphone à côté de la machine cliente, paste dans son terminal.
                  <div style={{ marginTop: 6 }}><Btn label="Export PNG…" hot="P" /></div>
                </div>
              </div>
            )}
          </div>
        </Box>
      </div>

      <Rule />
      <div style={{ display: "flex", alignItems: "center", gap: 12, padding: "8px 4px" }}>
        <span className="spinner" /><span className="dim">Subscribe(token) actif · en attente du POST sur /probe/&lt;token&gt;…</span>
        <span style={{ flex: 1 }} />
        <span className="dim">one-shot · stop auto · TTL 1 min</span>
      </div>

      <div style={{ marginTop: 12, textAlign: "right" }}>
        <Btn label="(demo) simuler réception" hot="↵" kind="primary" onClick={onSimulate} />
      </div>
    </div>
  );
}

function ProbeReceived({ close }) {
  return (
    <div>
      <Box title="✓ Fingerprint reçu" focused>
        <div style={{ padding: 12 }}>
          <KV k="from IP"   v="78.92.1.8 (TLS pinned · admin token OK)" />
          <KV k="received"  v="13:43:02 (il y a 2s, via channel)" />
          <KV k="label"     v="alice-laptop" />
          <KV k="hostname"  v={<span style={{ color: "var(--fg)" }}>laptop-alice</span>} />
          <KV k="OS / arch" v="linux / amd64" />
          <KV k="cpu"       v="AMD Ryzen 7 PRO 7840U" />
          <Rule />
          <KV k="hostid.Local()"     v={<span className="c-cyan">0c8a91d3f4a2…f4d2</span>} />
          <KV k="hostid.Composite()" v={<span className="c-cyan">7c91aa3b1208…1208</span>} />
        </div>
      </Box>

      <Rule />
      <SecHead>Ajouter au binding « machine »</SecHead>
      <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
        <Btn label="ajouter Local"        hot="L" onClick={close} />
        <Btn label="ajouter Composite"    hot="C" onClick={close} />
        <Btn label="ajouter les deux"     hot="↵" kind="primary" focused onClick={close} />
        <Btn label="ignorer"              hot="esc" onClick={close} />
      </div>
      <div className="dim" style={{ marginTop: 10 }}>
        Après ajout, le token devient consommé. Modal demandera si tu veux garder le ProbeServer ON pour d'autres tokens.
      </div>
    </div>
  );
}

function ProbeRegenModal({ o, close }) {
  return (
    <Scrim>
      <div className="modal" style={{ padding: 16, width: 620, borderColor: "var(--yellow)" }}>
        <div className="b-yellow" style={{ marginBottom: 6 }}>Régénérer l'admin token ?</div>
        <div className="dim" style={{ marginBottom: 10 }}>
          La régénération est <span className="b-red">destructrice</span> et procède en 3 étapes :
        </div>
        <div style={{ marginLeft: 14, marginBottom: 12 }}>
          <div><span className="c-yellow">1.</span> arrêt du serveur <span className="b-cyan">revocation :8443</span></div>
          <div><span className="c-yellow">2.</span> génération d'un nouveau token (affiché une fois après)</div>
          <div><span className="c-yellow">3.</span> redémarrage automatique sur le même port</div>
        </div>
        <div className="dim" style={{ marginBottom: 12 }}>
          Pendant ~2-3s le serveur sera indisponible. Tout admin client utilisant l'ancien token devra être mis à jour.
        </div>
        <div style={{ display: "flex", gap: 8, justifyContent: "flex-end" }}>
          <Btn label="Annuler" hot="esc" onClick={close} />
          <Btn label="Stop + Regen + Restart" hot="↵" kind="primary" focused onClick={close} />
        </div>
      </div>
    </Scrim>
  );
}

function ProbeKeepModal({ o, close }) {
  return (
    <Scrim>
      <div className="modal" style={{ padding: 16, width: 580 }}>
        <div className="b-magenta" style={{ marginBottom: 6 }}>Garder le ProbeServer actif ?</div>
        <div className="dim" style={{ marginBottom: 14 }}>
          Le token <span className="c-cyan">tk_aB3xZ9mLqP21vR</span> a été consommé. Tu peux laisser le serveur <span className="c-cyan">probe :8445</span> tourner pour générer d'autres tokens (utile pour un batch §10), ou l'arrêter maintenant.
        </div>
        <div style={{ display: "flex", gap: 8, justifyContent: "flex-end" }}>
          <Btn label="Arrêter" hot="s" onClick={close} />
          <Btn label="Garder ON" hot="↵" kind="primary" focused onClick={close} />
        </div>
      </div>
    </Scrim>
  );
}

// --- HELP ---------------------------------------------------------------

function HelpModal({ o, close }) {
  return (
    <Scrim>
      <div className="modal" style={{ padding: 16, width: 760 }}>
        <div className="b-magenta" style={{ marginBottom: 10 }}>? Aide — touches</div>
        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 22 }}>
          <div>
            <SecHead>Universel</SecHead>
            <HelpRow k="1-9"      l="changer d'onglet" />
            <HelpRow k="esc"      l="retour / fermer overlay" />
            <HelpRow k="?"        l="cette aide" />
            <HelpRow k="q"        l="quitter (confirme si serveur ON)" />
            <HelpRow k="/"        l="rechercher dans la vue courante" />
            <HelpRow k="r"        l="rafraîchir" />
            <SecHead style={{ marginTop: 10 }}>Listes</SecHead>
            <HelpRow k="↑ ↓"      l="naviguer" />
            <HelpRow k="d"        l="détail (split-pane sous la table)" />
            <HelpRow k="n"        l="nouveau" />
            <HelpRow k="e"        l="éditer / re-émettre" />
            <HelpRow k="x"        l="supprimer / révoquer (confirme)" />
            <HelpRow k="f"        l="cycler filtre status" />
          </div>
          <div>
            <SecHead>Formulaires & wizard</SecHead>
            <HelpRow k="Tab"      l="champ suivant" />
            <HelpRow k="⇧Tab"     l="champ précédent" />
            <HelpRow k="↵"        l="valider" />
            <HelpRow k="ctrl+c"   l="annuler opération" />
            <HelpRow k="1-8"      l="aller à étape (wizard)" />
            <SecHead style={{ marginTop: 10 }}>Serveurs / Probe</SecHead>
            <HelpRow k="s"        l="start / stop serveur" />
            <HelpRow k="g"        l="régénérer admin token (confirm)" />
            <HelpRow k="R H P"    l="sous-onglets revocation / heartbeat / probe" />
            <HelpRow k="T H L"    l="probe : Tokens / History / Live log" />
            <SecHead style={{ marginTop: 10 }}>Détail licence</SecHead>
            <HelpRow k="I B P A C" l="Identité · Bindings · PEM · Audit · Chaîne" />
          </div>
        </div>
        <Rule />
        <div style={{ textAlign: "right" }}>
          <Btn label="Fermer" hot="esc" focused onClick={close} />
        </div>
      </div>
    </Scrim>
  );
}
function HelpRow({ k, l }) {
  return (
    <div style={{ display: "flex", gap: 12, padding: "1px 0" }}>
      <span className="b-magenta" style={{ width: "10ch" }}>{k}</span>
      <span className="dim" style={{ flex: 1 }}>{l}</span>
    </div>
  );
}

// Re-use the SecHead / KV from licenses (assumes loaded after licenses)
function SecHead({ children, style }) {
  return <div className="c-cyan" style={{ fontWeight: 700, marginBottom: 4, ...(style || {}) }}>{children}</div>;
}
function KV({ k, v }) {
  return (
    <div style={{ display: "flex", gap: 8 }}>
      <span className="dim" style={{ width: "10ch" }}>{k}</span>
      <span style={{ flex: 1, color: "var(--fg)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{v}</span>
    </div>
  );
}

Object.assign(window, { OverlayHost });
