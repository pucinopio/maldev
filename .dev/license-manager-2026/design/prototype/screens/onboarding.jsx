// Démarrage : DB existante (passphrase prompt) ou DB vide (onboarding wizard)
// Affiché plein écran en overlay au-dessus de l'app si state.session === "locked" | "onboarding"

function OnboardingScreen({ mode, setSession, openOverlay }) {
  // mode === "passphrase" → DB existante, demander passphrase
  // mode === "first-run"  → DB inexistante, wizard 4 étapes

  if (mode === "passphrase") {
    return <PassphrasePrompt setSession={setSession} openOverlay={openOverlay} />;
  }
  return <FirstRunWizard setSession={setSession} />;
}

function PassphrasePrompt({ setSession, openOverlay }) {
  return (
    <div style={{
      position: "absolute", inset: 0,
      display: "flex", alignItems: "center", justifyContent: "center",
      background: "var(--bg)",
    }}>
      <div style={{ width: 540, textAlign: "center" }}>
        <div className="b-magenta">◆ license-manager</div>
        <div className="mute" style={{ marginTop: 4 }}>local-first · offline-capable · zero-friction</div>
        <Rule />
        <div className="dim" style={{ marginBottom: 14 }}>
          Base existante à <span style={{ color: "var(--fg)" }}>~/.config/license-manager/db.sqlite</span>
        </div>

        <Box>
          <div style={{ padding: 16 }}>
            <Input label="passphrase" value="••••••••••••••" hint="entrée pour déverrouiller" masked focused />
            <div style={{ display: "flex", gap: 8, justifyContent: "center", marginTop: 8 }}>
              <Btn label="Déverrouiller" hot="↵" kind="primary" focused onClick={() => setSession("ready")} />
              <Btn label="Quitter" hot="esc" />
            </div>
          </div>
        </Box>
        <div className="mute" style={{ marginTop: 12 }}>
          <span className="c-yellow">3</span> essais restants · backoff exponentiel après chaque échec · cascade non-résolue (cf. Settings)
        </div>
      </div>
    </div>
  );
}

const FIRST_RUN_STEPS = [
  { n: 1, label: "Bienvenue", hint: "ce que va faire ce wizard" },
  { n: 2, label: "Passphrase DB", hint: "chiffre la base sqlite" },
  { n: 3, label: "Issuer & 1ère clé", hint: "identité d'émission + Ed25519" },
  { n: 4, label: "Première licence", hint: "pour toi, sur ta machine" },
];

function FirstRunWizard({ setSession }) {
  const [step, setStep] = React.useState(0);
  const s = FIRST_RUN_STEPS[step];

  return (
    <div style={{ position: "absolute", inset: 0, display: "flex", flexDirection: "column", background: "var(--bg)" }}>
      <div style={{
        padding: "14px 20px",
        borderBottom: "1px solid var(--border)",
        background: "linear-gradient(180deg, rgba(255,54,212,0.08), transparent)",
      }}>
        <div style={{ display: "flex", alignItems: "baseline", gap: 14 }}>
          <span className="b-magenta">◆ PREMIÈRE UTILISATION</span>
          <span className="dim">étape</span>
          <span style={{ color: "var(--fg)", fontWeight: 700 }}>{s.n}/4</span>
          <span className="dim">·</span>
          <span className="glow-cyan">{s.label}</span>
          <span style={{ flex: 1 }} />
          <HK k="Tab" important>continuer</HK>
        </div>
        <div className="progress-track" style={{ marginTop: 8 }}>
          <div className="progress-bar" style={{ width: `${(s.n / 4) * 100}%` }} />
        </div>
      </div>

      <div style={{ flex: 1, display: "flex", alignItems: "center", justifyContent: "center", padding: 30 }}>
        {step === 0 && <FrWelcome onNext={() => setStep(1)} />}
        {step === 1 && <FrPassphrase onNext={() => setStep(2)} onBack={() => setStep(0)} />}
        {step === 2 && <FrIssuer onNext={() => setStep(3)} onBack={() => setStep(1)} />}
        {step === 3 && <FrFirstLicense onNext={() => setSession("ready")} onBack={() => setStep(2)} />}
      </div>
    </div>
  );
}

function FrWelcome({ onNext }) {
  return (
    <div style={{ maxWidth: 720 }}>
      <div className="b-magenta" style={{ marginBottom: 12, textAlign: "center" }}>
        Aucune base détectée
      </div>
      <div className="dim" style={{ marginBottom: 20, textAlign: "center" }}>
        Ce wizard va te guider pour&nbsp;:
      </div>
      <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 14 }}>
        {[
          { n: 1, t: "Chiffrer une base SQLite locale avec ta passphrase",   c: "var(--cyan)" },
          { n: 2, t: "Créer ton identité d'émission (issuer)",               c: "var(--cyan)" },
          { n: 3, t: "Générer ta première paire de clés Ed25519",            c: "var(--magenta)" },
          { n: 4, t: "Émettre une licence pour toi, pinnée à ta machine",    c: "var(--magenta)" },
        ].map(it => (
          <div key={it.n} className="box" style={{ padding: 14, display: "flex", gap: 12 }}>
            <span style={{
              minWidth: 24, height: 24, display: "inline-flex", alignItems: "center", justifyContent: "center",
              border: `1px solid ${it.c}`, color: it.c, fontWeight: 700,
            }}>{it.n}</span>
            <span style={{ flex: 1 }}>{it.t}</span>
          </div>
        ))}
      </div>
      <Rule />
      <div className="dim" style={{ marginBottom: 14, textAlign: "center" }}>
        Tu pourras revenir sur tous ces choix dans Settings après coup.
      </div>
      <div style={{ display: "flex", gap: 8, justifyContent: "center" }}>
        <Btn label="Commencer" hot="↵" kind="primary" focused onClick={onNext} />
        <Btn label="Quitter" hot="esc" />
      </div>
    </div>
  );
}

function FrPassphrase({ onNext, onBack }) {
  return (
    <div style={{ width: 560 }}>
      <SecHead>2 / 4 — Passphrase de la base</SecHead>
      <div className="dim" style={{ marginBottom: 14 }}>
        Cette passphrase est demandée à chaque lancement. Elle ne peut pas être récupérée. Note-la dans ton gestionnaire de mots de passe.
      </div>
      <Input label="passphrase"   value="••••••••••••••" hint="≥ 12 caractères" masked focused />
      <Input label="confirmation" value="••••••••••••••" masked />
      <div className="dim" style={{ fontSize: 12, marginTop: 6 }}>
        force : <span style={{ color: "var(--green)" }}>forte</span> · entropie ≈ 92 bits · zxcvbn score 4/4
      </div>
      <Rule />
      <div style={{ display: "flex", gap: 8 }}>
        <Btn label="Suivant" hot="Tab/↵" kind="primary" focused onClick={onNext} />
        <Btn label="Précédent" hot="⇧Tab" onClick={onBack} />
      </div>
    </div>
  );
}

function FrIssuer({ onNext, onBack }) {
  return (
    <div style={{ width: 720, display: "grid", gridTemplateColumns: "1fr 1fr", gap: 30 }}>
      <div>
        <SecHead>3 / 4 — Issuer & première paire de clés</SecHead>
        <Input label="issuer (sera Settings → default)" value="research@offsec.local" focused />
        <Input label="KeyID (auto-suggéré)" value="k2026-05" suffix="↻ régénérer" />
        <Input label="nom court de la clé"  value="rshell-prod-2026Q2" />
      </div>
      <div>
        <SecHead>Aperçu</SecHead>
        <Box style={{ padding: 0 }}>
          <div style={{ padding: 14 }}>
            <div className="dim" style={{ marginBottom: 4 }}>algo</div>
            <div className="glow-cyan" style={{ marginBottom: 10 }}>Ed25519</div>
            <div className="dim" style={{ marginBottom: 4 }}>keyid</div>
            <div className="b-magenta" style={{ marginBottom: 10 }}>k2026-05</div>
            <div className="dim" style={{ marginBottom: 4 }}>fingerprint (sera calculé)</div>
            <div className="mute">— après génération —</div>
          </div>
        </Box>
        <div style={{ marginTop: 14, display: "flex", gap: 8 }}>
          <Btn label="Générer & continuer" hot="↵" kind="primary" focused onClick={onNext} />
          <Btn label="Précédent" hot="⇧Tab" onClick={onBack} />
        </div>
      </div>
    </div>
  );
}

function FrFirstLicense({ onNext, onBack }) {
  return (
    <div style={{ width: 720 }}>
      <SecHead>4 / 4 — Première licence (pour toi, sur ta machine)</SecHead>
      <div className="dim" style={{ marginBottom: 16 }}>
        On crée une licence minimale pour valider le flow. Tu pourras émettre la suivante depuis l'onglet Licences.
      </div>
      <Input label="subject"   value="research@offsec.local" focused />
      <Input label="audience"  value="rshell" />
      <Input label="NotAfter"  value="365d" />
      <Input label="bindings"  value="machine: ma machine actuelle (auto-détectée)" suffix="ID local 0c8a91…f4d2" />
      <Rule />
      <div style={{ display: "flex", gap: 8 }}>
        <Btn label="Émettre & entrer dans l'app" hot="↵" kind="primary" focused onClick={onNext} />
        <Btn label="Précédent" hot="⇧Tab" onClick={onBack} />
      </div>
    </div>
  );
}

window.OnboardingScreen = OnboardingScreen;
