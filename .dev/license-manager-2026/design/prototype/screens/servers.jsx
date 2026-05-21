// Servers : 3 sous-onglets (R)evoc, (H)eartbeat, (P)robe.
// Probe a en plus 3 vues internes : Tokens / History / Live log.

const SERVERS_SUBS = [
  { k: "R", id: "revoc",     label: "Revocation" },
  { k: "H", id: "heartbeat", label: "Heartbeat" },
  { k: "P", id: "probe",     label: "Fingerprint probe" },
];
window.SERVERS_SUBS = SERVERS_SUBS;

function ServersScreen({ sub, setSub, probeView, setProbeView, openOverlay }) {
  const srv = window.DATA.servers.find(s => s.id === sub) || window.DATA.servers[0];

  return (
    <div style={{ display: "flex", flexDirection: "column", height: "100%", overflow: "hidden" }}>
      {/* sub-tabs */}
      <div style={{ display: "flex", padding: "5px 14px 0", borderBottom: "1px solid var(--border)", background: "var(--bg-1)" }}>
        {SERVERS_SUBS.map(t => {
          const s = window.DATA.servers.find(x => x.id === t.id);
          const active = sub === t.id;
          return (
            <div key={t.id} onClick={() => setSub(t.id)} style={{
              padding: "4px 14px",
              cursor: "pointer",
              borderBottom: active ? "2px solid var(--cyan)" : "2px solid transparent",
              color: active ? "var(--fg)" : "var(--fg-dim)",
              display: "flex", alignItems: "center", gap: 10,
              marginBottom: -1,
            }}>
              <span style={{ color: active ? "var(--cyan)" : "var(--fg-mute)", fontWeight: 700 }}>[{t.k}]</span>
              <span>{t.label}</span>
              <Dot kind={s?.on ? "green" : "dim"} />
            </div>
          );
        })}
        <span style={{ flex: 1 }} />
        <span className="dim" style={{ padding: "4px 14px" }}>events fan-in via httpsrv.MergedEvents()</span>
      </div>

      <div style={{ flex: 1, display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12, padding: 16, minHeight: 0 }}>
        {/* left: status + config */}
        <div style={{ display: "flex", flexDirection: "column", gap: 12, minHeight: 0 }}>
          <Box title="Status" focused right={<span className="dim">[s] {srv.on ? "stop" : "start"}</span>}>
            <div style={{ padding: 12 }}>
              <div style={{ display: "flex", alignItems: "center", gap: 12, marginBottom: 8 }}>
                <Dot kind={srv.on ? "green" : "dim"} />
                <StatusPill status={srv.on ? "on" : "off"} />
                <span className="dim">port</span><span style={{ color: "var(--fg)" }}>:{srv.port}</span>
              </div>
              {srv.on ? (
                <>
                  <KV k="url"     v={<span className="c-cyan">{srv.url}</span>} />
                  <KV k="uptime"  v={srv.uptime} />
                  <KV k="req tot" v={srv.reqs.toLocaleString("fr-FR")} />
                  <KV k="req/s"   v="0.21 (1min avg)" />
                </>
              ) : (
                <div className="dim">— serveur arrêté — démarrer avec <span className="b-magenta">[s]</span></div>
              )}
            </div>
          </Box>

          <Box title="Configuration" right={<span className="dim">[e] éditer · [g] régénérer token</span>}>
            <div style={{ padding: 12 }}>
              <Input label="port" value={String(srv.port)} hint="1024-65535" />
              <Input label="TLS cert" value="/etc/license-manager/tls.crt" hint="filepicker" suffix="↵ pour ouvrir" />
              <Input label="TLS key"  value="/etc/license-manager/tls.key" hint="filepicker" suffix="↵ pour ouvrir" />
              <Input label="admin token" value={srv.on ? "tk_••••••••••••••" : "tk_aB3xZ9mLqP21vR..."} hint={srv.on ? "masqué — [g] régénérer (stop+regen+restart)" : "affiché une fois"} masked={srv.on} suffix="[g] regen" />
              {sub === "probe" && (
                <>
                  <Input label="token TTL par défaut" value="60s" hint="ProbeService.NewToken(label, ttl)" />
                  <Input label="max tokens actifs"     value="8" />
                </>
              )}
              <div className="dim" style={{ marginTop: 6 }}>
                {sub !== "probe"
                  ? <>endpoint : <span className="c-cyan">{sub === "revoc" ? "/crl, /revoke (admin)" : "/heartbeat/<id>, /metrics"}</span></>
                  : <>endpoints : <span className="c-cyan">/probe/&lt;token&gt; (one-shot), /probe/&lt;token&gt;/agent</span></>}
              </div>
            </div>
          </Box>
        </div>

        {/* right: variable per sub */}
        <div style={{ display: "flex", flexDirection: "column", gap: 12, minHeight: 0 }}>
          {sub === "revoc" && <RevocPanel srv={srv} />}
          {sub === "heartbeat" && <HeartbeatPanel srv={srv} />}
          {sub === "probe" && <ProbePanel srv={srv} view={probeView} setView={setProbeView} openOverlay={openOverlay} />}
        </div>
      </div>
    </div>
  );
}

function LiveLog({ srv, filter }) {
  return (
    <Box title={`Live log${filter ? " · filtre " + filter : ""}`} right={<span className="dim">[c] clear · [a] auto-scroll on</span>}
         style={{ flex: 1, minHeight: 0 }} bodyStyle={{ height: "100%", overflow: "hidden" }}>
      <div style={{ height: "100%", overflow: "auto", padding: "4px 0" }}>
        {srv.on ? (
          <pre style={{ margin: 0, padding: "0 12px", fontFamily: "inherit", color: "var(--fg-dim)" }}>
{`13:42:18  POST  /revoke           200  18ms   1.2.3.4    ua=rshell/0.4.0
13:42:14  GET   /crl              200   3ms   1.2.3.4    ua=rshell/0.4.0
13:42:08  GET   /crl              200   3ms   10.0.4.21  ua=rshell/0.4.0
13:42:01  GET   /healthz          200   1ms   127.0.0.1  ua=manager-probe
13:41:55  GET   /crl              200   4ms   78.92.1.8  ua=rshell/0.3.9
13:41:42  POST  /revoke           401  12ms   78.92.1.8  ua=curl/8.4
13:41:31  GET   /crl              200   3ms   10.0.4.21  ua=rshell/0.4.0
13:41:18  GET   /metrics          200   2ms   127.0.0.1  ua=prom/2.50
13:41:02  GET   /crl              304   1ms   10.0.4.21  ua=rshell/0.4.0
13:40:55  GET   /crl              304   1ms   78.92.1.8  ua=rshell/0.3.9
13:40:48  GET   /crl              200   3ms   10.0.4.21  ua=rshell/0.4.0
…`}
          </pre>
        ) : (
          <div className="dim" style={{ padding: 14, textAlign: "center" }}>— pas de log — serveur arrêté —</div>
        )}
      </div>
    </Box>
  );
}

function RevocPanel({ srv }) {
  return (
    <>
      <LiveLog srv={srv} filter='Server=="revocation"' />
      <Box title="Endpoints" style={{ height: 160 }}>
        <div style={{ padding: 12 }}>
          <KV k="GET /crl"          v="renvoie la CRL signée (PEM)" />
          <KV k="GET /crl?keyid=…"  v="filtré par KeyID" />
          <KV k="POST /revoke"      v="ajoute entry (admin token requis)" />
          <KV k="GET /healthz"      v="200 OK" />
          <KV k="GET /metrics"      v="prometheus" />
        </div>
      </Box>
    </>
  );
}

function HeartbeatPanel({ srv }) {
  return (
    <>
      <LiveLog srv={srv} filter='Server=="heartbeat"' />
      <Box title="Licences live (toggle individuel)" right={<span className="dim">[space] toggle considéré-révoqué</span>}
           style={{ height: 200 }} bodyStyle={{ height: "100%", overflow: "auto" }}>
        {window.DATA.licenses.slice(0, 6).map((l, i) => (
          <div key={i} className="row" style={{ padding: "3px 12px", gap: 10 }}>
            <span style={{ width: "2ch", color: l.status === "revoked" ? "var(--red)" : "var(--green)", fontWeight: 700 }}>
              {l.status === "revoked" ? "✗" : "✓"}
            </span>
            <span style={{ flex: 1 }}>{l.subj}</span>
            <span className="dim">{l.keyid}</span>
          </div>
        ))}
      </Box>
    </>
  );
}

function ProbePanel({ srv, view, setView, openOverlay }) {
  const tokens = window.DATA.probe_tokens;
  const history = window.DATA.probe_history;

  return (
    <>
      {/* internal view selector */}
      <div style={{ display: "flex", border: "1px solid var(--border)", background: "var(--bg-1)" }}>
        {[
          { id: "tokens",  k: "T", l: `Tokens actifs (${tokens.length})` },
          { id: "history", k: "H", l: `History (${history.length})` },
          { id: "log",     k: "L", l: "Live log" },
        ].map(t => (
          <div key={t.id} onClick={() => setView(t.id)} style={{
            padding: "4px 14px",
            cursor: "pointer",
            color: view === t.id ? "var(--fg)" : "var(--fg-dim)",
            background: view === t.id ? "var(--bg-2)" : "transparent",
            borderRight: "1px solid var(--border)",
            display: "flex", gap: 8, alignItems: "center",
          }}>
            <span style={{ color: view === t.id ? "var(--magenta)" : "var(--fg-mute)", fontWeight: 700 }}>[{t.k}]</span>
            <span>{t.l}</span>
          </div>
        ))}
        <span style={{ flex: 1, borderBottom: "1px solid var(--border)" }} />
      </div>

      {view === "tokens" && (
        <Box title={`Tokens actifs (${tokens.length})`} right={<span className="dim">[n] générer · [q] QR · [x] révoquer</span>}
             style={{ flex: 1, minHeight: 0 }} bodyStyle={{ height: "100%", overflow: "auto" }}>
          <Table
            sel={0}
            cols={[
              { h: "TOKEN",  w: 1.6, cell: (r) => <span className="b-magenta">{r.token}</span> },
              { h: "LABEL",  w: 1.2, k: "label" },
              { h: "ISSUED", w: 1.4, k: "issued" },
              { h: "TTL",    w: 0.8, cell: (r) => {
                const m = Math.floor(r.ttl / 60), s = r.ttl % 60;
                const c = r.ttl < 60 ? "var(--yellow)" : "var(--fg)";
                return <span style={{ color: c, fontWeight: r.ttl < 60 ? 700 : 400 }}>{m > 0 ? `${m}m` : ""}{String(s).padStart(2, "0")}s</span>;
              }},
              { h: "STATE",  w: 0.8, cell: (r) => <span className="c-cyan">{r.state}</span> },
            ]}
            rows={tokens}
          />
        </Box>
      )}

      {view === "history" && (
        <Box title={`Fingerprints history (${history.length})`} right={<span className="dim">[d] détail · [↵] créer licence depuis…</span>}
             style={{ flex: 1, minHeight: 0 }} bodyStyle={{ height: "100%", overflow: "auto" }}>
          <Table
            sel={0}
            cols={[
              { h: "RECEIVED", w: 1.6, k: "received", cell: (r) => <span className="dim">{r.received}</span> },
              { h: "LABEL",    w: 1.1, k: "label" },
              { h: "HOSTNAME", w: 1.3, k: "hostname" },
              { h: "OS",       w: 1.0, k: "os" },
              { h: "LOCAL",    w: 1.2, cell: (r) => <span className="c-cyan">{r.local}</span> },
              { h: "USED IN",  w: 1.4, cell: (r) => r.used ? <span className="c-magenta">{r.used}</span> : <span className="mute">— unused —</span> },
            ]}
            rows={history}
          />
        </Box>
      )}

      {view === "log" && <LiveLog srv={srv} filter='Server=="probe"' />}

      <Box title="Astuce" style={{ height: 92 }}>
        <div style={{ padding: 12 }} className="dim">
          Le ProbeServer s'utilise surtout depuis le wizard licence → bindings → machine → « récupérer depuis une machine distante »
          (overlay). Mais tu peux le démarrer ici pour générer un batch de tokens (cas d'usage §10 Probe batch).
        </div>
      </Box>
    </>
  );
}

window.ServersScreen = ServersScreen;
