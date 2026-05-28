# Concepts — index

Conceptual pages that explain ONE notion each. Linked from
[`concepts.md`](../concepts.md) (the architecture overview) and
from the runnable examples that exercise the notion.

| Page | Notion |
|---|---|
| [issuer.md](./issuer.md) | Ed25519 signing keys, active vs retired, key-id routing |
| [bindings.md](./bindings.md) | machine / password / TOTP / custom evidence + AND semantics |
| [crl.md](./crl.md) | Signed revocation list, freshness invariants, cache downgrade defence |
| [audit-chain.md](./audit-chain.md) | Atomic audit-with-mutation, immutable trail |
| [kek-passphrase.md](./kek-passphrase.md) | Argon2id → KEK → ChaCha20-Poly1305 column wrapping, passphrase cascade |

Coming:

- `argon-preset.md` — the fast / default / paranoid tuning curve
  for password bindings.
- `sealed-payload.md` — X25519 sealed boxes for per-recipient
  payload encryption.
- `identity-pin.md` — host SHA-256 identity pinning vs machine
  bindings.

These three exist on the [main concepts index](../concepts.md)
with placeholder cross-links; the dedicated pages land alongside
the matching examples in the next batch.
