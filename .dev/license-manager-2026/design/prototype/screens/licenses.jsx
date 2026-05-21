// Licenses : table en haut, détail (split-pane) en bas pour la ligne sélectionnée.
// Fidèle à ce que bubbles/table permet : pas d'inline expand.

function LicensesScreen({ goto, openWizard, openOverlay, sel, setSel, search, setSearch, filters, setFilters, detailOpen, setDetailOpen, detailTab, setDetailTab }) {
  const all = window.DATA.licenses;
  const visible = all.filter(l => {
    if (filters.status && l.status !== filters.status) return false;
    if (search && !l.subj.toLowerCase().includes(search.toLowerCase())) return false;
    return true;
  });
  const selectedLic = visible[sel] || visible[0];

  return (
    <div style={{ padding: 16, display: "flex", flexDirection: "column", gap: 10, height: "100%", overflow: "hidden" }}>
      {/* Filters bar */}
      <div style={{ display: "flex", gap: 12, alignItems: "center" }}>
        <Box style={{ flex: 1 }}>
          <div style={{ padding: "4px 10px", display: "flex", alignItems: "center", gap: 10 }}>
            <span className="c-cyan">/</span>
            <span style={{ flex: 1, color: search ? "var(--fg)" : "var(--fg-mute)" }}>
              {search || "rechercher dans subject…"}
              <span className="caret">&nbsp;</span>
            </span>
            <span className="dim">{visible.length}/{all.length}</span>
          </div>
        </Box>

        <div style={{ display: "flex", gap: 6 }}>
          {["all", "active", "expiring", "expired", "revoked", "superseded"].map(s => {
            const active = filters.status === (s === "all" ? null : s);
            return (
              <span key={s}
                className={"chip" + (active ? " active" : "")}
                style={{ cursor: "pointer" }}
                onClick={() => setFilters({ ...filters, status: s === "all" ? null : s })}
              >
                <span className="k">f</span>{s}
              </span>
            );
          })}
        </div>
      </div>

      {/* TABLE */}
      <Box title={`Licences (${visible.length})`} focused
           right={<span className="dim">[↑↓] nav · [d] détail · [n] nouvelle · [x] révoquer · [e] re-émettre</span>}
           style={{ flex: detailOpen ? 0.55 : 1, minHeight: 0, overflow: "hidden" }}
           bodyStyle={{ height: "100%", overflow: "auto" }}>
        <Table
          sel={sel}
          cols={[
            { h: "STATUS",   w: 1.1, cell: (r) => <StatusPill status={r.status} /> },
            { h: "SUBJECT",  w: 1.6, cell: (r) => (
              <span>
                {r.subj}
                {r.parent     && <span className="c-violet" title="re-émise"> ↩</span>}
                {r.successors?.length > 0 && <span className="c-violet" title="re-émise plus tard"> ↪</span>}
              </span>
            ) },
            { h: "ISSUER",   w: 1.7, cell: (r) => <span className="dim">{r.iss}</span> },
            { h: "AUDIENCE", w: 1.0, k: "aud" },
            { h: "KEYID",    w: 1.0, cell: (r) => <span className="c-cyan">{r.keyid}</span> },
            { h: "EXPIRES",  w: 1.0, cell: (r) => <span style={{ color: r.status === "expiring" ? "var(--yellow)" : r.status === "expired" ? "var(--fg-mute)" : "var(--fg)" }}>{r.exp}</span> },
            { h: "FEATURES", w: 2.0, cell: (r) => (
              <span style={{ display: "flex", gap: 4, flexWrap: "wrap" }}>
                {r.features.map(f => <span key={f} className="chip" style={{ padding: "0 6px" }}>{f}</span>)}
              </span>
            ) },
          ]}
          rows={visible}
        />
      </Box>

      {/* DETAIL split-pane below */}
      {detailOpen && selectedLic && (
        <Box title={<span>Détail · <span className="c-magenta">{selectedLic.id}</span> · {selectedLic.subj}</span>}
             right={<span className="dim">[d] replier · onglets : [I]dent · [B]ind · [P]EM · [A]udit · [C]haîne</span>}
             style={{ flex: 0.45, minHeight: 0, overflow: "hidden" }}
             bodyStyle={{ height: "100%", overflow: "auto" }}>
          <LicenseDetail license={selectedLic} tab={detailTab} setTab={setDetailTab} openOverlay={openOverlay} />
        </Box>
      )}
    </div>
  );
}

function LicenseDetail({ license, tab = "ident", setTab, openOverlay }) {
  const hasTotp = license.subj.startsWith("alice");
  const hasMachine = !license.subj.startsWith("rshell-demo");
  const hasPwd = license.subj.startsWith("evgeny");
  const parent = license.parent && window.DATA.licenses.find(l => l.id === license.parent);
  const successors = (license.successors || []).map(id => window.DATA.licenses.find(l => l.id === id)).filter(Boolean);
  const chainLen = (parent ? 1 : 0) + 1 + successors.length;

  return (
    <div>
      {/* parent banner */}
      {parent && (
        <div style={{ padding: "5px 12px", borderBottom: "1px solid var(--border)", background: "rgba(160,112,255,0.06)", color: "var(--violet)" }}>
          ↩ Re-émise depuis <span className="b-violet">{parent.id}</span> ({parent.subj}, expirée {parent.exp})
          <span className="dim"> · enter pour ouvrir</span>
        </div>
      )}

      {/* tab strip detail */}
      <div style={{ display: "flex", borderBottom: "1px solid var(--border)", background: "var(--bg-1)" }}>
        {[
          { k: "I", id: "ident",  l: "Identité" },
          { k: "B", id: "bind",   l: "Bindings" },
          { k: "P", id: "pem",    l: "PEM" },
          { k: "A", id: "audit",  l: "Audit" },
          { k: "C", id: "chain",  l: `Chaîne (${chainLen})` },
        ].map(t => (
          <div key={t.id} onClick={() => setTab && setTab(t.id)} style={{
            padding: "4px 14px",
            cursor: "pointer",
            color: tab === t.id ? "var(--fg)" : "var(--fg-dim)",
            background: tab === t.id ? "var(--bg-2)" : "transparent",
            borderBottom: tab === t.id ? "2px solid var(--magenta)" : "2px solid transparent",
            marginBottom: "-1px",
            display: "flex", gap: 8, alignItems: "center",
          }}>
            <span style={{ color: tab === t.id ? "var(--magenta)" : "var(--fg-mute)", fontWeight: 700 }}>[{t.k}]</span>
            <span>{t.l}</span>
          </div>
        ))}
        <span style={{ flex: 1 }} />
        <div style={{ padding: "4px 14px", display: "flex", gap: 6 }}>
          <ActionRow k="c" label="copier PEM" inline />
          <ActionRow k="o" label="sauver…" inline />
          {hasTotp && <ActionRow k="q" label="QR" inline />}
          <ActionRow k="e" label="re-émettre" inline />
          <ActionRow k="x" label="révoquer" danger inline />
        </div>
      </div>

      {/* body */}
      <div style={{ padding: "8px 12px" }}>
        {tab === "ident" && (
          <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 16 }}>
            <div>
              <SecHead>Identité</SecHead>
              <KV k="lic_id"   v={<span className="c-magenta">{license.id}</span>} />
              <KV k="subject"  v={license.subj} />
              <KV k="issuer"   v={license.iss} />
              <KV k="audience" v={license.aud} />
              <KV k="keyid"    v={<span className="c-cyan">{license.keyid}</span>} />
            </div>
            <div>
              <SecHead>Validité</SecHead>
              <KV k="not_before" v="2026-05-20 13:42:18 UTC" />
              <KV k="not_after"  v={<span style={{ color: license.status === "expiring" ? "var(--yellow)" : "var(--fg)" }}>{license.exp} 00:00:00 UTC</span>} />
              <KV k="features"   v={license.features.join(", ")} />
              <KV k="status"     v={<StatusPill status={license.status} />} />
            </div>
          </div>
        )}

        {tab === "bind" && (
          <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 16 }}>
            <div>
              <SecHead>Bindings ({(hasMachine?1:0)+(hasTotp?1:0)+(hasPwd?1:0)})</SecHead>
              {hasMachine && (
                <div style={{ marginBottom: 6 }}>
                  <div className="c-violet">◆ machine (OR de 2 IDs)</div>
                  <KV k="local"     v="0c8a91…f4d2" />
                  <KV k="composite" v="7c91aa…1208" />
                </div>
              )}
              {hasTotp && (
                <div style={{ marginBottom: 6 }}>
                  <div className="c-violet">◆ totp</div>
                  <KV k="secret" v={<span className="mute">[masqué — q pour QR]</span>} />
                  <KV k="alg"    v="SHA1 · digits 6 · period 30s" />
                </div>
              )}
              {hasPwd && (
                <div style={{ marginBottom: 6 }}>
                  <div className="c-violet">◆ password (argon2id)</div>
                  <KV k="preset" v="paranoid (t=4 m=512MiB p=2)" />
                </div>
              )}
            </div>
            <div>
              <SecHead>Pinning</SecHead>
              <KV k="identity"  v="rshell-linux-amd64.bin (01ffa2d8…7c4)" />
              <KV k="binary"    v="8b3c91ad…2e1" />
              <SecHead>Sealed payload</SecHead>
              <KV k="recipient" v={hasTotp ? "r2026-01 (default-recipient)" : <span className="mute">— aucun —</span>} />
            </div>
          </div>
        )}

        {tab === "pem" && (
          <div>
            <SecHead>PEM (preview)</SecHead>
            <div className="ascii" style={{ background: "var(--bg)", padding: 8, border: "1px solid var(--border)", color: "var(--green)" }}>
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
          </div>
        )}

        {tab === "audit" && (
          <div>
            <SecHead>Audit · target_id = {license.id}</SecHead>
            <div className="box">
              {[
                { t: "2026-05-20 13:42:18", k: "license.issue",  n: "k2026-04, +machine +totp" },
                { t: "2026-05-20 13:42:18", k: "audit.append",   n: "auto" },
                { t: "2026-05-19 17:01:00", k: "license.export", n: "PEM → /tmp/alice.lic" },
              ].map((a, i) => (
                <div key={i} className="row" style={{ padding: "3px 10px", borderTop: i === 0 ? "none" : "1px dashed var(--border)" }}>
                  <span className="mute" style={{ width: "20ch" }}>{a.t}</span>
                  <span className="c-cyan" style={{ width: "16ch" }}>{a.k}</span>
                  <span className="dim" style={{ flex: 1 }}>{a.n}</span>
                </div>
              ))}
            </div>
          </div>
        )}

        {tab === "chain" && (
          <div>
            <SecHead>Chaîne de re-émissions</SecHead>
            <div className="ascii" style={{ color: "var(--fg)", padding: "4px 8px" }}>
              {parent ? (
                <span><span className="c-violet">↩ {parent.id}</span> <span className="mute">({parent.subj}, {parent.status})</span>{"\n    │\n    ▼\n"}</span>
              ) : <span className="mute">{"(licence racine — pas de parent)\n"}</span>}
              <span className="b-magenta">● {license.id}</span> <span className="mute">({license.subj}, </span>
              <StatusPill status={license.status} />
              <span className="mute">)</span>
              {successors.length > 0 && successors.map(s => (
                <React.Fragment key={s.id}>
                  {"\n    │\n    ▼\n"}
                  <span className="c-violet">↪ {s.id}</span> <span className="mute">({s.subj}, {s.status}, expire {s.exp})</span>
                </React.Fragment>
              ))}
              {successors.length === 0 && <span className="mute">{"\n(pas de successeur — licence terminale)"}</span>}
            </div>
            {license.status === "superseded" && (
              <div style={{ marginTop: 8, padding: "5px 10px", border: "1px dashed var(--violet)", color: "var(--violet)" }}>
                Cette licence est <span className="b-violet">superseded</span> — re-émise plus tard et marquée non-utilisable. Re-émettre depuis ici est <span className="c-red">refusé</span> (cf. cas d'erreur §8).
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
}

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
function ActionRow({ k, label, danger, enabled = true, inline }) {
  const c = danger ? "var(--red)" : "var(--magenta)";
  if (inline) {
    return (
      <span style={{ display: "inline-flex", alignItems: "center", gap: 6, opacity: enabled ? 1 : 0.35 }}>
        <span style={{
          minWidth: 16, height: 16, display: "inline-flex", alignItems: "center", justifyContent: "center",
          border: `1px solid ${c}`, color: c, fontWeight: 700,
        }}>{k}</span>
        <span className="dim">{label}</span>
      </span>
    );
  }
  return (
    <div style={{ display: "flex", alignItems: "center", gap: 10, opacity: enabled ? 1 : 0.35 }}>
      <span style={{
        minWidth: 20, height: 20, display: "inline-flex", alignItems: "center", justifyContent: "center",
        border: `1px solid ${c}`, color: c, fontWeight: 700,
      }}>{k}</span>
      <span className="dim">{label}</span>
    </div>
  );
}

window.LicensesScreen = LicensesScreen;
