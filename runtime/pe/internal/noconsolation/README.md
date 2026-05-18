# No-Consolation embedded object

This directory holds the compiled
[Fortra No-Consolation](https://github.com/fortra/No-Consolation)
BOF object file (`NoConsolation.x64.o`, MIT-licensed) that
`runtime/pe` embeds via `//go:embed` when built with the
`pe_noconsolation` build tag.

The `.o` is **committed to the repo** so operators don't need
mingw at run time — same pattern as
`kernel/driver/rtcore64/RTCore64.sys`. The embed stays behind a
build tag because the binary is a public BOF and AV/EDR
signatures will likely flag the resulting implant unless
operators opt in explicitly.

## Rebuilding

Pinned commit: see `scripts/build-no-consolation.sh` (currently
`fortra/No-Consolation @ dbdb16b`). Rebuild from clean source
with:

```bash
bash scripts/build-no-consolation.sh
```

The script clones the upstream repo, compiles with
`x86_64-w64-mingw32-gcc`, and drops the artefact alongside this
README plus the upstream `LICENSE`.
