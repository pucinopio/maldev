// Terminal chrome: titlebar, tab strip, status bar, breadcrumb.

const TABS = [
  { k: "1", id: "dashboard",   label: "Dashboard"   },
  { k: "2", id: "licenses",    label: "Licenses"    },
  { k: "3", id: "issuers",     label: "Issuer keys" },
  { k: "4", id: "recipients",  label: "Recipients"  },
  { k: "5", id: "identities",  label: "Identities"  },
  { k: "6", id: "revocation",  label: "Revocation"  },
  { k: "7", id: "servers",     label: "Servers"     },
  { k: "8", id: "audit",       label: "Audit"       },
  { k: "9", id: "settings",    label: "Settings"    },
];
window.TABS = TABS;

function Titlebar({ db, server_on_count, online }) {
  return (
    <div style={{
      display: "flex", alignItems: "center", gap: 16,
      padding: "4px 14px",
      borderBottom: "1px solid var(--border)",
      background: "var(--bg-1)",
    }}>
      <span className="b-magenta">◆ license-manager</span>
      <span className="mute">v0.4.0-dev</span>
      <span style={{ flex: 1 }} />
      <span className="dim">db: <span style={{ color: "var(--fg)" }}>{db}</span></span>
      <span className="dim">net: <span style={{ color: online ? "var(--green)" : "var(--fg-mute)" }}>{online ? "online" : "offline"}</span></span>
      <span className="dim">http: <span style={{ color: server_on_count > 0 ? "var(--green)" : "var(--fg-mute)" }}>{server_on_count}/3 ON</span></span>
      <span className="dim">{new Date().toLocaleString("fr-FR", { hour12: false }).replace(",", "")}</span>
    </div>
  );
}

function TabStrip({ active, onSwitch }) {
  return (
    <div style={{
      display: "flex",
      borderBottom: "1px solid var(--border)",
      background: "var(--bg-1)",
      fontSize: 13,
    }}>
      {TABS.map(t => (
        <div
          key={t.id}
          className={"tab" + (active === t.id ? " active" : "")}
          onClick={() => onSwitch(t.id)}
          style={{ cursor: "pointer" }}
        >
          <span className="num">{t.k}</span>
          <span>{t.label}</span>
        </div>
      ))}
      <div style={{ flex: 1, borderRight: 0 }} />
    </div>
  );
}

function StatusBar({ hints, lastKey, message }) {
  return (
    <div style={{
      borderTop: "1px solid var(--border)",
      padding: "4px 12px",
      display: "flex", alignItems: "center", gap: 14,
      fontSize: 12,
      background: "var(--bg-1)",
      minHeight: 28,
    }}>
      <div style={{ flex: 1, display: "flex", flexWrap: "wrap", gap: 0 }}>
        {hints && hints.map((h, i) => <HK key={i} k={h.k} important={h.imp}>{h.t}</HK>)}
      </div>
      {message && (
        <span style={{ color: message.kind === "err" ? "var(--red)" : message.kind === "ok" ? "var(--green)" : "var(--cyan)" }}>
          {message.text}
        </span>
      )}
      {lastKey && (
        <span className="mute" style={{ marginLeft: 8 }}>
          last: <span className="glow-magenta">{lastKey}</span>
        </span>
      )}
    </div>
  );
}

function Crumb({ items }) {
  return (
    <div className="crumb" style={{ padding: "6px 14px", fontSize: 12, borderBottom: "1px solid var(--border)", background: "var(--bg-1)" }}>
      {items.map((it, i) => (
        <React.Fragment key={i}>
          {i > 0 && <span className="mute"> ▸ </span>}
          <span className={i === items.length - 1 ? "here" : ""}>{it}</span>
        </React.Fragment>
      ))}
    </div>
  );
}

Object.assign(window, { Titlebar, TabStrip, StatusBar, Crumb });
