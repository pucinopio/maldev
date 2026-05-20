// Package license provides a defensive framing primitive for maldev research
// binaries: signed, structured license tokens that constrain who may run a
// given binary, on which machines, with which secrets, until when, and against
// which revocation/heartbeat policy.
//
// Technique: License framing for authorized maldev tooling.
// MITRE ATT&CK: N/A (defensive primitive; no offensive technique mapped).
// Detection level: N/A (no on-host artefacts emitted; consult docs/techniques/license-framing.md).
//
// Threat model (summary; see docs/license/threat-model.md for the full version):
//
//   Resists: license forgery, post-issuance tampering, replay across audiences,
//   cross-binary reuse, stale-cache substitution, brute-force on password
//   bindings, clock rollback below trusted-floor, algorithm-confusion attacks
//   on the signature.
//
//   Does NOT resist: an attacker who patches Verify in the binary; permanent
//   offline use beyond grace period; perfect clock tamper without TPM; binary
//   modification combined with identity-bytes modification; hostid spoofing on
//   a machine the attacker controls fully.
//
// Composition: the root package exposes Issue/Verify/GenerateKey and the
// option set. Sub-packages provide optional features (revocation, heartbeat,
// identity, sealed payload, NTP). A consumer that only needs offline
// verification imports only the root package and pulls no sub-package code.
package license
