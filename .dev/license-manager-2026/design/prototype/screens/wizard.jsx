// Wizard "nouvelle licence" — 8 étapes, sidebar + step progress

const WIZARD_STEPS = [
  { n: 1, key: "identity", label: "Identité",       hint: "subject, issuer, audience, KeyID" },
  { n: 2, key: "validity", label: "Validité",       hint: "NotBefore · NotAfter" },
  { n: 3, key: "bindings", label: "Bindings",       hint: "machine · password · TOTP · k/v" },
  { n: 4, key: "features", label: "Features",       hint: "tags fonctionnels" },
  { n: 5, key: "pinning",  label: "Pinning",        hint: "identity.bin · binary SHA256" },
  { n: 6, key: "payload",  label: "Payload",        hint: "claims JSON libres" },
  { n: 7, key: "sealed",   label: "Sealed payload", hint: "optionnel — chiffré pour recipient" },
  { n: 8, key: "recap",    label: "Récap & émettre", hint: "decode preview + signature" },
];
window.WIZARD_STEPS = WIZARD_STEPS;

function WizardScreen({ wiz, setWiz, openOverlay, closeOverlay }) {
  const step = WIZARD_STEPS[wiz.step];

  const goto = (n) => setWiz({ ...wiz, step: Math.max(0, Math.min(WIZARD_STEPS.length - 1, n)) });

  return (
    <div style={{ display: "flex", flexDirection: "column", height: "100%", overflow: "hidden" }}>
      {/* Progress strip */}
      <div style={{
        padding: "8px 14px 5px",
        borderBottom: "1px solid var(--border)",
        background: "var(--bg-1)",
      }}>
        <div style={{ display: "flex", alignItems: "baseline", gap: 14, marginBottom: 4 }}>
          <span className="b-magenta">NOUVELLE LICENCE</span>
          <span className="dim">étape</span>
          <span style={{ color: "var(--fg)", fontWeight: 700 }}>{step.n}/{WIZARD_STEPS.length}</span>
          <span className="dim">·</span>
          <span className="c-cyan">{step.label}</span>
          <span style={{ flex: 1 }} />
          <span className="dim">
            <HK k="Tab">suivant</HK><HK k="⇧Tab">précédent</HK><HK k="1-8">aller à</HK><HK k="esc">annuler</HK>
          </span>
        </div>
        <div className="progress-track">
          <div className="progress-bar" style={{ width: `${(step.n / WIZARD_STEPS.length) * 100}%` }} />
        </div>
      </div>

      {/* body: sidebar + step content */}
      <div style={{ flex: 1, display: "grid", gridTemplateColumns: "260px 1fr", minHeight: 0 }}>
        {/* sidebar */}
        <div style={{ borderRight: "1px solid var(--border)", background: "var(--bg-1)", padding: "6px 0" }}>
          {WIZARD_STEPS.map((s, i) => (
            <div
              key={s.key}
              onClick={() => goto(i)}
              style={{
                padding: "5px 12px",
                cursor: "pointer",
                borderLeft: wiz.step === i ? "2px solid var(--magenta)" : "2px solid transparent",
                background: wiz.step === i ? "var(--bg-2)" : "transparent",
              }}
            >
              <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
                <span style={{
                  minWidth: 20, height: 20, display: "inline-flex", alignItems: "center", justifyContent: "center",
                  border: `1px solid ${wiz.step === i ? "var(--magenta)" : wiz.done?.[s.key] ? "var(--green)" : "var(--border)"}`,
                  color: wiz.step === i ? "var(--magenta)" : wiz.done?.[s.key] ? "var(--green)" : "var(--fg-mute)",
                  fontWeight: 700,
                }}>
                  {wiz.done?.[s.key] && wiz.step !== i ? "✓" : s.n}
                </span>
                <span style={{ color: wiz.step === i ? "var(--fg)" : "var(--fg-dim)", fontWeight: wiz.step === i ? 700 : 400 }}>{s.label}</span>
              </div>
              <div className="mute" style={{ marginLeft: 30 }}>{s.hint}</div>
            </div>
          ))}
        </div>

        {/* step content */}
        <div style={{ padding: 20, overflow: "auto", minHeight: 0 }}>
          {wiz.step === 0 && <StepIdentity wiz={wiz} setWiz={setWiz} />}
          {wiz.step === 1 && <StepValidity wiz={wiz} setWiz={setWiz} />}
          {wiz.step === 2 && <StepBindings wiz={wiz} setWiz={setWiz} openOverlay={openOverlay} />}
          {wiz.step === 3 && <StepFeatures wiz={wiz} setWiz={setWiz} />}
          {wiz.step === 4 && <StepPinning wiz={wiz} setWiz={setWiz} />}
          {wiz.step === 5 && <StepPayload wiz={wiz} setWiz={setWiz} />}
          {wiz.step === 6 && <StepSealed wiz={wiz} setWiz={setWiz} />}
          {wiz.step === 7 && <StepRecap wiz={wiz} setWiz={setWiz} openOverlay={openOverlay} />}
        </div>
      </div>
    </div>
  );
}

// --- STEP 1 : IDENTITÉ ----------------------------------------------------
function StepIdentity({ wiz, setWiz }) {
  return (
    <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 30, maxWidth: 900 }}>
      <div>
        <Input label="subject" value={wiz.subject || "alice@research"} hint="email ou identifiant" focused />
        <div className="dim" style={{ fontSize: 12, marginTop: -6, marginBottom: 10 }}>
          autocomplete: <span style={{ color: "var(--cyan)" }}>alice@research</span>, alex@partner, bob@research
        </div>
        <Input label="issuer" value={wiz.issuer || "research@offsec.local"} hint="défaut: Settings → Issuer" />
        <Input label="audience" value={wiz.audience || "rshell, rshell-edu"} hint="multi-select (séparé par ,)" suffix="+2 saved" />
      </div>
      <div>
        <Input label="KeyID" value={wiz.keyid || "k2026-04"} hint="défaut: clé active" suffix="↓ pour choisir" />
        <Box title="Clé sélectionnée" style={{ marginTop: 4 }}>
          <div style={{ padding: 12 }}>
            <div className="b-magenta">k2026-04</div>
            <div className="dim" style={{ marginTop: 4 }}>rshell-prod-2026Q2</div>
            <div className="dim" style={{ marginTop: 2 }}>fpr ed25519:a4f2…91bc</div>
            <div className="dim" style={{ marginTop: 2 }}><StatusPill status="active" /> · 47 licences signées</div>
          </div>
        </Box>
      </div>
    </div>
  );
}

// --- STEP 2 : VALIDITÉ ----------------------------------------------------
function StepValidity({ wiz, setWiz }) {
  return (
    <div style={{ maxWidth: 700 }}>
      <Input label="NotBefore" value={wiz.nbf || "2026-05-20 13:42:18 UTC (maintenant)"} hint="ISO 8601 ou « now »" />
      <Input label="NotAfter" value={wiz.exp || "90d"} hint="durée relative (« 30d », « 6mo », « 1y ») ou date absolue" focused />
      <div style={{ marginTop: 6, fontSize: 13 }}>
        <span className="dim">→ résolu : </span>
        <span className="glow-cyan" style={{ fontWeight: 600 }}>2026-08-18 13:42:18 UTC</span>
        <span className="dim"> (dans 90 jours)</span>
      </div>
      <Rule />
      <div style={{ display: "flex", gap: 6 }}>
        <span className="chip"><span className="k">1</span>30d</span>
        <span className="chip"><span className="k">2</span>90d (settings default)</span>
        <span className="chip"><span className="k">3</span>6mo</span>
        <span className="chip"><span className="k">4</span>1y</span>
        <span className="chip"><span className="k">5</span>5y</span>
      </div>
    </div>
  );
}

// --- STEP 3 : BINDINGS ----------------------------------------------------
function StepBindings({ wiz, setWiz, openOverlay }) {
  const bindings = wiz.bindings || [
    { kind: "machine", id: "m1", ids: ["0c8a91…f4d2 (local)", "7c91aa…1208 (composite)"], note: "ma machine actuelle · 20 mai 13:40" },
  ];
  return (
    <div>
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "baseline", marginBottom: 8 }}>
        <SecHead>Bindings déclarés ({bindings.length})</SecHead>
        <span className="dim" style={{ fontSize: 12 }}>
          <HK k="m">+ machine</HK><HK k="p">+ password</HK><HK k="t">+ TOTP</HK><HK k="k">+ k/v</HK>
        </span>
      </div>

      <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
        {bindings.map((b, i) => (
          <BindingCard key={i} binding={b} idx={i} openOverlay={openOverlay} />
        ))}
        <div className="box" style={{ padding: "16px 14px", borderStyle: "dashed", color: "var(--fg-dim)", textAlign: "center" }}>
          <span className="dim">empty — ajouter un binding avec [m] [p] [t] [k]</span>
        </div>
      </div>

      <Rule />
      <div className="dim" style={{ fontSize: 12 }}>
        <span style={{ color: "var(--violet)" }}>◆</span> Les bindings sont en <span style={{ color: "var(--fg)" }}>AND</span>{" "}entre eux,{" "}
        en <span style={{ color: "var(--fg)" }}>OR</span> entre les valeurs d'un même binding. Empile autant que tu veux.
      </div>
    </div>
  );
}

function BindingCard({ binding, idx, openOverlay }) {
  const kindColor = { machine: "var(--cyan)", password: "var(--violet)", totp: "var(--magenta)", kv: "var(--yellow)" }[binding.kind] || "var(--fg)";
  return (
    <div className="box" style={{ borderColor: kindColor }}>
      <div style={{ padding: "6px 10px", display: "flex", alignItems: "center", gap: 10, borderBottom: "1px dashed var(--border)" }}>
        <span style={{ color: kindColor, fontWeight: 700 }}>◆ {binding.kind.toUpperCase()}</span>
        <span className="dim">#{idx + 1}</span>
        <span style={{ flex: 1 }} />
        <span className="dim" style={{ fontSize: 12 }}>
          <HK k="e">éditer</HK><HK k="x">retirer</HK>
        </span>
      </div>
      {binding.kind === "machine" && (
        <div style={{ padding: "8px 12px" }}>
          <div className="dim" style={{ fontSize: 12, marginBottom: 4 }}>IDs acceptés (OR) :</div>
          {binding.ids.map((id, i) => (
            <div key={i} style={{ marginLeft: 14 }}><span className="glow-cyan">{id}</span></div>
          ))}
          <div style={{ marginTop: 8, display: "flex", gap: 8, fontSize: 12 }}>
            <span className="chip"><span className="k">l</span>ma machine actuelle</span>
            <span className="chip" style={{ borderColor: "var(--cyan)", color: "var(--cyan)" }} onClick={() => openOverlay({ kind: "probe_drawer" })}>
              <span className="k">r</span>récupérer depuis machine distante…
            </span>
          </div>
        </div>
      )}
    </div>
  );
}

// --- STEP 4 : FEATURES ----------------------------------------------------
function StepFeatures({ wiz }) {
  const sel = ["scan", "report"];
  const all = ["scan", "report", "exec", "beacon", "uplink", "decrypt", "kerberos", "lateral"];
  return (
    <div style={{ maxWidth: 700 }}>
      <Input label="features" value={sel.join(", ")} hint="multi · autocomplete sur l'historique" focused />
      <Rule />
      <SecHead>Suggestions (DB)</SecHead>
      <div style={{ display: "flex", gap: 6, flexWrap: "wrap", marginTop: 4 }}>
        {all.map(f => (
          <span key={f} className="chip" style={{
            borderColor: sel.includes(f) ? "var(--magenta)" : "var(--border)",
            color: sel.includes(f) ? "var(--magenta)" : "var(--fg-dim)",
          }}>{sel.includes(f) ? "✓ " : ""}{f}</span>
        ))}
      </div>
    </div>
  );
}

// --- STEP 5 : PINNING ----------------------------------------------------
function StepPinning({ wiz }) {
  return (
    <div style={{ maxWidth: 900, display: "grid", gridTemplateColumns: "1fr 1fr", gap: 30 }}>
      <div>
        <SecHead>IdentitySHA256</SecHead>
        <Input label="identity.bin" value="rshell-linux-amd64.bin" hint="↓ pour choisir · « + » pour créer" suffix="↓+" />
        <div style={{ marginTop: -2, marginBottom: 8 }}><KV k="sha256" v={<span className="glow-cyan">01ffa2d8…7c4</span>} /></div>
        <Box title="identities disponibles" style={{ maxHeight: 160, overflow: "auto" }}>
          {window.DATA.identities.map((id, i) => (
            <div key={i} className="row" style={{ padding: "4px 12px", borderTop: i === 0 ? "none" : "1px dashed var(--border)" }}>
              <span style={{ flex: 1 }}>{id.name}</span>
              <span className="dim">{id.sha}</span>
            </div>
          ))}
        </Box>
      </div>
      <div>
        <SecHead>BinarySHA256</SecHead>
        <Input label="chemin du binaire" value="/work/build/rshell.exe" hint="Entrée pour ouvrir le filepicker" suffix="📁" focused />
        <div className="dim" style={{ fontSize: 12, marginBottom: 4 }}>hashing…</div>
        <div className="progress-track" style={{ marginBottom: 8 }}><div className="progress-bar" style={{ width: "62%" }} /></div>
        <div className="dim" style={{ fontSize: 12 }}>4.3 MB / 7.0 MB · 12 ms</div>
        <Rule />
        <KV k="sha256" v={<span className="glow-cyan">8b3c91ad…2e1 <span className="mute">(en cours)</span></span>} />
        <Input label="ou coller un hash manuellement" value="" hint="64 chars hex" />
      </div>
    </div>
  );
}

// --- STEP 6 : PAYLOAD ----------------------------------------------------
function StepPayload({ wiz }) {
  return (
    <div>
      <div style={{ display: "flex", gap: 6, marginBottom: 10 }}>
        <span className="chip" style={{ borderColor: "var(--magenta)", color: "var(--magenta)" }}><span className="k">1</span>vide {`{}`}</span>
        <span className="chip"><span className="k">2</span>JSON inline</span>
        <span className="chip"><span className="k">3</span>importer fichier…</span>
      </div>
      <Box title="payload (JSON)" focused>
        <div style={{ padding: 12, fontFamily: "inherit", minHeight: 200, color: "var(--green)" }}>
{`{`}<br />
<span style={{ marginLeft: 16 }}>
  <span style={{ color: "var(--cyan)" }}>"tier"</span>: <span style={{ color: "var(--yellow)" }}>"pro"</span>,
</span><br />
<span style={{ marginLeft: 16 }}>
  <span style={{ color: "var(--cyan)" }}>"seats"</span>: <span style={{ color: "var(--magenta)" }}>3</span>,
</span><br />
<span style={{ marginLeft: 16 }}>
  <span style={{ color: "var(--cyan)" }}>"note"</span>: <span style={{ color: "var(--yellow)" }}>"trial pour partner"</span>
</span><br />
{`}`}<span className="caret">&nbsp;</span>
        </div>
      </Box>
    </div>
  );
}

// --- STEP 7 : SEALED PAYLOAD ----------------------------------------------
function StepSealed({ wiz }) {
  return (
    <div style={{ maxWidth: 700 }}>
      <div className="dim" style={{ fontSize: 12, marginBottom: 12 }}>
        Optionnel. Chiffre un secret pour qu'il ne soit lisible qu'avec la clé X25519 du recipient choisi.
      </div>
      <Input label="recipient key" value="r2026-01 — default-recipient" hint="↓ pour choisir · « none » pour sauter" />
      <Input label="secret en clair" value="ZWY4YjRkNmFjMTk5ZGUwNQ==" hint="masqué" masked focused />
      <Rule />
      <div className="dim" style={{ fontSize: 12 }}>
        Le payload sera encapsulé en NaCl box (x25519+xsalsa20+poly1305) avec recipient = <span className="glow-cyan">x25519:7a90…003c</span>.
      </div>
    </div>
  );
}

// --- STEP 8 : RÉCAP -------------------------------------------------------
function StepRecap({ wiz, openOverlay }) {
  return (
    <div style={{ display: "grid", gridTemplateColumns: "1.1fr 1fr", gap: 20 }}>
      <div>
        <SecHead>Récapitulatif</SecHead>
        <KV k="subject"   v="alice@research" />
        <KV k="issuer"    v="research@offsec.local" />
        <KV k="audience"  v="rshell, rshell-edu" />
        <KV k="keyid"     v={<span className="glow-cyan">k2026-04</span>} />
        <KV k="not_before" v="2026-05-20 13:42:18 UTC" />
        <KV k="not_after"  v="2026-08-18 13:42:18 UTC (90d)" />
        <KV k="features"   v="scan, report" />
        <KV k="bindings"   v="machine (2 IDs)" />
        <KV k="identity"   v="rshell-linux-amd64.bin (01ffa2d8…7c4)" />
        <KV k="binary"     v="rshell.exe (8b3c91ad…2e1)" />
        <KV k="payload"    v="{ tier:pro, seats:3, note:… }" />
        <KV k="sealed"     v="r2026-01 (16 bytes)" />
        <Rule />
        <div style={{ display: "flex", gap: 8 }}>
          <Btn label="Émettre" hot="↵" kind="primary" focused />
          <Btn label="Retour" hot="esc" kind="" />
        </div>
      </div>
      <div>
        <SecHead>Aperçu PEM (après signature)</SecHead>
        <div className="ascii" style={{ background: "var(--bg)", padding: 8, border: "1px solid var(--border)", color: "var(--green)", height: 280, overflow: "hidden" }}>
{`-----BEGIN LICENSE-----
MIIBlTCCATsCFFq1jD3K8mWeP2pT0xN9rVbXq2sVMA0GCS
qGSIb3DQEBCwUAMBkxFzAVBgNVBAMMDnJzaGVsbC1pc3N1
ZXIwHhcNMjYwNTIwMTM0MjE4WhcNMjYwODE0MDAwMDAwWj
AfMR0wGwYDVQQDDBRyZXNlYXJjaEBvZmZzZWMubG9jYWww
WTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAATp3K7eYxNXuB
T9p1l8eAaMfFD9JfDdH9MzJ2WlR1tH6w9YqAyOSExgVrYi
…
-----END LICENSE-----`}
        </div>
        <div style={{ marginTop: 10, display: "flex", gap: 8 }}>
          <Btn label="Copier" hot="c" />
          <Btn label="Sauver…" hot="o" />
          <Btn label="QR (TOTP)" hot="q" />
        </div>
      </div>
    </div>
  );
}

window.WizardScreen = WizardScreen;
