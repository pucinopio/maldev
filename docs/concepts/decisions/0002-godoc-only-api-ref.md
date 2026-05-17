# ADR-0002 — godoc-only API reference

**Status:** accepted
**Date:** 2026-05-17

## Context

Every per-technique page under `docs/techniques/` used to carry a
hand-written `## API Reference` section with one block per
exported symbol:

```markdown
### `Foo(arg Type) (Result, error)`

[godoc](https://pkg.go.dev/...)

Description.

**Parameters:** ...
**Returns:** ...
**Side effects:** ...
```

Over 100 pages followed this pattern. The shape *recopied* godoc's
content. Every Go rename, signature change, or new parameter
silently invalidated the prose without anyone noticing.

A 2026-05-17 audit found ~9500 lines of these tables across 81
technique pages.

## Decision

We will **not** maintain a handwritten API reference in the docs.
Every technique page ends with:

```markdown
## API → godoc

[`pkg.go.dev/<path>`](https://pkg.go.dev/<path>) is the authoritative
reference for every exported symbol. This page teaches the
*concepts*; the godoc is the *specification*.
```

The CI gate `internal/tools/docgen --check-template` rejects any
PR that re-introduces a `## API Reference` section.

## Consequences

- **Positive.** Go-side renames no longer invalidate doc pages.
  Drift is structurally impossible.
- **Positive.** Pages are leaner. Readers see *concepts* + *non-
  obvious behaviour*, not duplicated signatures.
- **Positive.** godoc gets the love it deserves — contributors
  write better godoc comments knowing the doc site links straight
  to them.
- **Negative.** Readers who want a one-page printable cheat sheet
  per package don't get it from the doc site (pkg.go.dev is fine
  for browsing but doesn't print well).
- **Negative.** Some context that only lived in the prose
  "Parameters:" lines (e.g., "this RVA must be 16-aligned") needs
  to be re-anchored in the godoc itself or in the page's
  "Non-obvious behaviour" section.

## Alternatives considered

- **Auto-generate the table from `go doc -json`.** Considered for
  a Docusaurus migration (would render a React component). Adds
  toolchain complexity. Postponed.
- **Just write a script that pre-validates handwritten tables
  against godoc.** Possible but every contributor would have to
  remember to run it; drift would still happen on small renames.
- **Mintlify-style "API tabs"** — too heavy for mdBook.

## References

- Doc-refonte plan, phases G.5 and G.6 (commits `85809d7`, `49e7401`).
- `docs/conventions/documentation.md` § "Per-technique pages".
- `docs/templates/technique-page.md`.
