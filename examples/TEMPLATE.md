# `examples/<domain>-<technique>/README.md` template

Copy this file and fill the sections. Keep the order — readers
skim from top to bottom.

```markdown
# `<domain>-<technique>` — one-line tagline

> One-paragraph summary of what the binary does and why a reader
> would care. Mention the **technique** and the **package(s)** it
> exercises.

## What it demonstrates

- Bullet list of the specific capabilities / packages chained.
- Cross-link each `maldev` package the example imports.
- E.g. "`evasion/amsi.PatchAll` — AMSI bypass at process start."

## Build

```bash
GOOS=windows GOARCH=amd64 go build -o /tmp/example.exe ./examples/<name>
```

If the example needs CGO, build deps, or testdata fixtures, list
them here. Otherwise stay terse.

## Run

```bash
# minimal invocation
./example.exe

# operator-shaped invocation with flags
./example.exe -flag value
```

Document **every flag** the binary accepts.

## Expected output

Either paste the literal stdout (in a code fence) or describe the
file / marker the binary writes. If the demo is "interactive"
(reverse shell, etc.), describe what to do on the operator side.

## What defenders see

- Brief detection surface: ETW events, syscall sequence, registry
  keys touched, file artefacts dropped.
- Cross-link the matching `docs/techniques/<domain>/<technique>.md`
  "Detection & Forensics" section.

## MITRE ATT&CK / D3FEND

| ID | Title | Why |
|---|---|---|
| `T1XXX` | Technique name | What this example exercises |

## Limitations

- Windows version coverage (Win10 22H2, Win11 23H2, …).
- Defender posture (any exclusions / AMSI bypasses required).
- Known broken cases.

## Related

- `docs/techniques/<domain>/<technique>.md` — the reference page.
- Sibling examples that chain into this one.
```

## Exemplar

[`privesc-dll-hijack/README.md`](privesc-dll-hijack/README.md) is the
exemplar — pedagogical narrative, mermaid attack-chain diagram,
operator-step walkthrough, troubleshooting table, detection
section. Aim for that quality.

## Phasing

Existing examples shipped with one-line file-header docstrings
only. Filling out per-example READMEs is incremental work — open
a PR per example, follow the template above.
