// Recipient keys X25519 (pour sealed payload)

function RecipientsScreen({ sel }) {
  const keys = window.DATA.recipient_keys;
  return (
    <div style={{ padding: 16, display: "flex", flexDirection: "column", gap: 12, height: "100%", overflow: "hidden" }}>
      <div className="dim" style={{ fontSize: 12 }}>
        Les <span style={{ color: "var(--fg)" }}>recipient keys</span> servent à sceller un payload (NaCl box).
        Le destinataire d'une licence possède la clé privée X25519 et peut déchiffrer le sealed payload.
        Moins central que les clés d'émission Ed25519.
      </div>
      <Box title={`Recipient keys X25519 (${keys.length})`} focused
           right={<span className="dim">[n] générer · [i] importer · [E] export .pub · [x] retirer</span>}
           style={{ flex: 1, minHeight: 0, overflow: "auto" }}>
        <Table
          sel={sel}
          cols={[
            { h: "KEYID",      w: 1.0, cell: (r) => <span className="glow-cyan" style={{ fontWeight: 600 }}>{r.keyid}</span> },
            { h: "NOM",        w: 1.8, k: "name" },
            { h: "CRÉÉE",      w: 1.0, k: "created" },
            { h: "# SEALED",   w: 0.8, align: "right", cell: (r) => <span style={{ color: "var(--fg)" }}>{r.sealed}</span> },
            { h: "FINGERPRINT",w: 2.2, cell: (r) => <span className="dim" style={{ fontSize: 12 }}>{r.fpr}</span> },
          ]}
          rows={keys}
          expandedRowRender={(r) => (
            <div style={{ padding: "6px 4px", display: "grid", gridTemplateColumns: "1fr 1fr", gap: 20 }}>
              <div>
                <SecHead>Détail</SecHead>
                <KV k="keyid" v={<span className="glow-cyan">{r.keyid}</span>} />
                <KV k="name"  v={r.name} />
                <KV k="created" v={r.created} />
                <KV k="fpr"   v={<span className="glow-cyan">{r.fpr}</span>} />
                <KV k="sealed" v={`${r.sealed} licences ont un payload chiffré pour cette clé`} />
              </div>
              <div>
                <SecHead>Actions</SecHead>
                <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
                  <ActionRow k="E" label="exporter clé publique (.pub) pour l'embarquer dans le binaire" />
                  <ActionRow k="K" label="exporter clé privée (.key) — réservé au destinataire" />
                  <ActionRow k="x" label="retirer (sealed payloads existants deviennent inutilisables)" danger />
                </div>
              </div>
            </div>
          )}
        />
      </Box>
    </div>
  );
}

window.RecipientsScreen = RecipientsScreen;
