// Revocation : CRL active

function RevocationScreen({ sel }) {
  const entries = window.DATA.revocations;
  return (
    <div style={{ padding: 16, display: "flex", flexDirection: "column", gap: 12, height: "100%", overflow: "hidden" }}>
      <div style={{ display: "grid", gridTemplateColumns: "repeat(3, 1fr)", gap: 12 }}>
        <Tile label="Entries CRL" value={entries.length} kind="danger" footer="révocations signées par l'issuer actif" />
        <Tile label="Pushed via :8443" value="oui" kind="good" footer="serveur révocation en ligne" />
        <Tile label="Dernier export" value="13:22" footer="manager.crl.pem (offline copy)" />
      </div>

      <Box title={`Liste des révocations (${entries.length})`} focused
           right={<span className="dim">[n] ajouter (sélectionner licence) · [x] retirer · [E] exporter PEM signé</span>}
           style={{ flex: 1, minHeight: 0, overflow: "auto" }}>
        <Table
          sel={sel}
          cols={[
            { h: "LICENCE", w: 2.4, cell: (r) => <span className="glow-magenta">{r.lic}</span> },
            { h: "KEYID",   w: 1.0, cell: (r) => <span className="glow-cyan">{r.keyid}</span> },
            { h: "AT",      w: 1.4, k: "at" },
            { h: "REASON",  w: 1.6, cell: (r) => <span style={{ color: "var(--red)" }}>{r.reason}</span> },
          ]}
          rows={entries}
          expandedRowRender={(r) => (
            <div style={{ padding: "6px 4px", display: "grid", gridTemplateColumns: "1fr 1fr", gap: 20 }}>
              <div>
                <SecHead>Détail</SecHead>
                <KV k="lic_id" v={<span className="glow-magenta">{r.lic}</span>} />
                <KV k="keyid"  v={<span className="glow-cyan">{r.keyid}</span>} />
                <KV k="at"     v={r.at} />
                <KV k="reason" v={<span style={{ color: "var(--red)" }}>{r.reason}</span>} />
                <KV k="actor"  v="operator" />
                <KV k="serial" v="00:ff:1a:9c:..." />
              </div>
              <div>
                <SecHead>Actions</SecHead>
                <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
                  <ActionRow k="↵" label="ouvrir la licence dans l'onglet [2]" />
                  <ActionRow k="x" label="retirer de la CRL (réhabilite la licence)" danger />
                </div>
              </div>
            </div>
          )}
        />
      </Box>
    </div>
  );
}

window.RevocationScreen = RevocationScreen;
