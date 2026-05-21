// Settings

function SettingsScreen({ openOverlay }) {
  return (
    <div style={{ padding: 16, display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12, height: "100%", overflow: "auto" }}>
      <Box title="Defaults licence (wizard nouvelle licence)" focused>
        <div style={{ padding: 12 }}>
          <Input label="default_issuer_name" value="research@offsec.local" />
          <Input label="default_audience"     value="rshell, rshell-edu" hint="multi · séparé par ," />
          <Input label="default_ttl_seconds"  value="7776000 (90 j)" hint="durée par défaut NotAfter" />
          <Input label="default_keyid"        value="active" hint="« active » suit la clé courante, sinon un keyid explicite" />
        </div>
      </Box>

      <Box title="default_argon_preset (binding password)">
        <div style={{ padding: 12 }}>
          <div style={{ display: "flex", gap: 6, marginBottom: 10 }}>
            <span className="chip"><span className="k">1</span>fast (t=2 m=64MiB p=1)</span>
            <span className="chip active"><span className="k">2</span>default (t=3 m=256MiB p=2)</span>
            <span className="chip"><span className="k">3</span>paranoid (t=4 m=512MiB p=2)</span>
          </div>
          <div className="dim">
            Coût à la vérification côté binaire. paranoid ≈ 2.5s sur un i7.
          </div>
        </div>
      </Box>

      <Box title="Identité opérateur (audit actor)">
        <div style={{ padding: 12 }}>
          <Input label="operator_name" value={window.DATA.operator_name} hint="apparaît comme actor dans audit log" />
          <div className="dim" style={{ marginTop: 4 }}>
            Toutes les entries Audit sont tag­guées avec cette valeur. Modifiable à chaud.
          </div>
        </div>
      </Box>

      <Box title="Base de données">
        <div style={{ padding: 12 }}>
          <KV k="chemin" v={<span style={{ color: "var(--fg)" }}>{window.DATA.settings.db_path}</span>} />
          <KV k="taille" v="412 KiB · 47 licences · 4 keys · 4 identities" />
          <KV k="passphrase" v={<span className="dim">résolu via <span className="c-cyan">{window.DATA.settings.passphrase_resolved_via}</span></span>} />
          <div style={{ marginTop: 10 }}>
            <ActionRow k="P" label="changer la passphrase (rekey complet en transaction)" onClick={() => openOverlay && openOverlay({ kind: "rekey" })} />
            <ActionRow k="V" label="vacuum + analyse" />
            <ActionRow k="B" label="backup chiffré → fichier…" />
          </div>
        </div>
      </Box>

      <Box title="Cycle de vie serveurs HTTP">
        <div style={{ padding: 12 }}>
          <SecHead>À la fermeture</SecHead>
          <Toggle label="confirm_quit_with_servers — modal si serveur(s) ON" on={window.DATA.settings.confirm_quit_with_servers} />
          <Toggle label="arrêter tous les serveurs avant de sortir" on />
          <Rule />
          <SecHead>Au démarrage</SecHead>
          <Toggle label="auto_start_servers — démarrer les serveurs au boot" on={window.DATA.settings.auto_start_servers} />
          <Toggle label="ouvrir directement Dashboard (défaut)" on />
        </div>
      </Box>

      <Box title="Apparence">
        <div style={{ padding: 12 }}>
          <div style={{ display: "flex", gap: 6, marginBottom: 10, alignItems: "center" }}>
            <span className="dim">thème :</span>
            <span className="chip active"><span className="k">1</span>neon</span>
            <span className="chip"><span className="k">2</span>mono</span>
            <span className="chip"><span className="k">3</span>nord-soft</span>
          </div>
          <Toggle label="bold + couleur saturée (équivalent glow en TUI)" on />
          <Toggle label="densité confort (+1 ligne de padding partout)" />
          <Toggle label="show timestamps en local au lieu d'UTC" />
        </div>
      </Box>

      <Box title="Cascade passphrase au boot (read-only)">
        <div style={{ padding: 12 }} className="dim">
          La passphrase est résolue selon la cascade :
          <div style={{ marginLeft: 12, marginTop: 6, color: "var(--fg)" }}>
            <div>1. <span className="c-cyan">--passphrase-file</span> &lt;path&gt;</div>
            <div>2. env <span className="c-cyan">MALDEV_MGR_PASSPHRASE_FILE</span></div>
            <div>3. env <span className="c-cyan">MALDEV_MGR_PASSPHRASE</span></div>
            <div>4. fallback prompt TUI interactif</div>
          </div>
          <div style={{ marginTop: 8 }}>
            Cette session a résolu via : <span className="c-magenta">{window.DATA.settings.passphrase_resolved_via}</span>{" "}
            → écran passphrase <span className="c-green">sauté</span>.
          </div>
        </div>
      </Box>
    </div>
  );
}

function Toggle({ label, on }) {
  return (
    <div style={{ display: "flex", alignItems: "center", gap: 12, padding: "2px 0" }}>
      <span style={{
        display: "inline-block", width: 24, height: 12, position: "relative",
        background: "transparent",
        border: `1px solid ${on ? "var(--green)" : "var(--border)"}`,
      }}>
        <span style={{
          position: "absolute", top: 1, left: on ? 12 : 1,
          width: 9, height: 8,
          background: on ? "var(--green)" : "var(--fg-mute)",
        }} />
      </span>
      <span style={{ flex: 1, color: "var(--fg)" }}>{label}</span>
      <span className="dim">{on ? "on" : "off"}</span>
    </div>
  );
}

function ActionRow({ k, label, danger, enabled = true, onClick }) {
  const c = danger ? "var(--red)" : "var(--magenta)";
  return (
    <div style={{ display: "flex", alignItems: "center", gap: 10, opacity: enabled ? 1 : 0.35, padding: "1px 0", cursor: onClick ? "pointer" : "default" }} onClick={onClick}>
      <span style={{
        minWidth: 20, height: 20, display: "inline-flex", alignItems: "center", justifyContent: "center",
        border: `1px solid ${c}`, color: c, fontWeight: 700,
      }}>{k}</span>
      <span className="dim">{label}</span>
    </div>
  );
}

function SecHead({ children, style }) {
  return <div className="c-cyan" style={{ fontWeight: 700, marginBottom: 4, ...(style || {}) }}>{children}</div>;
}
function KV({ k, v }) {
  return (
    <div style={{ display: "flex", gap: 8 }}>
      <span className="dim" style={{ width: "12ch" }}>{k}</span>
      <span style={{ flex: 1, color: "var(--fg)" }}>{v}</span>
    </div>
  );
}

window.SettingsScreen = SettingsScreen;
