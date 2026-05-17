# Technique-page template

Skeleton every page under `docs/techniques/<domain>/<technique>.md`
must follow. Sections are in fixed order. Optional sections are
marked.

The CI runs `internal/tools/docgen --check-template`; pages that
diverge structurally are blocked.

```markdown
---
package: github.com/oioio-space/maldev/<path>
mitre: T1XXX[, T1YYY]
---

# <Human-readable title (FR or EN, accessible vocabulary)>

[← <domain> index](README.md) · [docs/index](../../index.md)

> **TL;DR** — one sentence. Concrete. Names the technique.

## What it does

2-4 paragraphs vulgarised. Explain:
- the defensive control or attack surface being targeted,
- why the operator wants this capability,
- the cost (artefacts, complexity, scope).

Use `> [!IMPORTANT]` or `> [!WARNING]` callouts for hard scope
constraints (per-process only, requires admin, Win11-only, …).

## How it works

The mechanism that matters. Mermaid OK **only** if it shows real
ordering, decision tree, or sequence — not a 5-box fan-out
paraphrasing the prose. Max 1 diagram per page (rare exceptions
documented; see `docs/architecture.md`).

## Usage

A single, minimal, real Go snippet. Show the imports. If multiple
modes exist (e.g. basic vs composed), give one snippet per mode,
max 3 total.

```go
import "github.com/oioio-space/maldev/<path>"

caller, _ := wsyscall.New(wsyscall.MethodIndirect)
if err := pkg.DoThing(caller); err != nil {
    return fmt.Errorf("DoThing: %w", err)
}
```

## Non-obvious behaviour

Bullet list of pitfalls, side effects, dependencies that godoc
doesn't surface clearly:
- "caller == nil falls back to WinAPI for debug only".
- "Idempotent — safe to re-invoke".
- "Mutates `amsi.dll` .text section in current process".

## OPSEC & detection

Table: artefact ↔ where defenders look. Cross-reference D3FEND
counters. If applicable, hardening notes (Win11 mitigations etc.).

## MITRE ATT&CK

| T-ID | Name | Sub-coverage |
|---|---|---|
| [T1XXX](https://attack.mitre.org/techniques/T1XXX/) | … | … |

## Limitations

Bullet list of known broken cases / not-yet-supported axes.

## API → godoc

[`pkg.go.dev/github.com/oioio-space/maldev/<path>`](https://pkg.go.dev/...) is
the authoritative reference for every exported symbol. This page
teaches the *concepts*; the godoc is the *specification*.

## See also (optional)

- `[<sibling package>](other-technique.md)` — for what.
- Cookbook entry: `../../examples/<recipe>.md`.
- External references: papers, blog posts, original PoCs.
```

## Rules enforced by `docgen --check-template`

1. **No `## API Reference` section.** That pattern duplicates godoc
   and is the dominant drift surface. Use `## API → godoc` instead.
2. **No `last_reviewed:` / `reflects_commit:` in frontmatter.**
   `git log` is the authoritative "last touched" record; the
   frontmatter rotted silently across 100+ pages before being
   removed in G.6.
3. **Every page has `## API → godoc`** (or no API section at all,
   for pure cross-reference pages).
4. **Every page declares `mitre:` in frontmatter** when it covers a
   technique mapped to ATT&CK (cross-ref / index pages may omit).

## What a "concept page" omits

Cross-reference index pages (`README.md` per domain, the synthetic
`docs/techniques/collection/lsass-dump.md` cross-ref, etc.) MAY
omit `## API → godoc` if they don't expose API directly. The
checker tolerates this when `package:` is missing from frontmatter.

## Adding a new technique

1. Copy this template skeleton into
   `docs/techniques/<domain>/<technique>.md`.
2. Fill the frontmatter (`package`, `mitre`) and every section.
3. Add the entry to the corresponding `docs/techniques/<domain>/README.md`
   index table.
4. Run `go run ./internal/tools/docgen` to refresh
   `docs/index.md`.
5. Run `python3 scripts/check-doc-links.py` to verify no broken
   cross-links.
6. Verify locally with `mdbook build` if the toolchain is
   available.

The pre-commit hook in `scripts/pre-commit` runs the docgen drift
check; CI runs both the drift check and the strict link check.
