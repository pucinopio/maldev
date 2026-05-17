# Architecture Decision Records (ADRs)

> Short notes that capture **why** maldev is shaped the way it is.
> Each ADR has the same minimal structure (Context → Decision →
> Consequences) so a reader can answer "why was X chosen over Y?"
> in two minutes.
>
> Pattern from [Michael Nygard's original ADR proposal](https://cognitect.com/blog/2011/11/15/documenting-architecture-decisions)
> — adopted by Spotify, ThoughtWorks, Kubernetes, and many others.

## Why ADRs

Code shows *what* we built. godoc shows *how* to call it. ADRs
show *why* the alternatives were rejected. Without them, every
new contributor (or you in 6 months) re-litigates the same
choices.

Examples of questions ADRs answer:

- Why does every NT* call accept an optional `*wsyscall.Caller`?
- Why does the packer expose 10 modes instead of 1 with flags?
- Why mdBook and not Docusaurus?
- Why `examples/` at top-level and not `cmd/examples/`?
- Why is the API ref godoc-only?

## Template

```markdown
# ADR-NNNN — Short title

**Status:** proposed | accepted | superseded by ADR-MMMM
**Date:** YYYY-MM-DD

## Context

What forces are at play? What are the constraints? What did we
already try? Cite the issue / PR / discussion that triggered the
decision.

## Decision

The choice in one paragraph. Imperative voice: "We will use X."

## Consequences

- Positive: what we gain.
- Negative: what we lose, what we'll have to live with.
- Neutral: behavioural changes that aren't clearly pro or con.

## Alternatives considered

- **Option A** — why rejected.
- **Option B** — why rejected.

## References

Links to issues / PRs / external materials.
```

## Index

ADRs are sequentially numbered, never renumbered, never deleted
(superseded ADRs stay readable so the history is traceable).

| # | Title | Status |
|---|---|---|
| [0001](0001-wsyscall-caller-pattern.md) | The `wsyscall.Caller` pattern | accepted |
| [0002](0002-godoc-only-api-ref.md) | godoc-only API reference | accepted |
| [0003](0003-mdbook-over-docusaurus.md) | mdBook over Docusaurus (for now) | accepted |
| [0004](0004-diataxis-pragmatic.md) | Diátaxis-pragmatic, not Diátaxis-pure | accepted |

## Adding a new ADR

1. Copy the template above into `NNNN-short-title.md` (next
   sequential number).
2. Fill the sections — keep it short (≤2 pages typical).
3. Add a row to the index table above.
4. Open a PR. ADRs get reviewed like code; the discussion in the
   PR is part of the record.

## Anti-patterns

- **Don't write speculative ADRs.** Capture decisions you've
  actually made, not options you're still considering.
- **Don't rewrite old ADRs.** If a decision is reversed, write a
  new ADR that supersedes the old one. The history is the point.
- **Don't put "why we chose X" content inside technique pages.**
  Technique pages explain *what* and *how*. ADRs explain *why*.
