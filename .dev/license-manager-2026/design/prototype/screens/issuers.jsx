// Issuer keys (Ed25519)

function IssuersScreen({ sel, setSel }) {
  const keys = window.DATA.issuer_keys;
  return (
    <div style={{ padding: 16, display: "flex", flexDirection: "column", gap: 12, height: "100%", overflow: "hidden" }}>
      <Box title={`Clés d'émission Ed25519 (${keys.length})`} focused
           right={<span className="dim">[n] générer · [i] importer · [a] désigner active · [E] export .pub · [K] export .key · [x] retirer</span>}
           style={{ flex: 1, minHeight: 0, overflow: "auto" }}>
        <Table
          sel={sel}
          cols={[
            { h: "KEYID",    w: 1.1, cell: (r) => <span className={r.status === "active" ? "glow-magenta" : "glow-cyan"} style={{ fontWeight: 600 }}>{r.keyid}</span> },
            { h: "NOM",      w: 2.0, k: "name" },
            { h: "STATUS",   w: 0.9, cell: (r) => <StatusPill status={r.status} /> },
            { h: "CRÉÉE",    w: 1.1, k: "created" },
            { h: "# SIGNÉES",w: 1.0, align: "right", cell: (r) => <span style={{ color: "var(--fg)" }}>{r.signed}</span> },
            { h: "FINGERPRINT", w: 2.4, cell: (r) => <span className="dim" style={{ fontSize: 12 }}>{r.fpr}</span> },
          ]}
          rows={keys}
          expandedRowRender={(r) => (
            <div style={{ display: "grid", gridTemplateColumns: "1fr 1.2fr", gap: 20, padding: "6px 4px" }}>
              <div>
                <SecHead>Métadonnées</SecHead>
                <KV k="keyid"   v={<span className="glow-cyan">{r.keyid}</span>} />
                <KV k="name"    v={r.name} />
                <KV k="status"  v={<StatusPill status={r.status} />} />
                <KV k="created" v={r.created} />
                <KV k="signed"  v={`${r.signed} licences`} />
                <KV k="fpr"     v={<span className="glow-cyan">{r.fpr}</span>} />
                <Rule />
                <SecHead>Actions</SecHead>
                <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
                  <ActionRow k="a" label={r.status === "active" ? "déjà active" : "désigner active"} enabled={r.status !== "active"} />
                  <ActionRow k="E" label="exporter clé publique (.pub)" />
                  <ActionRow k="K" label="exporter clé privée (.key) — confirmation + passphrase" />
                  <ActionRow k="x" label="retirer (la clé reste vérifiable côté binaire)" danger />
                </div>
              </div>
              <div>
                <SecHead>Licences signées par cette clé ({Math.min(r.signed, 6)} affichées)</SecHead>
                <div className="box" style={{ maxHeight: 240, overflow: "auto" }}>
                  {window.DATA.licenses.filter(l => l.keyid === r.keyid).slice(0, 8).map((l, i) => (
                    <div key={i} className="row" style={{ padding: "4px 12px", borderTop: i === 0 ? "none" : "1px dashed var(--border)" }}>
                      <StatusPill status={l.status} />
                      <span style={{ flex: 1 }}>{l.subj}</span>
                      <span className="dim">{l.exp}</span>
                    </div>
                  ))}
                  {window.DATA.licenses.filter(l => l.keyid === r.keyid).length === 0 && (
                    <div className="dim" style={{ padding: "12px 14px", textAlign: "center" }}>— aucune licence n'utilise cette clé —</div>
                  )}
                </div>
              </div>
            </div>
          )}
        />
      </Box>
    </div>
  );
}

window.IssuersScreen = IssuersScreen;
