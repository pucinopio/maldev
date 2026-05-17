# ADR-0004 — Diátaxis-pragmatic, not Diátaxis-pure

**Status:** accepted
**Date:** 2026-05-17

## Context

The doc refonte adopted [Diátaxis](https://diataxis.fr) as the
top-level information architecture. Diátaxis prescribes 4 distinct
content quadrants:

1. **Tutorials** — learning-oriented.
2. **How-to guides** — task-oriented.
3. **Reference** — information-oriented.
4. **Explanation** — understanding-oriented.

The strict reading of Diátaxis is that **each page belongs to
exactly one quadrant** and **the writing style differs per
quadrant** (verbose for tutorials, austere for reference, etc.).

Applied to maldev's 112 technique pages, strict Diátaxis would
mean splitting each technique into 4 pages (1 per quadrant) — ≈ 450
pages total. That's:

- 4× the maintenance.
- Disperses the technique-narrative that currently keeps pages
  readable.
- Disconnected from what real-world reference Diátaxis adopters
  actually do (Django, Cargo, Kubernetes, Cobra, Stripe — all
  keep mixed-quadrant per-package pages).

## Decision

We will use Diátaxis as **navigation taxonomy** (top-level sections
labelled by quadrant) but allow **pragmatic mixing inside individual
pages**.

- Top-level sections: Get started / Cookbook / Techniques / Tooling
  / Concepts / Contributing.
- Each technique page may mix quadrants: prose explanation (`How
  it works`, `OPSEC`) + brief how-to (`Usage`) + reference pointer
  (`API → godoc`).
- The fix gate is *which dominant section the page lives in*, not
  that the page is single-quadrant pure.

## Consequences

- **Positive.** Readers get a clear navigation map without each
  technique splitting into 4 pages.
- **Positive.** Aligns with the "pragmatic Diátaxis" pattern
  Diátaxis itself acknowledges in
  ["How Diátaxis is mis-used"](https://diataxis.fr/diataxis-mistakes/).
- **Positive.** Lower migration cost — existing pages weren't
  shredded into 4 quadrants.
- **Negative.** Strict Diátaxis purists won't be happy.
- **Negative.** Without the discipline of pure quadrants, page
  voices can drift (a Reference page accidentally adopts
  Tutorial hand-holding tone).

## Anti-Diátaxis acknowledged

- `getting-started.md` (Quick start) is Tutorial-leaning but short
  — a real Tutorial in Diátaxis sense is
  `get-started/first-payload.md` (added 2026-05-17).
- `mitre.md` is **Reference** content shelved under Concepts (it's
  a lookup table, not Explanation). Could move to a Reference
  sub-section if the misclassification becomes a real reader
  problem.
- The 3 "Guides by role" under Concepts mix all 4 quadrants
  intentionally — they're audience-bridging documents, not
  Diátaxis-pure content.

## Alternatives considered

- **Strict Diátaxis** — 4-way page split per technique. Rejected
  for cost + dispersion (see Context).
- **Drop Diátaxis altogether** and use a custom IA. Rejected:
  Diátaxis labels are recognisable to anyone who reads docs, even
  imperfectly applied.

## References

- https://diataxis.fr — the framework.
- https://diataxis.fr/diataxis-mistakes/ — Daniele Procida himself
  notes pragmatic adoption is common and not a failure.
- Doc-refonte plan, section G (commits `27401c9` through `725a619`).
