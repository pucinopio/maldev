// Dashboard: counters + active key + 3 servers + last audit
// Fidèle au rendu lipgloss : bold + couleur saturée, pas de font-size variable.

function DashboardScreen({ goto }) {
  const d = window.DATA;
  return (
    <div style={{ padding: 16, display: "flex", flexDirection: "column", gap: 12, height: "100%", overflow: "hidden" }}>
      {/* COUNTERS — même taille de cellule que tout le reste */}
      <div style={{ display: "grid", gridTemplateColumns: "repeat(5, 1fr)", gap: 12 }}>
        <Tile k="a" label="Actives"          value={d.counters.active}      kind="good"   footer="signées par la clé active" />
        <Tile k="r" label="Révoquées"        value={d.counters.revoked}     kind="danger" footer="présentes dans la CRL" />
        <Tile k="e" label="Expirées"         value={d.counters.expired}     kind=""       footer="NotAfter dépassé" />
        <Tile k="w" label="Expirent < 7 j"   value={d.counters.expiring_7d} kind="warn"   footer="à renouveler" />
        <Tile k="u" label="Superseded"       value={d.counters.superseded}  kind=""       footer="re-émises plus tard" />
      </div>

      {/* MIDDLE ROW */}
      <div style={{ display: "grid", gridTemplateColumns: "1.1fr 1.3fr", gap: 12, flex: 1, minHeight: 0 }}>
        {/* Active key + servers */}
        <div style={{ display: "flex", flexDirection: "column", gap: 12, minHeight: 0 }}>
          <Box title="Clé d'émission active" right={<span className="dim">[k] gérer</span>}>
            <div style={{ padding: "8px 12px" }}>
              <div style={{ display: "flex", justifyContent: "space-between", alignItems: "baseline", marginBottom: 6 }}>
                <span className="b-magenta">{d.active_key.keyid}</span>
                <StatusPill status="active" />
              </div>
              <div className="dim">nom <span style={{ color: "var(--fg)" }}>{d.active_key.name}</span></div>
              <div className="dim">fpr <span style={{ color: "var(--fg)" }}>{d.active_key.fingerprint}</span></div>
            </div>
          </Box>

          <Box title="Serveurs HTTP" right={<span className="dim">[7] détail · [s] start/stop</span>} style={{ flex: 1, minHeight: 0 }}>
            <div>
              {d.servers.map((s, i) => (
                <div key={s.id} style={{
                  padding: "6px 12px",
                  display: "flex", alignItems: "center", gap: 12,
                  borderTop: i === 0 ? "none" : "1px dashed var(--border)",
                }}>
                  <Dot kind={s.on ? "green" : "dim"} />
                  <div style={{ flex: 1 }}>
                    <div style={{ display: "flex", gap: 10, alignItems: "baseline" }}>
                      <span style={{ color: "var(--fg)", fontWeight: 700 }}>{s.name}</span>
                      <span className="mute">:{s.port}</span>
                    </div>
                    <div className="dim">
                      {s.on ? <>{s.url} · {s.reqs.toLocaleString("fr-FR")} req · up {s.uptime}</> : <>arrêté · démarrer via onglet [7]</>}
                    </div>
                  </div>
                  <StatusPill status={s.on ? "on" : "off"} />
                </div>
              ))}
            </div>
          </Box>
        </div>

        {/* Audit + Raccourcis */}
        <div style={{ display: "flex", flexDirection: "column", gap: 12, minHeight: 0 }}>
          <Box title="5 dernières actions" right={<span className="dim">[8] tout l'audit</span>} style={{ flex: 1, minHeight: 0 }} bodyStyle={{ overflow: "hidden" }}>
            <div style={{ padding: "4px 0" }}>
              {d.audit.map((a, i) => (
                <div key={i} className="row" style={{ padding: "3px 14px", gap: 10 }}>
                  <span className="mute" style={{ width: "7ch" }}>{a.t}</span>
                  <span className="c-cyan" style={{ width: "16ch" }}>{a.kind}</span>
                  <span style={{ flex: 1, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                    {a.target}{a.note && <span className="mute"> — {a.note}</span>}
                  </span>
                </div>
              ))}
            </div>
          </Box>

          <Box title="Raccourcis" right={<span className="dim">touche → écran</span>}>
            <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr 1fr", gap: 0 }}>
              {[
                { k: "n", l: "nouvelle licence",   c: "var(--magenta)" },
                { k: "/", l: "rechercher",         c: "var(--cyan)" },
                { k: "x", l: "révoquer",           c: "var(--red)" },
                { k: "k", l: "clés d'émission",    c: "var(--cyan)" },
                { k: "i", l: "identity.bin",       c: "var(--cyan)" },
                { k: "?", l: "aide contextuelle",  c: "var(--violet)" },
              ].map((s, i) => (
                <div key={i} style={{
                  padding: "8px 12px",
                  display: "flex", alignItems: "center", gap: 12,
                  borderTop: i > 2 ? "1px dashed var(--border)" : "none",
                  borderRight: (i % 3 < 2) ? "1px dashed var(--border)" : "none",
                }}>
                  <span style={{
                    minWidth: 20, height: 20, display: "inline-flex", alignItems: "center", justifyContent: "center",
                    border: `1px solid ${s.c}`, color: s.c, fontWeight: 700,
                  }}>{s.k}</span>
                  <span className="dim">{s.l}</span>
                </div>
              ))}
            </div>
          </Box>
        </div>
      </div>
    </div>
  );
}

window.DashboardScreen = DashboardScreen;
