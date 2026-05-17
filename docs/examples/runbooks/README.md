# Runbooks — when things go wrong

> **Difference from the Cookbook recipes:** the Cookbook shows
> *how to build*; runbooks show *what to do when something breaks*
> in the field. They are diagnostic decision trees with concrete
> next steps, format inspired by SRE incident playbooks and
> Pagerduty runbooks.
>
> Each runbook is structured: Symptom → Most likely causes → Step-
> by-step diagnostic → Mitigation. Skim the symptom, identify the
> cause, follow the steps.

## When to read a runbook

You're mid-engagement (or mid-test) and something doesn't behave as
the Cookbook said it would. The runbook is the page you wish you
had during the incident — written by someone who already saw it.

## Index

| Symptom | Runbook |
|---|---|
| Packed binary detected by Defender at write-to-disk | [Defender catch on dropper](defender-catch.md) |
| `LoadLibrary("hijackme.dll")` succeeds but my payload didn't run | [DLL hijack succeeded but silent](dll-hijack-silent.md) |
| `amsi.PatchAll` returns nil, AMSI still scans my Assembly.Load | [AMSI re-armed mid-flight](amsi-re-armed.md) |

## Conventions

Every runbook follows the same shape:

```markdown
# <Symptom in operator's words>

## When you see this

Concrete observable signal that matches this runbook.

## Most likely causes (ranked)

1. Cause A — short explanation, % frequency from past incidents.
2. Cause B — …

## Diagnostic steps

Numbered, executable. Each step has a clear pass/fail next-pointer.

## Mitigation

The fix(es) ordered by cost (cheapest first).

## Prevention

How to avoid the same symptom in the next engagement.
```

## Adding a runbook

1. Write the symptom in operator's words first (this is the page
   title and what someone searching the docs at 2 AM will type).
2. Capture concrete observable signals — log lines, error messages,
   defender events.
3. Order causes by past-incident frequency, not theoretical likeliness.
4. Each diagnostic step should have a `pass: … / fail: …` pointer.
5. Cross-link the matching `docs/techniques/` page for the deep
   technical explanation.
