// Package manager houses the local backend for the license-manager TUI.
//
// Layers:
//
//   crypto/   - passphrase-derived KEK + ChaCha20-Poly1305 wrap/unwrap
//   store/    - ENT-backed SQLite store
//   service/  - domain services orchestrating store + audit + tx
//   httpsrv/  - lifecycle-managed HTTP servers (revocation / heartbeat / probe)
//   probe/    - embedded per-OS agent binaries for remote fingerprinting
//
// The TUI layer (cmd/license-manager) consumes *service.Services and never
// touches store or crypto directly.
package manager
