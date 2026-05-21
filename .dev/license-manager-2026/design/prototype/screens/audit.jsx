// Audit log — paginated table + filters + detail

function AuditScreen({ sel, setSel, filters, setFilters }) {
  const rows = window.DATA.audit_long.filter(a => {
    if (filters.kind && a.kind !== filters.kind) return false;
    return true;
  });

  const kinds = Array.from(new Set(window.DATA.audit_long.map(a => a.kind)));

  return (
    <div style={{ padding: 16, display: "flex", flexDirection: "column", gap: 12, height: "100%", overflow: "hidden" }}>
      <div style={{ display: "flex", gap: 8, flexWrap: "wrap", alignItems: "center" }}>
        <span className="dim" style={{ fontSize: 12 }}>filtres&nbsp;:</span>
        <span className="chip" style={{
          borderColor: !filters.kind ? "var(--magenta)" : "var(--border)",
          color: !filters.kind ? "var(--magenta)" : "var(--fg-dim)",
        }} onClick={() => setFilters({ ...filters, kind: null })}>
          <span className="k">f</span>all kinds
        </span>
        {kinds.map(k => (
          <span key={k} className="chip" style={{
            borderColor: filters.kind === k ? "var(--magenta)" : "var(--border)",
            color: filters.kind === k ? "var(--magenta)" : "var(--fg-dim)",
            cursor: "pointer",
          }} onClick={() => setFilters({ ...filters, kind: k })}>{k}</span>
        ))}
        <span style={{ flex: 1 }} />
        <span className="dim" style={{ fontSize: 12 }}>
          <HK k="E">export CSV</HK><HK k="J">export JSON</HK>
        </span>
      </div>

      <Box title={`Audit (${rows.length})`} focused
           right={<span className="dim">[d] détail · [r] refresh · [pgup/pgdn] page</span>}
           style={{ flex: 1, minHeight: 0, overflow: "auto" }}>
        <Table
          sel={sel}
          cols={[
            { h: "TIMESTAMP", w: 1.4, cell: (r) => <span className="dim" style={{ fontSize: 12 }}>{r.t}</span> },
            { h: "KIND",      w: 1.2, cell: (r) => <span className="glow-cyan" style={{ fontSize: 13 }}>{r.kind}</span> },
            { h: "ACTOR",     w: 0.7, k: "actor" },
            { h: "TARGET",    w: 2.0, cell: (r) => <span className="glow-magenta">{r.target}</span> },
            { h: "NOTE",      w: 1.4, cell: (r) => <span className="dim">{r.note || "—"}</span> },
          ]}
          rows={rows}
          expandedRowRender={(r) => (
            <div style={{ display: "grid", gridTemplateColumns: "1fr 1.3fr", gap: 20, padding: "6px 4px" }}>
              <div>
                <SecHead>Entry</SecHead>
                <KV k="id"     v={r.id} />
                <KV k="t"      v={r.t} />
                <KV k="kind"   v={<span className="glow-cyan">{r.kind}</span>} />
                <KV k="actor"  v={r.actor} />
                <KV k="target" v={<span className="glow-magenta">{r.target}</span>} />
                <KV k="note"   v={r.note || "—"} />
              </div>
              <div>
                <SecHead>Payload JSON</SecHead>
                <div className="ascii" style={{ background: "var(--bg)", padding: 8, border: "1px solid var(--border)" }}>
{`{`}<br />
<span style={{ marginLeft: 8 }}><span style={{ color: "var(--cyan)" }}>"kind"</span>: <span style={{ color: "var(--yellow)" }}>"{r.kind}"</span>,</span><br />
<span style={{ marginLeft: 8 }}><span style={{ color: "var(--cyan)" }}>"actor"</span>: <span style={{ color: "var(--yellow)" }}>"{r.actor}"</span>,</span><br />
<span style={{ marginLeft: 8 }}><span style={{ color: "var(--cyan)" }}>"target"</span>: <span style={{ color: "var(--yellow)" }}>"{r.target}"</span>,</span><br />
<span style={{ marginLeft: 8 }}><span style={{ color: "var(--cyan)" }}>"keyid"</span>: <span style={{ color: "var(--yellow)" }}>"k2026-04"</span>,</span><br />
<span style={{ marginLeft: 8 }}><span style={{ color: "var(--cyan)" }}>"ip"</span>: <span style={{ color: "var(--yellow)" }}>"127.0.0.1"</span>,</span><br />
<span style={{ marginLeft: 8 }}><span style={{ color: "var(--cyan)" }}>"db_version"</span>: <span style={{ color: "var(--magenta)" }}>17</span></span><br />
{`}`}
                </div>
              </div>
            </div>
          )}
        />
      </Box>
    </div>
  );
}

window.AuditScreen = AuditScreen;
