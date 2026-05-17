# ADR-0003 — mdBook over Docusaurus (for now)

**Status:** accepted
**Date:** 2026-05-17

## Context

The doc site is currently mdBook (Rust-based static-site generator
that consumes plain Markdown). The doc refonte raised the question:
should we migrate to Docusaurus (React-based, more powerful,
Algolia search, MDX components, versioning, i18n)?

The two main pain points that motivated the question:

1. **API ref drift** — pkg.go.dev links are fine but we wanted
   inline auto-generated API tables. Docusaurus could do this via
   a React component over `go doc -json` output.
2. **Search** — mdBook's local search is regex-ish; Algolia
   DocSearch (free for OSS) is dramatically better for 100+ pages.

## Decision

We stay on mdBook for now. Migration deferred to a separate
milestone (probably v0.160+) and only if one of these triggers:

- We genuinely want Algolia search after living with the
  restructure.
- We need a real auto-gen-API-from-godoc component.
- We start an FR translation effort.

ADR-0002 (godoc-only API ref) already removes the principal
drift surface without requiring Docusaurus, so the urgency is
lower than it looked initially.

## Consequences

- **Positive.** Build stays fast (<1 s for the whole book), no
  Node.js / `node_modules` / Webpack toolchain.
- **Positive.** Hackable: every contributor knows Markdown,
  none have to know React.
- **Positive.** mdBook ships a single binary; CI workflow is
  trivial.
- **Negative.** Search is weaker than Algolia.
- **Negative.** No interactive components (tabs, code playground,
  embedded React).
- **Negative.** Theming is more limited; the default theme is
  spartan.

## Alternatives considered

- **Docusaurus full migration.** ~7-11 days of work. Adds React/
  Webpack vendor-lock and a 10-30 s build for marginal current-day
  gain. Postponed.
- **Hugo / mkdocs-material.** Considered briefly. Hugo would
  require rewriting layouts; mkdocs-material is excellent but
  Python-toolchain-heavy. mdBook already works.
- **Doxygen / Sphinx.** Not idiomatic for Go projects.

## References

- Brainstorm conversation 2026-05-17 (doc refonte plan, section H
  comparison table).
- mdBook docs: https://rust-lang.github.io/mdBook/
- Docusaurus docs: https://docusaurus.io/
