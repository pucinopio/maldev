// Identités binaires (identity.bin)

function IdentitiesScreen({ sel, openOverlay }) {
  const items = window.DATA.identities;
  return (
    <div style={{ padding: 16, display: "flex", flexDirection: "column", gap: 12, height: "100%", overflow: "hidden" }}>
      <div className="dim" style={{ fontSize: 12 }}>
        Une <span style={{ color: "var(--fg)" }}>identity.bin</span> est un blob de 32 octets aléatoires embarqué dans le binaire via{" "}
        <code style={{ color: "var(--cyan)" }}>//go:embed</code>. Une licence peut être pinnée à son sha256 (IdentitySHA256) pour
        qu'elle ne soit valide que pour cette identité de binaire précise.
      </div>
      <Box title={`Identities (${items.length})`} focused
           right={<span className="dim">[n] créer · [E] export .bin · [R] régénérer ⚠ · [x] supprimer</span>}
           style={{ flex: 1, minHeight: 0, overflow: "auto" }}>
        <Table
          sel={sel}
          cols={[
            { h: "NOM",     w: 2.2, k: "name" },
            { h: "SHA256",  w: 2.0, cell: (r) => <span className="glow-cyan" style={{ fontSize: 13 }}>{r.sha}</span> },
            { h: "# REFS",  w: 0.7, align: "right", cell: (r) => <span style={{ color: r.refs > 0 ? "var(--fg)" : "var(--fg-mute)" }}>{r.refs}</span> },
            { h: "CRÉÉE",   w: 1.0, k: "created" },
          ]}
          rows={items}
          expandedRowRender={(r) => (
            <div style={{ padding: "6px 4px", display: "grid", gridTemplateColumns: "1fr 1fr", gap: 20 }}>
              <div>
                <SecHead>Détail</SecHead>
                <KV k="name"   v={r.name} />
                <KV k="sha256" v={<span className="glow-cyan">{r.sha}</span>} />
                <KV k="bytes"  v="32 (aléatoires, crypto/rand)" />
                <KV k="created" v={r.created} />
                <KV k="refs"   v={`${r.refs} licence(s) pinnée(s) sur cette identité`} />
              </div>
              <div>
                <SecHead>Actions</SecHead>
                <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
                  <ActionRow k="E" label="exporter le .bin (prêt pour //go:embed)" />
                  <ActionRow k="R" label={`régénérer (casse ${r.refs} licence${r.refs > 1 ? "s" : ""})`} danger={r.refs > 0} />
                  <ActionRow k="x" label={r.refs > 0 ? `supprimer (impossible : ${r.refs} refs)` : "supprimer"} danger enabled={r.refs === 0} />
                </div>
                {r.refs > 0 && (
                  <div style={{ marginTop: 10, padding: "8px 10px", border: "1px dashed var(--yellow)", color: "var(--yellow)", fontSize: 12 }}>
                    ⚠ Régénérer change le sha256. Toute licence pinnée sur l'ancien sha cessera de valider.
                  </div>
                )}
              </div>
            </div>
          )}
        />
      </Box>
    </div>
  );
}

window.IdentitiesScreen = IdentitiesScreen;
