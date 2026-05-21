// Shared primitives — fidèles à ce que produit lipgloss/bubbles.
// PAS de text-shadow, box-shadow, gradient, transition, font-size variable.
// Seules animations conservées : caret blink (bubbles/cursor) + spinner (bubbles/spinner).
const { useState, useEffect, useRef, useMemo, useCallback } = React;

function Box({ title, focused, right, children, style, bodyStyle, kind }) {
  const cls = "box" + (focused ? " focused-box" : "");
  return (
    <div className={cls} style={style}>
      {title !== undefined && (
        <div className={"box-title" + (focused ? " focused" : "")}>
          <span>{title}</span>
          {right && <span style={{ color: "var(--fg-dim)", fontWeight: 400 }}>{right}</span>}
        </div>
      )}
      <div style={bodyStyle}>{children}</div>
    </div>
  );
}

function HK({ k, children, important }) {
  return (
    <span className="hk">
      <span className="k">{k}</span>{" "}
      <span style={{ color: "var(--fg-dim)" }}>{children}</span>
    </span>
  );
}

function Dot({ kind }) {
  return <span className={"dot " + (kind || "dim")} />;
}

// Status pill — bordure colorée, bold. Pas de shadow.
function StatusPill({ status }) {
  const map = {
    active:     { c: "var(--green)",   l: "ACTIVE"     },
    expiring:   { c: "var(--yellow)",  l: "EXPIRING"   },
    expired:    { c: "var(--fg-mute)", l: "EXPIRED"    },
    revoked:    { c: "var(--red)",     l: "REVOKED"    },
    superseded: { c: "var(--violet)",  l: "SUPERSEDED" },
    retired:    { c: "var(--fg-mute)", l: "RETIRED"    },
    on:         { c: "var(--green)",   l: "ON"         },
    off:        { c: "var(--fg-mute)", l: "OFF"        },
  };
  const m = map[status] || { c: "var(--fg-dim)", l: status?.toUpperCase() };
  return (
    <span style={{
      display: "inline-block", padding: "0 6px",
      color: m.c, border: `1px solid ${m.c}`,
      fontWeight: 700,
      minWidth: Math.max(7 * (m.l?.length || 4), 28)
    }}>{m.l}</span>
  );
}

// Table flat — pas d'inline expand-row, l'écran appelle <TableDetailPane> sous la table.
function Table({ cols, rows, sel, emptyText }) {
  return (
    <div>
      <div className="row" style={{
        color: "var(--cyan)", fontWeight: 700,
        borderBottom: "1px solid var(--border)", padding: "4px 12px"
      }}>
        {cols.map((c, i) => (
          <div key={i} style={{ flex: (c.w || 1), textAlign: c.align || "left", minWidth: 0 }}>{c.h}</div>
        ))}
      </div>
      {rows.length === 0 && (
        <div className="dim" style={{ padding: "16px 12px", textAlign: "center" }}>{emptyText || "— vide —"}</div>
      )}
      {rows.map((r, i) => (
        <div key={i} className={"row" + (sel === i ? " selected" : "")}>
          {cols.map((c, j) => (
            <div key={j} style={{ flex: (c.w || 1), textAlign: c.align || "left", minWidth: 0, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
              {c.cell ? c.cell(r, i) : r[c.k]}
            </div>
          ))}
        </div>
      ))}
    </div>
  );
}

// Tile — compteurs Dashboard. Pas de font-size>14 : terminal = cellule monospace fixe.
// Le poids visuel vient de bold + couleur saturée + un caractère de garde plus large.
function Tile({ k, label, value, kind, footer }) {
  const color = kind === "danger" ? "var(--red)" : kind === "warn" ? "var(--yellow)" : kind === "good" ? "var(--green)" : "var(--cyan)";
  return (
    <div className="box">
      <div style={{
        padding: "3px 12px",
        borderBottom: "1px solid var(--border)",
        display: "flex", justifyContent: "space-between",
        color: "var(--fg-dim)",
      }}>
        <span>{label}</span>
        {k !== undefined && <span style={{ color: "var(--magenta)", fontWeight: 700 }}>{`[${k}]`}</span>}
      </div>
      <div style={{ padding: "8px 12px" }}>
        <div style={{ color, fontWeight: 700 }}>
          <span style={{ display: "inline-block", minWidth: "5ch" }}>{value}</span>
        </div>
        {footer && <div className="dim" style={{ marginTop: 2 }}>{footer}</div>}
      </div>
    </div>
  );
}

// Hr — séparateur 1 ligne
function Rule({ color = "var(--border)" }) {
  return <div style={{ borderTop: `1px solid ${color}`, margin: "6px 0" }} />;
}

// QR ASCII (qrterminal-style)
function AsciiQR({ size = 25 }) {
  const lines = useMemo(() => {
    let s = 1;
    const rnd = () => { s = (s * 9301 + 49297) % 233280; return s / 233280; };
    const matrix = [];
    for (let y = 0; y < size; y++) {
      let row = "";
      for (let x = 0; x < size; x++) {
        const inFinder = (
          (x < 7 && y < 7) ||
          (x >= size - 7 && y < 7) ||
          (x < 7 && y >= size - 7)
        );
        let on;
        if (inFinder) {
          const fx = x < 7 ? x : size - 1 - x;
          const fy = y < 7 ? y : size - 1 - y;
          on = (fx === 0 || fx === 6 || fy === 0 || fy === 6) || (fx >= 2 && fx <= 4 && fy >= 2 && fy <= 4);
        } else {
          on = rnd() > 0.55;
        }
        row += on ? "██" : "  ";
      }
      matrix.push(row);
    }
    return matrix;
  }, [size]);
  return <div className="ascii qr">{lines.join("\n")}</div>;
}

// Input — bubbles/textinput look-alike. Underline coloré sur focus, pas de shadow.
function Input({ label, value, hint, focused, masked, error, suffix, width }) {
  const v = masked ? "•".repeat((value || "").length) : (value || "");
  return (
    <div style={{ marginBottom: 8, width: width || "100%" }}>
      <div style={{ color: focused ? "var(--magenta)" : "var(--fg-dim)", fontWeight: focused ? 700 : 400, marginBottom: 1 }}>
        {label}{focused && <span className="mute" style={{ marginLeft: 8, fontWeight: 400 }}>{hint}</span>}
      </div>
      <div style={{
        borderBottom: `1px solid ${focused ? "var(--magenta)" : error ? "var(--red)" : "var(--border)"}`,
        padding: "2px 0",
        display: "flex", alignItems: "center", gap: 8,
      }}>
        <span style={{ color: v ? "var(--fg)" : "var(--fg-mute)", flex: 1 }}>
          {v || <span className="mute">— vide —</span>}
          {focused && <span className="caret">&nbsp;</span>}
        </span>
        {suffix && <span className="dim">{suffix}</span>}
      </div>
      {error && <div style={{ color: "var(--red)", marginTop: 2 }}>{error}</div>}
    </div>
  );
}

// Button — bordure colorée, bold focus. Pas de shadow ni de transition.
function Btn({ label, hot, kind, focused, onClick }) {
  const color = kind === "danger" ? "var(--red)" : kind === "primary" ? "var(--magenta)" : "var(--cyan)";
  return (
    <button
      onClick={onClick}
      style={{
        background: focused ? "var(--bg-2)" : "transparent",
        border: `1px solid ${color}`,
        color,
        padding: "2px 12px",
        fontFamily: "inherit",
        fontSize: "inherit",
        cursor: "pointer",
        fontWeight: 700,
      }}
    >
      {hot && <span style={{ marginRight: 8, opacity: 0.8 }}>[{hot}]</span>}
      {label}
    </button>
  );
}

Object.assign(window, { Box, HK, Dot, StatusPill, Table, Tile, Rule, AsciiQR, Input, Btn });
