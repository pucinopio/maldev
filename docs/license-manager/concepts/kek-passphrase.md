---
package: github.com/oioio-space/maldev/internal/manager/crypto
---

# KEK & passphrase cascade

> The operator's passphrase NEVER touches the database. It is
> derived (Argon2id) into a 32-byte **KEK** (Key Encryption Key)
> that lives in process RAM, used to wrap and unwrap sensitive
> columns via ChaCha20-Poly1305 AEAD. The KEK is zeroed on a
> clean shutdown.

## Why two layers

The passphrase is a human secret; the KEK is a machine secret.
Separating them lets the manager:

- Use a slow, memory-hard derivation (Argon2id) **once** at
  startup instead of on every column-decrypt operation.
- Rekey the DB by re-wrapping every column under a new KEK,
  without changing the passphrase — and vice versa.
- Zero the KEK from RAM on exit so a post-mortem memory dump
  doesn't yield the wrapping key.

## Derivation

```
passphrase + kek_salt (16 bytes, plaintext in Setting)
    → Argon2id(time=3, memory=64 MiB, threads=4, keylen=32)
    → KEK (32 bytes, in RAM only)
```

`Setting.kek_salt` is stored in plaintext on purpose — its only
job is to prevent two managers with the same passphrase from
landing on the same KEK. It is unique-per-DB and immutable.

## Column wrapping

```
[12-byte random nonce] || [ciphertext] || [16-byte AEAD tag]
```

ChaCha20-Poly1305 with a fresh random nonce per column. Catastrophic
if a nonce is reused (key recovery), so every wrap reads from
`crypto/rand.Reader`.

Wrapped columns:

| Table | Column | Contents |
|---|---|---|
| `Issuer` | `encrypted_priv` | Ed25519 private key (64 bytes) |
| `RecipientKey` | `encrypted_priv` | X25519 private key (32 bytes) |
| `TOTPSecret` | `encrypted_secret` | TOTP secret (base32) |
| `ServerConfig` | `revocation_admin_token_enc` | Revocation server admin token |

Everything else is plaintext. The reasoning: anything that can
be reconstructed from issued licences (subjects, audiences,
features) does not need wrapping, and wrapping it would add
latency to every query without changing the threat model.

## Canary

`Setting.kek_canary = KEK.Wrap(random32)` is written at DB
creation. On every startup the manager attempts
`KEK.Unwrap(canary)`:

- Success → the passphrase + salt produced the right KEK; carry on.
- Failure (AEAD tag mismatch) → wrong passphrase; the manager
  prompts again, three attempts then exit.

This is **the** authentication check — the rest of the DB schema
doesn't care whether the KEK is right because the wrapped columns
are only unwrapped on demand.

## Passphrase cascade at boot

The manager resolves the passphrase in strict order; the first
non-empty source wins:

```
1. flag --passphrase-file <path>   → read file, trim whitespace
2. env MALDEV_MGR_PASSPHRASE_FILE  → read the file named by the var
3. env MALDEV_MGR_PASSPHRASE       → direct value
4. (v2) OS keystore (DPAPI / Keychain / libsecret)
5. fallback: interactive TUI prompt (masked modal)
```

CI scripts plug into step 1 or 2; interactive operators land on
step 5. The TUI's Settings screen surfaces which step actually
resolved (read-only "Cette session a résolu via : …" line) so
the operator can audit that automation isn't accidentally
falling back to the prompt in headless runs.

## Rekey (ChangePassphrase)

[`SettingsService.ChangePassphrase`](../../../internal/manager/service/settings.go)
runs the rekey in a single SQLite transaction:

1. Verify the old passphrase via the canary.
2. Derive a NEW KEK from the new passphrase + a fresh
   `kek_salt`.
3. For every wrapped column: unwrap with old KEK, wrap with new
   KEK, write back.
4. Update `Setting.kek_salt` + `Setting.kek_canary`.
5. Replace the in-memory KEK + `.Wipe()` the old one.

A crash mid-rekey leaves the DB unchanged (transaction rolls
back); a crash after commit but before the in-memory swap
leaves the next boot with the new passphrase.

## Tested in

- [`examples/license-manager/01-issue-basic/`](../../../examples/license-manager/01-issue-basic/)
  and every other example boots a fresh in-memory store, derives
  a KEK from `"demo"`, and proves the canary check passes —
  exactly the path a real boot follows minus the passphrase
  prompt.
- [`examples/license-manager/09-import-and-verify/`](../../../examples/license-manager/09-import-and-verify/)
  proves a licence verifies without the KEK at all (verify only
  needs the issuer's PUBLIC key — the KEK protects writes, not
  reads of the licence PEM itself).
