// Fake fixture data so screens look populated.
window.DATA = {
  operator_name: "operator",
  active_key: { keyid: "k2026-04", name: "rshell-prod-2026Q2", fingerprint: "ed25519:a4f2…91bc" },
  issuer_default: "research@offsec.local",
  audience_defaults: ["rshell", "rshell-edu"],
  counters: { active: 47, revoked: 6, expired: 12, expiring_7d: 4, superseded: 9 },
  servers: [
    { id: "revoc",     name: "revocation",        port: 8443, on: true,  reqs: 1289, uptime: "2h 41m", url: "https://manager.local:8443" },
    { id: "heartbeat", name: "heartbeat",         port: 8444, on: true,  reqs: 5132, uptime: "2h 41m", url: "https://manager.local:8444" },
    { id: "probe",     name: "fingerprint probe", port: 8445, on: false, reqs: 0,    uptime: "—",      url: null },
  ],
  audit: [
    { t: "13:42:18", kind: "license.issue",  actor: "operator", target: "lic:9f3a-… (alice@research)",         note: "k2026-04, +machine +totp" },
    { t: "13:38:02", kind: "key.activate",   actor: "operator", target: "k2026-04",                            note: "" },
    { t: "13:22:55", kind: "license.revoke", actor: "operator", target: "lic:71bd-… (bob@research)",           note: "reason: key_compromised" },
    { t: "11:09:11", kind: "server.start",   actor: "operator", target: "revocation :8443",                    note: "" },
    { t: "10:58:00", kind: "identity.new",   actor: "operator", target: "rshell-windows-amd64.bin",            note: "sha256:8b…2e1" },
  ],
  audit_long: null, // populated below
  issuer_keys: [
    { keyid: "k2026-04", name: "rshell-prod-2026Q2", status: "active",  created: "2026-04-01", signed: 47, fpr: "ed25519:a4f2…91bc" },
    { keyid: "k2026-01", name: "rshell-prod-2026Q1", status: "retired", created: "2026-01-04", signed: 138, fpr: "ed25519:6c81…ab02" },
    { keyid: "k2025-10", name: "rshell-prod-2025Q4", status: "retired", created: "2025-10-02", signed: 211, fpr: "ed25519:1209…fe48" },
    { keyid: "k2025-07", name: "rshell-prod-2025Q3", status: "retired", created: "2025-07-08", signed: 96,  fpr: "ed25519:b330…7e21" },
  ],
  recipient_keys: [
    { keyid: "r2026-01", name: "default-recipient",  created: "2026-04-01", sealed: 12, fpr: "x25519:7a90…003c" },
    { keyid: "r-emerg",  name: "emergency-recovery", created: "2025-11-12", sealed: 1,  fpr: "x25519:0044…bbf2" },
  ],
  identities: [
    { name: "rshell-windows-amd64.bin", sha: "8b3c91ad…2e1", refs: 22, created: "2026-04-12" },
    { name: "rshell-linux-amd64.bin",   sha: "01ffa2d8…7c4", refs: 18, created: "2026-04-12" },
    { name: "rshell-darwin-arm64.bin",  sha: "9c2a55ff…b88", refs: 7,  created: "2026-04-13" },
    { name: "agent-stub.bin",           sha: "31aa4400…f02", refs: 0,  created: "2026-04-28" },
  ],
  licenses: [
    { id: "lic-alice-9f3a",   status: "active",     subj: "alice@research",  iss: "research@offsec.local", aud: "rshell",     keyid: "k2026-04", exp: "2026-08-14", features: ["scan","report","exec"],         parent: null,             successors: [] },
    { id: "lic-bob-71bd",     status: "active",     subj: "bob@research",    iss: "research@offsec.local", aud: "rshell",     keyid: "k2026-04", exp: "2026-07-02", features: ["scan","report"],                parent: null,             successors: [] },
    { id: "lic-carol-3a51",   status: "expiring",   subj: "carol@research",  iss: "research@offsec.local", aud: "rshell-edu", keyid: "k2026-04", exp: "2026-05-24", features: ["scan"],                         parent: null,             successors: [] },
    { id: "lic-demo-c021",    status: "active",     subj: "demo@external",   iss: "research@offsec.local", aud: "rshell-edu", keyid: "k2026-04", exp: "2026-06-01", features: ["scan","report"],                parent: null,             successors: [] },
    { id: "lic-lab04-7700",   status: "expiring",   subj: "lab-04",          iss: "research@offsec.local", aud: "rshell",     keyid: "k2026-04", exp: "2026-05-23", features: ["scan","exec"],                  parent: null,             successors: [] },
    { id: "lic-intern-0033",  status: "revoked",    subj: "ex-intern",       iss: "research@offsec.local", aud: "rshell",     keyid: "k2026-01", exp: "2026-09-12", features: ["scan"],                         parent: null,             successors: [] },
    { id: "lic-rshell-aabc",  status: "expired",    subj: "rshell-demo-v3",  iss: "research@offsec.local", aud: "rshell",     keyid: "k2026-01", exp: "2026-04-30", features: ["scan","report","exec"],         parent: null,             successors: ["lic-rshell-d8f1"] },
    { id: "lic-rshell-d8f1",  status: "active",     subj: "rshell-demo-v3",  iss: "research@offsec.local", aud: "rshell",     keyid: "k2026-04", exp: "2026-10-30", features: ["scan","report","exec"],         parent: "lic-rshell-aabc", successors: [] },
    { id: "lic-evg-1bc2",     status: "active",     subj: "evgeny@partner",  iss: "research@offsec.local", aud: "rshell",     keyid: "k2026-04", exp: "2027-04-12", features: ["scan","report","exec","beacon"],parent: null,             successors: [] },
    { id: "lic-qa-51aa",      status: "active",     subj: "qa-rig-01",       iss: "research@offsec.local", aud: "rshell",     keyid: "k2026-04", exp: "2026-12-01", features: ["scan","exec"],                  parent: null,             successors: [] },
    { id: "lic-leaked-88af",  status: "revoked",    subj: "leaked-key-2025", iss: "research@offsec.local", aud: "rshell",     keyid: "k2025-10", exp: "2026-01-01", features: ["scan"],                         parent: null,             successors: [] },
    { id: "lic-superseded-1", status: "superseded", subj: "alice@research",  iss: "research@offsec.local", aud: "rshell",     keyid: "k2026-01", exp: "2026-05-15", features: ["scan","report"],                parent: null,             successors: ["lic-alice-9f3a"] },
  ],
  revocations: [
    { lic: "lic:71bd-… (bob@research)",   keyid: "k2026-04", at: "2026-05-20 13:22", reason: "key_compromised" },
    { lic: "lic:0033-… (ex-intern)",      keyid: "k2026-01", at: "2026-05-04 09:11", reason: "offboarding"     },
    { lic: "lic:88af-… (leaked-key-2025)",keyid: "k2025-10", at: "2026-01-02 18:40", reason: "leak"            },
    { lic: "lic:51aa-… (qa-rig-03)",      keyid: "k2026-01", at: "2026-03-19 11:02", reason: "decommissioned"  },
  ],
  probe_tokens: [
    { token: "tk_aB3xZ9mLqP21vR", label: "alice-laptop",   ttl: 47,   issued: "2026-05-20 13:42", state: "waiting" },
    { token: "tk_pQ7nT3wXyZ80kE", label: "carol-vm-prod",  ttl: 142,  issued: "2026-05-20 13:39", state: "waiting" },
    { token: "tk_xL9mN5sB2vC44d", label: "lab-rig-02",     ttl: 1820, issued: "2026-05-20 13:11", state: "waiting" },
  ],
  probe_history: [
    { token: "tk_oldD42xQ", label: "alice-laptop", hostname: "laptop-alice", os: "linux/amd64", cpu: "AMD Ryzen 7 PRO 7840U", local: "0c8a91…f4d2", composite: "7c91aa…1208", received: "2026-05-20 13:09:11", used: "lic-alice-9f3a" },
    { token: "tk_oldR91vQ", label: "bob-mac",      hostname: "MBP-bob",      os: "darwin/arm64", cpu: "Apple M3 Pro",         local: "5e21fa…0b91", composite: "a302bf…ce04", received: "2026-05-19 17:28:55", used: "lic-bob-71bd" },
    { token: "tk_oldQ12hP", label: "rig-03",       hostname: "rig-03",       os: "linux/amd64",  cpu: "Intel Xeon E5-2680",   local: "aa18cc…211d", composite: "fe902a…8801", received: "2026-05-19 11:02:01", used: null },
    { token: "tk_oldZ77nM", label: "carol-vm",     hostname: "carol-vm",     os: "linux/amd64",  cpu: "QEMU virtual",         local: "31ee00…4422", composite: "881a3c…0e7e", received: "2026-05-18 14:55:30", used: "lic-carol-3a51" },
    { token: "tk_oldU03kL", label: "lab-04",       hostname: "lab-04",       os: "windows/amd64",cpu: "Intel i9-14900K",      local: "029ab1…cc11", composite: "f00102…aabb", received: "2026-05-18 09:20:00", used: "lic-lab04-7700" },
  ],
  settings: {
    default_issuer_name: "research@offsec.local",
    default_audience: ["rshell", "rshell-edu"],
    default_ttl_seconds: 90 * 86400,
    default_argon_preset: "default",
    default_keyid: "active",
    operator_name: "operator",
    auto_start_servers: false,
    confirm_quit_with_servers: true,
    db_path: "~/.config/license-manager/db.sqlite",
    passphrase_resolved_via: "MALDEV_MGR_PASSPHRASE_FILE",
  },
};

// long audit list
window.DATA.audit_long = (() => {
  const kinds = ["license.issue","license.reissue","license.revoke","key.activate","key.generate","server.start","server.stop","identity.new","crl.export","probe.token","probe.received","settings.rekey"];
  const out = [];
  let h = 13, m = 42;
  for (let i = 0; i < 28; i++) {
    out.push({
      id: 1000 + i,
      t: `2026-05-20 ${String(h).padStart(2,"0")}:${String(m).padStart(2,"0")}:${String(11 + (i*7)%48).padStart(2,"0")}`,
      kind: kinds[i % kinds.length],
      actor: "operator",
      target_kind: i % 3 === 0 ? "License" : i % 3 === 1 ? "Issuer" : "Identity",
      target: `lic:${(0x9f3a + i*131).toString(16)}-…`,
      note: i % 3 === 0 ? "k2026-04" : "",
    });
    m -= 7; if (m < 0) { m += 60; h -= 1; if (h < 0) h = 23; }
  }
  return out;
})();
